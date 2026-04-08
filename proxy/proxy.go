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

// playerEyeHeight is the vertical offset between a player's feet position and
// eye level in Minecraft Bedrock Edition (in blocks). Matches Oomph's
// game.DefaultPlayerHeightOffset constant.
const playerEyeHeight = float32(1.62)

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
// AuthenticationDisabled allows clients to connect without Xbox Live
// auth. Set to false in production when PMMP handles auth.
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
// Dial the downstream PMMP server (offline mode — PMMP verifies players).
serverConn, err := minecraft.Dialer{}.DialContext(ctx, "raknet", p.cfg.Proxy.RemoteAddr)
if err != nil {
p.log.Error("dial server", "addr", p.cfg.Proxy.RemoteAddr, "err", err)
_ = client.Close()
return
}

// Start the client game using data received from the downstream server.
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

// Client -> Server (with anti-cheat inspection).
go func() { errc <- p.clientToServer(ctx, sess) }()
// Server -> Client (transparent forwarding).
go func() { errc <- p.serverToClient(ctx, sess) }()

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

// ── PlayerAuthInput ──────────────────────────────────────────────────
// Primary movement + rotation packet (server-authoritative mode).
// Feeds tick, position, rotation into the manager in one call (OnInput).
case *packet.PlayerAuthInput:
pos := mgl32.Vec3{
typed.Position[0],
typed.Position[1] - playerEyeHeight,
typed.Position[2],
}
// Derive on-ground from vertical-collision flag while not jumping.
onGround := typed.InputData.Load(packet.InputFlagVerticalCollision) &&
!typed.InputData.Load(packet.InputFlagJumping)

// Feed all state to the manager.
p.ac.OnInput(sess.ID, typed.Tick, pos, onGround, typed.Yaw, typed.Pitch)

// Missed-swing counts as a swing event (matches Oomph's handling).
if typed.InputData.Load(packet.InputFlagMissedSwing) {
p.ac.OnSwing(sess.ID)
}

// ── MovePlayer ──────────────────────────────────────────────────────
// Legacy non-authoritative movement packet.
case *packet.MovePlayer:
pos := mgl32.Vec3{
typed.Position[0],
typed.Position[1] - playerEyeHeight,
typed.Position[2],
}
p.ac.OnMove(sess.ID, pos, typed.OnGround)

// ── Animate ─────────────────────────────────────────────────────────
// Arm-swing animation — primary swing signal for KillauraA.
case *packet.Animate:
if typed.ActionType == packet.AnimateActionSwingArm {
p.ac.OnSwing(sess.ID)
}

// ── LevelSoundEvent ─────────────────────────────────────────────────
// SoundEventAttackNoDamage fires when the player swings and hits an
// entity that takes no damage (e.g., through a block). Oomph uses this
// as a secondary swing + click signal.
case *packet.LevelSoundEvent:
if typed.SoundType == packet.SoundEventAttackNoDamage {
p.ac.OnSwing(sess.ID)
}

// ── InventoryTransaction ─────────────────────────────────────────────
// UseItemOnEntity with ActionType == Attack is the canonical hit packet.
case *packet.InventoryTransaction:
if typed.TransactionData != nil {
if hit, ok := typed.TransactionData.(*protocol.UseItemOnEntityTransactionData); ok &&
hit.ActionType == protocol.UseItemOnEntityActionAttack {
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
// A full implementation should map runtime IDs → player UUIDs via AddPlayer
// packets from the server.
func uuidFromEntityRID(_ *Session, rid uint64) uuid.UUID {
var id uuid.UUID
for i := 0; i < 8; i++ {
id[i] = byte(rid >> (i * 8))
}
return id
}
