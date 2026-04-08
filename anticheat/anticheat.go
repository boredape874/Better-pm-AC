// Package anticheat is the core of Better-pm-AC. It coordinates all detection
// checks and maintains per-player violation state using Oomph-AC's buffer-based
// system (DetectionMetadata with Buffer/FailBuffer/MaxBuffer/Violations).
package anticheat

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/boredape874/Better-pm-AC/anticheat/checks/combat"
	"github.com/boredape874/Better-pm-AC/anticheat/checks/movement"
	pkt "github.com/boredape874/Better-pm-AC/anticheat/checks/packet"
	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// Re-export so callers only need to import anticheat, not anticheat/meta.
type Detection = meta.Detection
type DetectionMetadata = meta.DetectionMetadata

// playerDetections holds one *DetectionMetadata copy per check, per player.
type playerDetections struct {
	speed        *DetectionMetadata
	speedB       *DetectionMetadata
	fly          *DetectionMetadata
	noFall       *DetectionMetadata
	noFallB      *DetectionMetadata
	noSlow       *DetectionMetadata
	phase        *DetectionMetadata
	reach        *DetectionMetadata
	killAura     *DetectionMetadata
	killAuraB    *DetectionMetadata
	killAuraC    *DetectionMetadata
	autoClicker  *DetectionMetadata
	autoClickerB *DetectionMetadata
	aim          *DetectionMetadata
	aimB         *DetectionMetadata
	badPacket    *DetectionMetadata
	badPacketB   *DetectionMetadata
	badPacketC   *DetectionMetadata
	badPacketD   *DetectionMetadata
	timer        *DetectionMetadata
	velocity     *DetectionMetadata
	scaffold     *DetectionMetadata
}

// Manager coordinates all anti-cheat checks and the player registry.
type Manager struct {
	cfg config.AnticheatConfig
	log *slog.Logger

	mu         sync.RWMutex
	players    map[uuid.UUID]*data.Player
	detections map[uuid.UUID]*playerDetections

	// Stateless check instances
	speed        *movement.SpeedCheck
	speedB       *movement.SpeedBCheck
	fly          *movement.FlyCheck
	noFall       *movement.NoFallCheck
	noFallB      *movement.NoFallBCheck
	noSlow       *movement.NoSlowCheck
	phase        *movement.PhaseACheck
	reach        *combat.ReachCheck
	killAura     *combat.KillAuraCheck
	killAuraB    *combat.KillAuraBCheck
	killAuraC    *combat.KillAuraCCheck
	autoClicker  *combat.AutoClickerCheck
	autoClickerB *combat.AutoClickerBCheck
	aim          *combat.AimCheck
	aimB         *combat.AimBCheck
	badPacket    *pkt.BadPacketCheck
	badPacketB   *pkt.BadPacketBCheck
	badPacketC   *pkt.BadPacketCCheck
	badPacketD   *pkt.BadPacketDCheck
	timer        *movement.TimerCheck
	velocity     *movement.VelocityCheck
	scaffold     *movement.ScaffoldCheck

	// KickFunc is called when a player should be disconnected.
	KickFunc func(id uuid.UUID, reason string)
}

// NewManager creates a Manager ready to process packets.
func NewManager(cfg config.AnticheatConfig, log *slog.Logger) *Manager {
	return &Manager{
		cfg:          cfg,
		log:          log,
		players:      make(map[uuid.UUID]*data.Player),
		detections:   make(map[uuid.UUID]*playerDetections),
		speed:        movement.NewSpeedCheck(cfg.Speed),
		speedB:       movement.NewSpeedBCheck(cfg.SpeedB),
		fly:          movement.NewFlyCheck(cfg.Fly),
		noFall:       movement.NewNoFallCheck(cfg.NoFall),
		noFallB:      movement.NewNoFallBCheck(cfg.NoFallB),
		noSlow:       movement.NewNoSlowCheck(cfg.NoSlow),
		phase:        movement.NewPhaseACheck(cfg.Phase),
		reach:        combat.NewReachCheck(cfg.Reach),
		killAura:     combat.NewKillAuraCheck(cfg.KillAura),
		killAuraB:    combat.NewKillAuraBCheck(cfg.KillAuraB),
		killAuraC:    combat.NewKillAuraCCheck(cfg.KillAuraC),
		autoClicker:  combat.NewAutoClickerCheck(cfg.AutoClicker),
		autoClickerB: combat.NewAutoClickerBCheck(cfg.AutoClickerB),
		aim:          combat.NewAimCheck(cfg.Aim),
		aimB:         combat.NewAimBCheck(cfg.AimB),
		badPacket:    pkt.NewBadPacketCheck(cfg.BadPacket),
		badPacketB:   pkt.NewBadPacketBCheck(cfg.BadPacketB),
		badPacketC:   pkt.NewBadPacketCCheck(cfg.BadPacketC),
		badPacketD:   pkt.NewBadPacketDCheck(cfg.BadPacketD),
		timer:        movement.NewTimerCheck(cfg.Timer),
		velocity:     movement.NewVelocityCheck(cfg.Velocity),
		scaffold:     movement.NewScaffoldCheck(cfg.Scaffold),
	}
}

