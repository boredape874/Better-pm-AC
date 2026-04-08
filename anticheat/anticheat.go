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
	speed       *DetectionMetadata
	fly         *DetectionMetadata
	noFall      *DetectionMetadata
	reach       *DetectionMetadata
	killAura    *DetectionMetadata
	killAuraB   *DetectionMetadata
	autoClicker *DetectionMetadata
	aim         *DetectionMetadata
	badPacket   *DetectionMetadata
	badPacketB  *DetectionMetadata
	timer       *DetectionMetadata
}

// Manager coordinates all anti-cheat checks and the player registry.
type Manager struct {
	cfg config.AnticheatConfig
	log *slog.Logger

	mu         sync.RWMutex
	players    map[uuid.UUID]*data.Player
	detections map[uuid.UUID]*playerDetections

	// Stateless check instances
	speed       *movement.SpeedCheck
	fly         *movement.FlyCheck
	noFall      *movement.NoFallCheck
	reach       *combat.ReachCheck
	killAura    *combat.KillAuraCheck
	killAuraB   *combat.KillAuraBCheck
	autoClicker *combat.AutoClickerCheck
	aim         *combat.AimCheck
	badPacket   *pkt.BadPacketCheck
	badPacketB  *pkt.BadPacketBCheck
	timer       *movement.TimerCheck

	// KickFunc is called when a player should be disconnected.
	KickFunc func(id uuid.UUID, reason string)
}

// NewManager creates a Manager ready to process packets.
func NewManager(cfg config.AnticheatConfig, log *slog.Logger) *Manager {
	return &Manager{
		cfg:         cfg,
		log:         log,
		players:     make(map[uuid.UUID]*data.Player),
		detections:  make(map[uuid.UUID]*playerDetections),
		speed:       movement.NewSpeedCheck(cfg.Speed),
		fly:         movement.NewFlyCheck(cfg.Fly),
		noFall:      movement.NewNoFallCheck(cfg.NoFall),
		reach:       combat.NewReachCheck(cfg.Reach),
		killAura:    combat.NewKillAuraCheck(cfg.KillAura),
		killAuraB:   combat.NewKillAuraBCheck(cfg.KillAuraB),
		autoClicker: combat.NewAutoClickerCheck(cfg.AutoClicker),
		aim:         combat.NewAimCheck(cfg.Aim),
		badPacket:   pkt.NewBadPacketCheck(cfg.BadPacket),
		badPacketB:  pkt.NewBadPacketBCheck(cfg.BadPacketB),
		timer:       movement.NewTimerCheck(cfg.Timer),
	}
}

func (m *Manager) newPlayerDetections() *playerDetections {
	return &playerDetections{
		speed:       m.speed.DefaultMetadata(),
		fly:         m.fly.DefaultMetadata(),
		noFall:      m.noFall.DefaultMetadata(),
		reach:       m.reach.DefaultMetadata(),
		killAura:    m.killAura.DefaultMetadata(),
		killAuraB:   m.killAuraB.DefaultMetadata(),
		autoClicker: m.autoClicker.DefaultMetadata(),
		aim:         m.aim.DefaultMetadata(),
		badPacket:   m.badPacket.DefaultMetadata(),
		badPacketB:  m.badPacketB.DefaultMetadata(),
		timer:       m.timer.DefaultMetadata(),
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

	// Speed/A — skip if teleporting or in creative mode.
	if !teleportGrace {
		if flagged, info := m.speed.Check(p); flagged {
			if det.speed.Fail(int64(tick)) {
				m.handleViolation(p, m.speed, det.speed, info)
			}
		} else {
			det.speed.Pass(0.05)
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

	// Aim/A
	if flagged, info, passAmount := m.aim.Check(p); flagged {
		if det.aim.Fail(int64(tick)) {
			m.handleViolation(p, m.aim, det.aim, info)
		}
	} else if passAmount > 0 {
		det.aim.Pass(passAmount)
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

	// AutoClicker/A
	if flagged, info := m.autoClicker.Check(p); flagged {
		if det.autoClicker.Fail(tick) {
			m.handleViolation(p, m.autoClicker, det.autoClicker, info)
		}
	}
}

// OnSwing records an arm-swing event.
func (m *Manager) OnSwing(id uuid.UUID) {
	if p := m.getPlayer(id); p != nil {
		p.RecordSwing()
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
