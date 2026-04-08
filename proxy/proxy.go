package proxy

import (
"context"
"fmt"
"log/slog"
"sync"

"github.com/boredape874/Better-pm-AC/anticheat"
"github.com/boredape874/Better-pm-AC/config"
"github.com/go-gl/mathgl/mgl32"
"github.com/google/uuid"
"github.com/sandertv/gophertunnel/minecraft"
"github.com/sandertv/gophertunnel/minecraft/protocol"
"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Proxy is the MiTM proxy that sits between Bedrock clients and a PMMP server.
type Proxy struct {
cfg config.Config
log *slog.Logger
ac  *anticheat.Manager

mu       sync.Mutex
sessions map[uuid.UUID]*Session
}

// New creates a new Proxy with the given configuration.
func New(cfg config.Config, log *slog.Logger) *Proxy {
ac := anticheat.NewManager(cfg.Anticheat, log)
p := &Proxy{
cfg:      cfg,
log:      log,
ac:       ac,
sessions: make(map[uuid.UUID]*Session),
}
ac.KickFunc = p.kick
return p
}

// ListenAndServe starts the listener and accepts connections until ctx is
// cancelled.
func (p *Proxy) ListenAndServe(ctx context.Context) error {
listener, err := minecraft.ListenConfig{
// AuthenticationDisabled allows clients to connect without Xbox Live auth.
// Set to false in production if the PMMP server handles auth separately.
AuthenticationDisabled: true,
}.Listen("raknet", p.cfg.Proxy.ListenAddr)
if err != nil {
return fmt.Errorf("listen %s: %w", p.cfg.Proxy.ListenAddr, err)
}

p.log.Info("proxy listening", "addr", p.cfg.Proxy.ListenAddr, "remote", p.cfg.Proxy.RemoteAddr)

go func() {
<-ctx.Done()
_ = listener.Close()
}()

for {
conn, err := listener.Accept()
if err != nil {
select {
case <-ctx.Done():
return nil
default:
p.log.Error("accept error", "err", err)
continue
}
}

clientConn := conn.(*minecraft.Conn)
go p.handleClient(ctx, clientConn)
}
}

// handleClient manages a single client <-> server pair.
func (p *Proxy) handleClient(ctx context.Context, client *minecraft.Conn) {
// Dial the downstream PMMP server. TokenSource is nil => no auth (offline
// mode). PMMP handles its own player verification.
serverConn, err := minecraft.Dialer{}.DialContext(ctx, "raknet", p.cfg.Proxy.RemoteAddr)
if err != nil {
p.log.Error("dial server", "addr", p.cfg.Proxy.RemoteAddr, "err", err)
_ = client.Close()
return
}

// Start the client game using the data received from the downstream server.
if err := client.StartGameContext(ctx, serverConn.GameData()); err != nil {
p.log.Error("start game (client)", "err", err)
_ = client.Close()
_ = serverConn.Close()
return
}

id, username := identityFromConn(client)
p.ac.AddPlayer(id, username)
defer p.ac.RemovePlayer(id)

sess := newSession(id, client, serverConn)
p.addSession(sess)
defer p.removeSession(id)

p.log.Info("player connected", "username", username, "uuid", id)

errc := make(chan error, 2)

// Client -> Server (with anti-cheat inspection)
go func() {
errc <- p.clientToServer(ctx, sess)
}()

// Server -> Client (transparent forwarding)
go func() {
errc <- p.serverToClient(ctx, sess)
}()

if err := <-errc; err != nil {
p.log.Info("session ended", "username", username, "err", err)
}
}

// clientToServer reads packets from the client, runs anti-cheat checks, and
// forwards approved packets to the server.
func (p *Proxy) clientToServer(ctx context.Context, sess *Session) error {
for {
pk, err := sess.Client.ReadPacket()
if err != nil {
return err
}

switch typed := pk.(type) {
case *packet.PlayerAuthInput:
// PlayerAuthInput is the server-authoritative movement packet.
// Eye-height offset (1.62 blocks) is removed to get feet position.
pos := mgl32.Vec3{
typed.Position[0],
typed.Position[1] - 1.62,
typed.Position[2],
}
// Derive on-ground: vertical collision present and not mid-jump.
onGround := typed.InputData.Load(packet.InputFlagVerticalCollision) &&
!typed.InputData.Load(packet.InputFlagJumping)
p.ac.OnMove(sess.ID, pos, onGround)

case *packet.MovePlayer:
// MovePlayer is used in legacy (non-authoritative) movement mode.
pos := mgl32.Vec3{
typed.Position[0],
typed.Position[1] - 1.62,
typed.Position[2],
}
p.ac.OnMove(sess.ID, pos, typed.OnGround)

case *packet.InventoryTransaction:
if typed.TransactionData != nil {
if hit, ok := typed.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok {
targetID := uuidFromEntityRID(sess, hit.TargetEntityRuntimeID)
targetPos := mgl32.Vec3{
hit.ClickedPosition[0],
hit.ClickedPosition[1],
hit.ClickedPosition[2],
}
p.ac.OnAttack(sess.ID, targetID, targetPos)
}
}
}

if err := sess.Server.WritePacket(pk); err != nil {
return err
}

select {
case <-ctx.Done():
return ctx.Err()
default:
}
}
}

// serverToClient reads packets from the server and forwards them to the client
// without modification.
func (p *Proxy) serverToClient(ctx context.Context, sess *Session) error {
for {
pk, err := sess.Server.ReadPacket()
if err != nil {
return err
}

if err := sess.Client.WritePacket(pk); err != nil {
return err
}

select {
case <-ctx.Done():
return ctx.Err()
default:
}
}
}

// kick disconnects the player identified by id with the given reason.
func (p *Proxy) kick(id uuid.UUID, reason string) {
p.mu.Lock()
sess, ok := p.sessions[id]
p.mu.Unlock()

if !ok {
return
}

p.log.Info("kicking player", "uuid", id, "reason", reason)
_ = sess.Client.WritePacket(&packet.Disconnect{
HideDisconnectionScreen: false,
Message:                 reason,
})
_ = sess.Client.Close()
}

func (p *Proxy) addSession(s *Session) {
p.mu.Lock()
defer p.mu.Unlock()
p.sessions[s.ID] = s
}

func (p *Proxy) removeSession(id uuid.UUID) {
p.mu.Lock()
defer p.mu.Unlock()
delete(p.sessions, id)
}

// identityFromConn extracts the UUID and username from a connection's identity
// data. Falls back to a random UUID when parsing fails (offline-mode).
func identityFromConn(conn *minecraft.Conn) (uuid.UUID, string) {
identity := conn.IdentityData()
id, err := uuid.Parse(identity.Identity)
if err != nil {
id = uuid.New()
}
return id, identity.DisplayName
}

// uuidFromEntityRID derives a deterministic UUID from an entity runtime ID.
// In a complete implementation a map populated from AddPlayer packets is used.
func uuidFromEntityRID(_ *Session, rid uint64) uuid.UUID {
var id uuid.UUID
for i := 0; i < 8; i++ {
id[i] = byte(rid >> (i * 8))
}
return id
}