func (m *Manager) newPlayerDetections() *playerDetections {
	return &playerDetections{
		speed:        m.speed.DefaultMetadata(),
		speedB:       m.speedB.DefaultMetadata(),
		fly:          m.fly.DefaultMetadata(),
		noFall:       m.noFall.DefaultMetadata(),
		noFallB:      m.noFallB.DefaultMetadata(),
		noSlow:       m.noSlow.DefaultMetadata(),
		phase:        m.phase.DefaultMetadata(),
		reach:        m.reach.DefaultMetadata(),
		killAura:     m.killAura.DefaultMetadata(),
		killAuraB:    m.killAuraB.DefaultMetadata(),
		killAuraC:    m.killAuraC.DefaultMetadata(),
		autoClicker:  m.autoClicker.DefaultMetadata(),
		autoClickerB: m.autoClickerB.DefaultMetadata(),
		aim:          m.aim.DefaultMetadata(),
		aimB:         m.aimB.DefaultMetadata(),
		badPacket:    m.badPacket.DefaultMetadata(),
		badPacketB:   m.badPacketB.DefaultMetadata(),
		badPacketC:   m.badPacketC.DefaultMetadata(),
		badPacketD:   m.badPacketD.DefaultMetadata(),
		timer:        m.timer.DefaultMetadata(),
		velocity:     m.velocity.DefaultMetadata(),
		scaffold:     m.scaffold.DefaultMetadata(),
	}
}

// AddPlayer registers a new player session.
func (m *Manager) AddPlayer(id uuid.UUID, username string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.players[id] = data.NewPlayer(id, username)
	m.detections[id] = m.newPlayerDetections()
	m.log.Info("player joined", "uuid", id, "username", username)
}

// RemovePlayer removes a player session and frees its detection state.
func (m *Manager) RemovePlayer(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.players, id)
	delete(m.detections, id)
}

func (m *Manager) getPlayer(id uuid.UUID) *data.Player {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.players[id]
}

// GetPlayer returns the Player data for the given UUID (thread-safe).
// Used by the proxy layer to update per-tick input state before running checks.
func (m *Manager) GetPlayer(id uuid.UUID) *data.Player {
	return m.getPlayer(id)
}

func (m *Manager) getDet(id uuid.UUID) *playerDetections {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.detections[id]
}

// UpdateEntityPos records the server-authoritative position of an entity.
// Called from the serverToClient goroutine for AddPlayer, AddActor,
// MovePlayer, and MoveActorAbsolute packets.
func (m *Manager) UpdateEntityPos(playerID uuid.UUID, entityRID uint64, pos mgl32.Vec3) {
	if p := m.getPlayer(playerID); p != nil {
		p.UpdateEntityPos(entityRID, pos)
	}
}

// MapEntityUID stores a uniqueID→runtimeID mapping for a non-player actor
// so it can be cleaned up when RemoveActor is received.
func (m *Manager) MapEntityUID(playerID uuid.UUID, uid int64, rid uint64) {
	if p := m.getPlayer(playerID); p != nil {
		p.MapEntityUID(uid, rid)
	}
}

// RemoveEntity removes an entity from a player's position table.
// Called when the server sends a RemoveActor packet.
func (m *Manager) RemoveEntity(playerID uuid.UUID, uid int64) {
	if p := m.getPlayer(playerID); p != nil {
		p.RemoveEntity(uid)
	}
}

