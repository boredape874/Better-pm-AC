package anticheat

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/boredape874/Better-pm-AC/anticheat/checks/combat"
	"github.com/boredape874/Better-pm-AC/anticheat/checks/movement"
	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// Manager coordinates all anti-cheat checks and the player registry.
type Manager struct {
	cfg config.AnticheatConfig
	log *slog.Logger

	mu      sync.RWMutex
	players map[uuid.UUID]*data.Player

	// Movement checks
	speed   *movement.SpeedCheck
	fly     *movement.FlyCheck
	noFall  *movement.NoFallCheck

	// Combat checks
	reach    *combat.ReachCheck
	killAura *combat.KillAuraCheck

	// KickFunc is called when a player should be disconnected.
	// The proxy layer sets this during initialisation.
	KickFunc func(id uuid.UUID, reason string)
}

// NewManager creates a Manager ready to process packets.
func NewManager(cfg config.AnticheatConfig, log *slog.Logger) *Manager {
	return &Manager{
		cfg:      cfg,
		log:      log,
		players:  make(map[uuid.UUID]*data.Player),
		speed:    movement.NewSpeedCheck(cfg.Speed),
		fly:      movement.NewFlyCheck(cfg.Fly),
		noFall:   movement.NewNoFallCheck(cfg.NoFall),
		reach:    combat.NewReachCheck(cfg.Reach),
		killAura: combat.NewKillAuraCheck(cfg.KillAura),
	}
}

// AddPlayer registers a new player session.
func (m *Manager) AddPlayer(id uuid.UUID, username string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.players[id] = data.NewPlayer(id, username)
	m.log.Info("player joined", "uuid", id, "username", username)
}

// RemovePlayer removes a player session.
func (m *Manager) RemovePlayer(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.players, id)
}

// GetPlayer returns the player data for id, or nil if not found.
func (m *Manager) GetPlayer(id uuid.UUID) *data.Player {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.players[id]
}

// OnMove is called whenever a player movement packet is intercepted.
func (m *Manager) OnMove(id uuid.UUID, pos mgl32.Vec3, onGround bool) {
	p := m.GetPlayer(id)
	if p == nil {
		return
	}

	p.UpdatePosition(pos, onGround)

	if flagged, vl := m.speed.Check(p); flagged {
		m.handleViolation(p, m.speed.Name(), vl, m.cfg.Speed.Violations)
	}
	if flagged, vl := m.fly.Check(p); flagged {
		m.handleViolation(p, m.fly.Name(), vl, m.cfg.Fly.Violations)
	}
	if flagged, vl := m.noFall.Check(p); flagged {
		m.handleViolation(p, m.noFall.Name(), vl, m.cfg.NoFall.Violations)
	}
}

// OnAttack is called when a player attacks another entity.
// targetPos is the last known position of the attacked entity.
func (m *Manager) OnAttack(attackerID uuid.UUID, targetID uuid.UUID, targetPos mgl32.Vec3) {
	p := m.GetPlayer(attackerID)
	if p == nil {
		return
	}

	if flagged, vl := m.reach.Check(p, targetPos); flagged {
		m.handleViolation(p, m.reach.Name(), vl, m.cfg.Reach.Violations)
	}
	if flagged, vl := m.killAura.Check(p, targetID); flagged {
		m.handleViolation(p, m.killAura.Name(), vl, m.cfg.KillAura.Violations)
	}
}

// handleViolation logs a violation and kicks the player when the threshold is
// reached.
func (m *Manager) handleViolation(p *data.Player, checkName string, vl, threshold int) {
	m.log.Warn("violation detected",
		"player", p.Username,
		"uuid", p.UUID,
		"check", checkName,
		"violations", vl,
		"threshold", threshold,
	)

	if vl >= threshold && m.KickFunc != nil {
		reason := fmt.Sprintf("Kicked by Better-pm-AC: %s (VL %d)", checkName, vl)
		m.KickFunc(p.UUID, reason)
	}
}
