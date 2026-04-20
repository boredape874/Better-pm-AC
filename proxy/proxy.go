package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/boredape874/Better-pm-AC/anticheat"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
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

// ListenAndServe starts the listener and accepts connections until ctx is cancelled.
func (p *Proxy) ListenAndServe(ctx context.Context) error {
	listener, err := minecraft.ListenConfig{
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
	serverConn, err := minecraft.Dialer{}.DialContext(ctx, "raknet", p.cfg.Proxy.RemoteAddr)
	if err != nil {
		p.log.Error("dial server", "addr", p.cfg.Proxy.RemoteAddr, "err", err)
		_ = client.Close()
		return
	}

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
	sess.EntityRID = serverConn.GameData().EntityRuntimeID
	p.addSession(sess)
	defer p.removeSession(id)

	p.log.Info("player connected", "username", username, "uuid", id)

	errc := make(chan error, 2)

	go func() { errc <- p.clientToServer(ctx, sess) }()
	go func() { errc <- p.serverToClient(ctx, sess) }()

	if err := <-errc; err != nil {
		p.log.Info("session ended", "username", username, "err", err)
	}

	// Release per-session state. World holds decoded chunks (potentially
	// MBs), so letting it outlive the session would leak memory across
	// reconnects. Rewind and Ack don't own significant memory but closing
	// is cheap and symmetric.
	if sess.World != nil {
		_ = sess.World.Close()
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

		// NetworkStackLatency: round-trip ack for markers dispatched via
		// sess.Ack. The client echoes the timestamp we sent in response
		// to its corresponding request. We invoke the callback with the
		// current SimFrame so checks can correlate "client confirmed it
		// processed tick N".
		case *packet.NetworkStackLatency:
			if pl := p.ac.GetPlayer(sess.ID); pl != nil && sess.Ack != nil {
				sess.Ack.OnResponse(typed.Timestamp, pl.SimFrame())
			}

		// PlayerAuthInput: primary movement + rotation packet.
		case *packet.PlayerAuthInput:
			pos := mgl32.Vec3{
				typed.Position[0],
				typed.Position[1] - playerEyeHeight,
				typed.Position[2],
			}
			onGround := typed.InputData.Load(packet.InputFlagVerticalCollision) &&
				!typed.InputData.Load(packet.InputFlagJumping)

			// Extract per-tick input state flags that affect check behaviour.
			sprinting := typed.InputData.Load(packet.InputFlagSprinting)
			sneaking := typed.InputData.Load(packet.InputFlagSneaking)

			// Maintain sticky inWater state for the session. The Bedrock protocol
			// only sends InputFlagStartSwimming once on swim entry and
			// InputFlagAutoJumpingInWater only on auto-jump; there is no continuous
			// "currently swimming" flag. We therefore keep a persistent bool in
			// sess.inWater and toggle it based on the start/stop events so that
			// SetInputFlags receives the correct value on every tick, not just the
			// entry tick. This fixes a bug where water exemptions (Fly/A, NoFall/A,
			// Speed/A/B) were only active for one tick per swim session.
			if typed.InputData.Load(packet.InputFlagStartSwimming) ||
				typed.InputData.Load(packet.InputFlagAutoJumpingInWater) {
				sess.inWater = true
			}
			if typed.InputData.Load(packet.InputFlagStopSwimming) {
				sess.inWater = false
			}

			// Maintain sticky crawling state. StartCrawling fires once on entry;
			// StopCrawling fires once on exit. Speed/A uses a dedicated crawl-speed
			// limit when this flag is active.
			if typed.InputData.Load(packet.InputFlagStartCrawling) {
				sess.isCrawling = true
			}
			if typed.InputData.Load(packet.InputFlagStopCrawling) {
				sess.isCrawling = false
			}

			// Maintain sticky item-use state. InputFlagStartUsingItem fires once
			// when the player begins using an item (eating, drawing a bow, raising a
			// shield, etc.). InputFlagPerformItemInteraction fires when the use
			// completes or is cancelled; we use it to clear the flag so that
			// NoSlow/A does not flag the player after the interaction is done.
			if typed.InputData.Load(packet.InputFlagStartUsingItem) {
				sess.isUsingItem = true
			}
			if typed.InputData.Load(packet.InputFlagPerformItemInteraction) {
				sess.isUsingItem = false
			}

			// Apply input state to player data so checks can read it.
			if pl := p.ac.GetPlayer(sess.ID); pl != nil {
				pl.SetLatency(sess.Client.Latency())
				terrainCollision := typed.InputData.Load(packet.InputFlagHorizontalCollision) ||
					typed.InputData.Load(packet.InputFlagVerticalCollision)
				pl.SetInputFlags(sprinting, sneaking, sess.inWater, sess.isCrawling, sess.isUsingItem, terrainCollision)

				// Track elytra gliding state from the start/stop events in
				// InputData so that Fly/A can exempt gliding players.
				if typed.InputData.Load(packet.InputFlagStartGliding) {
					pl.StartGliding()
				}
				if typed.InputData.Load(packet.InputFlagStopGliding) {
					pl.StopGliding()
				}

				// InputFlagHandledTeleport is set by the client after it has
				// processed a server-sent teleport packet.  The next position
				// will be at the teleport destination, producing a large
				// velocity spike.  Mark a teleport grace so Speed/A skips
				// this tick (mirrors Oomph's teleport exemption).
				if typed.InputData.Load(packet.InputFlagHandledTeleport) {
					pl.SetTeleportGrace()
				}
			}

			// Pass InputMode so Aim/A can exempt touch/gamepad clients.
			p.ac.OnInput(sess.ID, typed.Tick, pos, onGround, typed.Yaw, typed.Pitch, typed.InputMode, typed.InputData)

			if typed.InputData.Load(packet.InputFlagMissedSwing) {
				p.ac.OnSwing(sess.ID)
			}

		// MovePlayer: legacy non-authoritative movement packet.
		case *packet.MovePlayer:
			pos := mgl32.Vec3{
				typed.Position[0],
				typed.Position[1] - playerEyeHeight,
				typed.Position[2],
			}
			p.ac.OnMove(sess.ID, pos, typed.OnGround)

		// Animate: arm-swing animation for KillauraA.
		case *packet.Animate:
			if typed.ActionType == packet.AnimateActionSwingArm {
				p.ac.OnSwing(sess.ID)
			}

		// LevelSoundEvent: secondary swing signal for KillAura/A.
		// Both sound types indicate the player performed a swing animation and
		// are used as swing signals so that bots suppressing packet.Animate
		// cannot evade KillAura/A detection:
		//   - SoundEventAttackNoDamage: swing missed (no entity hit)
		//   - SoundEventAttack:         swing connected (entity hit successfully)
		// Both are intentionally treated as swing events rather than hit events;
		// the swing registration is what KillAura/A needs to correlate attacks.
		case *packet.LevelSoundEvent:
			if typed.SoundType == packet.SoundEventAttackNoDamage ||
				typed.SoundType == packet.SoundEventAttack {
				p.ac.OnSwing(sess.ID)
			}

		// InventoryTransaction: UseItemOnEntity with Attack action is the hit packet.
		// We now pass the entity runtime ID so the manager can look up the
		// server-authoritative position from the entity table instead of using
		// the client-supplied ClickedPosition (which is a hitbox-relative offset
		// and can be spoofed).
		// UseItem with ClickBlock action is the block-placement packet; pass the
		// block position and face to Scaffold/A for angle-based validation.
		case *packet.InventoryTransaction:
			if typed.TransactionData != nil {
				switch td := typed.TransactionData.(type) {
				case *protocol.UseItemOnEntityTransactionData:
					if td.ActionType == protocol.UseItemOnEntityActionAttack {
						targetID := uuidFromEntityRID(sess, td.TargetEntityRuntimeID)
						p.ac.OnAttack(sess.ID, targetID, td.TargetEntityRuntimeID)
					}
				case *protocol.UseItemTransactionData:
					if td.ActionType == protocol.UseItemActionClickBlock {
						// BlockPosition is a BlockPos (integer coordinates).
						// Convert to float32 for Scaffold/A's geometry math.
						bpf := mgl32.Vec3{
							float32(td.BlockPosition[0]),
							float32(td.BlockPosition[1]),
							float32(td.BlockPosition[2]),
						}
						p.ac.OnBlockPlace(sess.ID, bpf, td.BlockFace)
					}
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

// serverToClient reads packets from the server and forwards them to the client.
// It also inspects entity-related packets to maintain the per-player entity
// position table used by Reach/A, mirroring Oomph's entity-tracker approach.
func (p *Proxy) serverToClient(ctx context.Context, sess *Session) error {
	for {
		pk, err := sess.Server.ReadPacket()
		if err != nil {
			return err
		}

		// Maintain the entity position table for Reach/A.
		// Oomph populates its EntityTrackerComponent from MovePlayer,
		// MoveActorAbsolute, AddPlayer, and AddActor packets so that reach
		// validation uses real server-side positions rather than any value
		// supplied by the attacking client.
		switch typed := pk.(type) {

		// World state: LevelChunk / SubChunk / UpdateBlock feed the
		// per-session anticheat/world.Tracker. Errors are logged but
		// don't break forwarding — a decode failure on one chunk should
		// not disconnect the player.
		case *packet.LevelChunk:
			if err := sess.World.HandleLevelChunk(typed); err != nil {
				p.log.Debug("world: level chunk", "err", err, "pos", typed.Position)
			}

		case *packet.SubChunk:
			if err := sess.World.HandleSubChunk(typed); err != nil {
				p.log.Debug("world: sub chunk", "err", err)
			}

		case *packet.UpdateBlock:
			pos := cubePosFromBlockPos(typed.Position)
			if err := sess.World.HandleBlockUpdate(pos, typed.NewBlockRuntimeID); err != nil {
				p.log.Debug("world: update block", "err", err, "pos", pos)
			}

		case *packet.NetworkChunkPublisherUpdate:
			sess.World.HandleChunkPublisher(typed)

		case *packet.AddPlayer:
			// A new player entity has spawned. Store its initial position.
			// AddPlayer carries feet-level coordinates — no eye-height
			// adjustment is needed (the local player's position from
			// PlayerAuthInput IS eye-level, hence the subtraction there, but
			// server-sent entity positions are already feet-level).
			p.ac.UpdateEntityPos(sess.ID, typed.EntityRuntimeID, typed.Position)
			p.recordRewind(sess, typed.EntityRuntimeID, typed.Position, typed.Pitch, typed.HeadYaw)

		case *packet.AddActor:
			// A new non-player actor has spawned.
			p.ac.UpdateEntityPos(sess.ID, typed.EntityRuntimeID, typed.Position)
			// Also store the uniqueID→runtimeID mapping so RemoveActor can
			// clean up this entity from the table.
			p.ac.MapEntityUID(sess.ID, typed.EntityUniqueID, typed.EntityRuntimeID)
			p.recordRewind(sess, typed.EntityRuntimeID, typed.Position, typed.Pitch, typed.HeadYaw)

		case *packet.MovePlayer:
			// An existing player entity has moved. MovePlayer (server→client)
			// carries feet-level coordinates — no eye-height adjustment needed.
			p.ac.UpdateEntityPos(sess.ID, typed.EntityRuntimeID, typed.Position)
			p.recordRewind(sess, typed.EntityRuntimeID, typed.Position, typed.Pitch, typed.HeadYaw)

		case *packet.MoveActorAbsolute:
			// An existing non-player entity has moved.
			p.ac.UpdateEntityPos(sess.ID, typed.EntityRuntimeID, typed.Position)
			p.recordRewind(sess, typed.EntityRuntimeID, typed.Position, typed.Rotation[0], typed.Rotation[1])

		case *packet.MoveActorDelta:
			// MoveActorDelta is the bandwidth-optimised variant of MoveActorAbsolute
			// used for non-player entities in Bedrock 1.16.100+. Despite its name
			// the packet contains the new ABSOLUTE position (not a delta), so we
			// can update the entity table directly. We only update when at least one
			// positional axis is present in the packet flags; rotation-only updates
			// carry no position information and should not overwrite the stored position.
			hasPos := typed.Flags&(packet.MoveActorDeltaFlagHasX|
				packet.MoveActorDeltaFlagHasY|
				packet.MoveActorDeltaFlagHasZ) != 0
			if hasPos {
				p.ac.UpdateEntityPos(sess.ID, typed.EntityRuntimeID, typed.Position)
			}

		case *packet.RemoveActor:
			// An entity has been removed from the world; clean up the table.
			p.ac.RemoveEntity(sess.ID, typed.EntityUniqueID)
			// Rewind is keyed by runtimeID, not uniqueID — we'd need the
			// uid→rid map to purge cleanly. For β the ring buffer is fixed
			// size per rid and orphaned entries age out naturally; leaving
			// the purge unwired saves a lookup on every RemoveActor. γ
			// wires the purge once the integration bench shows memory
			// growth on long sessions.

		case *packet.SetPlayerGameType:
			// The server changed the player's game mode. Record it so checks
			// can apply creative-mode exemptions (fly, speed).
			if pl := p.ac.GetPlayer(sess.ID); pl != nil {
				pl.SetGameMode(typed.GameType)
			}

		case *packet.MobEffect:
			// A potion effect was added, modified, or removed for an entity.
			// We only care about effects on the player's own entity (sess.EntityRID)
			// so that checks can adjust their limits accordingly (e.g. Speed/A
			// increases MaxSpeed when the player has the Speed effect).
			if typed.EntityRuntimeID == sess.EntityRID {
				if pl := p.ac.GetPlayer(sess.ID); pl != nil {
					switch typed.Operation {
					case packet.MobEffectAdd, packet.MobEffectModify:
						pl.AddEffect(typed.EffectType, typed.Amplifier)
					case packet.MobEffectRemove:
						pl.RemoveEffect(typed.EffectType)
					}
				}
			}

		case *packet.SetActorMotion:
			// The server applied an external velocity to an entity (knockback from
			// damage, explosions, wind charges, launch pads, etc.).
			// If the packet targets the player's own entity, the resulting velocity
			// spike is legitimate: record a knockback grace window so that Speed/A
			// and Speed/B do not flag for the next several ticks.
			// The velocity is also stored for Velocity/A (Anti-KB) detection.
			if typed.EntityRuntimeID == sess.EntityRID {
				if pl := p.ac.GetPlayer(sess.ID); pl != nil {
					pl.RecordKnockback(typed.Velocity)
				}
			}

		case *packet.MotionPredictionHints:
			// Vanilla may use MotionPredictionHints instead of SetActorMotion when
			// spatial optimisations are enabled; treat it identically for knockback
			// grace purposes.
			if typed.EntityRuntimeID == sess.EntityRID {
				if pl := p.ac.GetPlayer(sess.ID); pl != nil {
					pl.RecordKnockback(typed.Velocity)
				}
			}
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

// identityFromConn extracts the UUID and username from a connection's identity data.
func identityFromConn(conn *minecraft.Conn) (uuid.UUID, string) {
	identity := conn.IdentityData()
	id, err := uuid.Parse(identity.Identity)
	if err != nil {
		id = uuid.New()
	}
	return id, identity.DisplayName
}

// uuidFromEntityRID derives a deterministic UUID from an entity runtime ID.
func uuidFromEntityRID(_ *Session, rid uint64) uuid.UUID {
	var id uuid.UUID
	for i := 0; i < 8; i++ {
		id[i] = byte(rid >> (i * 8))
	}
	return id
}

// cubePosFromBlockPos converts a gophertunnel protocol.BlockPos (int32 x/y/z)
// to a dragonfly cube.Pos (int x/y/z). The cast is safe on 64-bit hosts and
// necessary because anticheat/world uses cube.Pos throughout (reused from
// Dragonfly's block model APIs).
func cubePosFromBlockPos(b protocol.BlockPos) cube.Pos {
	return cube.Pos{int(b[0]), int(b[1]), int(b[2])}
}

// recordRewind pushes a single entity snapshot into the per-session rewind
// ring buffer. Called from every entity-move packet handler.
//
// The tick used is the attacker's most recent SimulationFrame (tracked on
// the data.Player). This couples rewind to the player's own input tick
// rather than wall-clock — checks later rewind to tick - (latency ticks)
// to reconstruct the pose the attacker saw at attack time.
//
// The BBox is a default 0.6×1.8 box because server-sent entity packets
// don't carry the actual collider dimensions. A γ improvement is to read
// MobEquipment / ActorFlagData to size per entity type (e.g. 0.9×0.9 for
// slimes). β uses the player-box as a reasonable default.
func (p *Proxy) recordRewind(sess *Session, rid uint64, pos mgl32.Vec3, pitch, yaw float32) {
	if sess.Rewind == nil {
		return
	}
	pl := p.ac.GetPlayer(sess.ID)
	if pl == nil {
		return
	}
	tick := pl.SimFrame()
	bbox := cube.Box(-0.3, 0, -0.3, 0.3, 1.8, 0.3).Translate(mgl32toVec64(pos))
	sess.Rewind.Record(rid, tick, pos, bbox, mgl32.Vec2{yaw, pitch})
}

// mgl32toVec64 converts an mgl32.Vec3 to a dragonfly mgl64.Vec3 — needed
// because cube.BBox.Translate takes mgl64. Duplicated from
// anticheat/sim/math.go rather than imported to avoid a proxy→sim
// dependency (the proxy shouldn't care how the sim models physics).
func mgl32toVec64(v mgl32.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(v[0]), float64(v[1]), float64(v[2])}
}