// OnInput is called for every PlayerAuthInput packet.
// inputMode is PlayerAuthInput.InputMode (1=Mouse, 2=Touch, 3=GamePad).
func (m *Manager) OnInput(id uuid.UUID, tick uint64, pos mgl32.Vec3, onGround bool, yaw, pitch float32, inputMode uint32) {
	p := m.getPlayer(id)
	det := m.getDet(id)
	if p == nil || det == nil {
		return
	}

	// Record arrival time for Timer/A before any state updates.
	p.RecordInputTime()

	// BadPacket/D: NaN/Infinity position — run before UpdatePosition so a
	// poison packet cannot corrupt the player's position state.
	if flagged, info := m.badPacketD.Check(pos); flagged {
		if det.badPacketD.Fail(int64(tick)) {
			m.handleViolation(p, m.badPacketD, det.badPacketD, info)
		}
		// Do not process further: updating state with NaN/Inf would corrupt checks.
		return
	}

	// BadPacket/A (tick validation) runs before UpdateTick so it can compare
	// the incoming tick against the previously stored simulation frame.
	if flagged, info := m.badPacket.Check(p, tick); flagged {
		if det.badPacket.Fail(int64(tick)) {
			m.handleViolation(p, m.badPacket, det.badPacket, info)
		}
	}

	p.UpdateTick(tick)
	p.UpdateRotation(yaw, pitch)
	p.UpdatePosition(pos, onGround)
	p.SetInputMode(inputMode)

	// BadPacket/B: pitch range validation.
	if flagged, info := m.badPacketB.Check(p, pitch); flagged {
		if det.badPacketB.Fail(int64(tick)) {
			m.handleViolation(p, m.badPacketB, det.badPacketB, info)
		}
	}

	// BadPacket/C: simultaneous Sprint+Sneak — impossible in vanilla.
	// Input flags are set by SetInputFlags() (called before OnInput from proxy)
	// so InputSnapshot() already reflects the current tick's flags.
	if flagged, info := m.badPacketC.Check(p); flagged {
		if det.badPacketC.Fail(int64(tick)) {
			m.handleViolation(p, m.badPacketC, det.badPacketC, info)
		}
	}

	// Timer/A: high-rate packet detection.
	if flagged, info := m.timer.Check(p); flagged {
		if det.timer.Fail(int64(tick)) {
			m.handleViolation(p, m.timer, det.timer, info)
		}
	} else {
		det.timer.Pass(0.05)
	}

	// Consume teleport grace: if the client just handled a server teleport,
	// skip position-based movement checks for this tick to avoid a false-positive
	// velocity spike from the position discontinuity.
	teleportGrace := p.ConsumeTeleportGrace()

	// Velocity/A (Anti-KB): consume any pending knockback snapshot recorded when
	// the server sent SetActorMotion / MotionPredictionHints and check whether the
	// player's current positional delta reflects the applied impulse. This runs
	// after UpdatePosition so that p.PositionDelta() reflects the current tick.
	// We only run the check when there is no active knockback grace window; the
	// first tick after the grace expires is when Anti-KB cheats become visible
	// because the player should still be carrying residual horizontal velocity.
	if !p.HasKnockbackGrace() {
		if kb := p.KnockbackSnapshot(); kb.Len() >= movement.VelocityAMinKB {
			if flagged, info := m.velocity.Check(p, kb); flagged {
				if det.velocity.Fail(int64(tick)) {
					m.handleViolation(p, m.velocity, det.velocity, info)
				}
			} else {
				det.velocity.Pass(1.0)
			}
		}
	}

	// Speed/A — skip if teleporting or in creative mode.
	if !teleportGrace {
		if flagged, info := m.speed.Check(p); flagged {
			if det.speed.Fail(int64(tick)) {
				m.handleViolation(p, m.speed, det.speed, info)
			}
		} else {
			det.speed.Pass(0.05)
		}

		// Speed/B — aerial horizontal speed.
		if flagged, info := m.speedB.Check(p); flagged {
			if det.speedB.Fail(int64(tick)) {
				m.handleViolation(p, m.speedB, det.speedB, info)
			}
		} else {
			det.speedB.Pass(0.05)
		}

		// Phase/A — impossible position delta without teleport.
		if flagged, info := m.phase.Check(p, false); flagged {
			if det.phase.Fail(int64(tick)) {
				m.handleViolation(p, m.phase, det.phase, info)
			}
		}
	}

	// Fly/A — creative and water exemptions are handled inside Check.
	if flagged, info := m.fly.Check(p); flagged {
		if det.fly.Fail(int64(tick)) {
			m.handleViolation(p, m.fly, det.fly, info)
		}
	} else {
		det.fly.Pass(0.5)
	}

	// NoFall/A
	if flagged, info := m.noFall.Check(p); flagged {
		if det.noFall.Fail(int64(tick)) {
			m.handleViolation(p, m.noFall, det.noFall, info)
		}
	} else {
		det.noFall.Pass(0.5)
	}

	// NoFall/B: OnGround spoof detection.
	if !teleportGrace {
		if flagged, info := m.noFallB.Check(p); flagged {
			if det.noFallB.Fail(int64(tick)) {
				m.handleViolation(p, m.noFallB, det.noFallB, info)
			}
		} else {
			det.noFallB.Pass(0.2)
		}
	}

	// NoSlow/A — item-use speed bypass (eating, bow, shield).
	if flagged, info := m.noSlow.Check(p); flagged {
		if det.noSlow.Fail(int64(tick)) {
			m.handleViolation(p, m.noSlow, det.noSlow, info)
		}
	} else {
		det.noSlow.Pass(0.2)
	}

	// Aim/A
	if flagged, info, passAmount := m.aim.Check(p); flagged {
		if det.aim.Fail(int64(tick)) {
			m.handleViolation(p, m.aim, det.aim, info)
		}
	} else if passAmount > 0 {
		det.aim.Pass(passAmount)
	}

	// Aim/B — constant pitch during yaw rotation (mouse clients only).
	if flagged, info := m.aimB.Check(p); flagged {
		if det.aimB.Fail(int64(tick)) {
			m.handleViolation(p, m.aimB, det.aimB, info)
		}
	} else {
		det.aimB.Pass(0.2)
	}
}

// OnMove is called for legacy MovePlayer packets (non-authoritative clients).
func (m *Manager) OnMove(id uuid.UUID, pos mgl32.Vec3, onGround bool) {
	p := m.getPlayer(id)
	det := m.getDet(id)
	if p == nil || det == nil {
		return
	}

	p.UpdatePosition(pos, onGround)
	tick := int64(p.SimFrame())
	teleportGrace := p.ConsumeTeleportGrace()

	if !teleportGrace {
		if flagged, info := m.speed.Check(p); flagged {
			if det.speed.Fail(tick) {
				m.handleViolation(p, m.speed, det.speed, info)
			}
		} else {
			det.speed.Pass(0.05)
		}

		if flagged, info := m.speedB.Check(p); flagged {
			if det.speedB.Fail(tick) {
				m.handleViolation(p, m.speedB, det.speedB, info)
			}
		} else {
			det.speedB.Pass(0.05)
		}

		if flagged, info := m.phase.Check(p, false); flagged {
			if det.phase.Fail(tick) {
				m.handleViolation(p, m.phase, det.phase, info)
			}
		}
	}

	if flagged, info := m.fly.Check(p); flagged {
		if det.fly.Fail(tick) {
			m.handleViolation(p, m.fly, det.fly, info)
		}
	} else {
		det.fly.Pass(0.5)
	}

	if flagged, info := m.noFall.Check(p); flagged {
		if det.noFall.Fail(tick) {
			m.handleViolation(p, m.noFall, det.noFall, info)
		}
	} else {
		det.noFall.Pass(0.5)
	}
}

// OnAttack is called when a player sends a UseItemOnEntity attack transaction.
// targetRID is the entity runtime ID of the attacked entity, used to look up
// the server-authoritative position from the entity position table.
// This replaces the broken ClickedPosition-based approach.
func (m *Manager) OnAttack(attackerID, targetID uuid.UUID, targetRID uint64) {
	p := m.getPlayer(attackerID)
	det := m.getDet(attackerID)
	if p == nil || det == nil {
		return
	}

	tick := int64(p.SimFrame())

	p.RecordClick()
	p.RecordAttack(targetID)

	// Reach/A: only run when we have a server-authoritative entity position.
	// If the entity is not in the table (e.g. the first attack before any
	// server position has been received), skip to avoid false positives.
	if targetPos, ok := p.EntityPos(targetRID); ok {
		if flagged, info := m.reach.Check(p, targetPos); flagged {
			if det.reach.Fail(tick) {
				m.handleViolation(p, m.reach, det.reach, info)
			}
		} else {
			det.reach.Pass(0.0015)
		}

		// KillAura/B: flag if the target is outside the player's FOV.
		if flagged, info := m.killAuraB.Check(p, targetPos); flagged {
			if det.killAuraB.Fail(tick) {
				m.handleViolation(p, m.killAuraB, det.killAuraB, info)
			}
		} else {
			det.killAuraB.Pass(1.0)
		}
	}

	// KillAura/A
	if flagged, info := m.killAura.Check(p); flagged {
		if det.killAura.Fail(tick) {
			m.handleViolation(p, m.killAura, det.killAura, info)
		}
	} else {
		det.killAura.Pass(1.0)
	}

	// KillAura/C: multi-target per-tick.
	// RecordAttack (called above) already updated the per-tick attack count.
	if flagged, info := m.killAuraC.Check(p); flagged {
		if det.killAuraC.Fail(tick) {
			m.handleViolation(p, m.killAuraC, det.killAuraC, info)
		}
	}

	// AutoClicker/A: CPS limit.
	// Pass() is called when CPS is within the allowed limit so that the buffer
	// decays during legitimate play, preventing false positives from brief
	// bursts that occurred before the player settled back to a normal rate.
	if flagged, info := m.autoClicker.Check(p); flagged {
		if det.autoClicker.Fail(tick) {
			m.handleViolation(p, m.autoClicker, det.autoClicker, info)
		}
	} else {
		det.autoClicker.Pass(0.5)
	}

	// AutoClicker/B: click interval regularity.
	// Autoclickers produce suspiciously uniform inter-click intervals (std dev
	// close to 0 ms). Human clicks have naturally high variance (> ~15 ms).
	if flagged, info := m.autoClickerB.Check(p); flagged {
		if det.autoClickerB.Fail(tick) {
			m.handleViolation(p, m.autoClickerB, det.autoClickerB, info)
		}
	} else {
		det.autoClickerB.Pass(0.3)
	}
}

// OnSwing records an arm-swing event.
func (m *Manager) OnSwing(id uuid.UUID) {
	if p := m.getPlayer(id); p != nil {
		p.RecordSwing()
	}
}

// OnBlockPlace is called when a player sends a UseItem (ClickBlock) inventory
// transaction. blockPos is the position of the base block being clicked (the
// block whose face was clicked to place a new block on), and face is the face
// index (0–5, matching Bedrock's BlockFace constants). The proxy extracts both
// values from UseItemTransactionData before calling here.
func (m *Manager) OnBlockPlace(id uuid.UUID, blockPos mgl32.Vec3, face int32) {
	p := m.getPlayer(id)
	det := m.getDet(id)
	if p == nil || det == nil {
		return
	}
	tick := int64(p.SimFrame())
	if flagged, info := m.scaffold.Check(p, blockPos, face); flagged {
		if det.scaffold.Fail(tick) {
			m.handleViolation(p, m.scaffold, det.scaffold, info)
		}
	} else {
		det.scaffold.Pass(0.3)
	}
}

// handleViolation logs the violation and kicks when the threshold is reached.
func (m *Manager) handleViolation(p *data.Player, d Detection, meta *DetectionMetadata, extraInfo string) {
	m.log.Warn("violation detected",
		"player", p.Username,
		"uuid", p.UUID,
		"check", d.Type()+"/"+d.SubType(),
		"violations", fmt.Sprintf("%.2f", meta.Violations),
		"max", meta.MaxViolations,
		"info", extraInfo,
	)

	if d.Punishable() && meta.Exceeded() && m.KickFunc != nil {
		reason := fmt.Sprintf(
			"Kicked by Better-pm-AC: %s/%s (VL %.2f)",
			d.Type(), d.SubType(), meta.Violations,
		)
		m.KickFunc(p.UUID, reason)
	}
}
