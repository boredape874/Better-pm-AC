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
// Mirrors Oomph's per-player detection-metadata model.
type playerDetections struct {
speed       *DetectionMetadata
fly         *DetectionMetadata
noFall      *DetectionMetadata
reach       *DetectionMetadata
killAura    *DetectionMetadata
autoClicker *DetectionMetadata
aim         *DetectionMetadata
badPacket   *DetectionMetadata
}

// Manager coordinates all anti-cheat checks and the player registry.
type Manager struct {
cfg config.AnticheatConfig
log *slog.Logger

mu         sync.RWMutex
players    map[uuid.UUID]*data.Player
detections map[uuid.UUID]*playerDetections

// ── Stateless check instances ──────────────────────────────────────────
speed       *movement.SpeedCheck
fly         *movement.FlyCheck
noFall      *movement.NoFallCheck
reach       *combat.ReachCheck
killAura    *combat.KillAuraCheck
autoClicker *combat.AutoClickerCheck
aim         *combat.AimCheck
badPacket   *pkt.BadPacketCheck

// KickFunc is called when a player should be disconnected.
// The proxy layer injects this during initialisation.
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
autoClicker: combat.NewAutoClickerCheck(cfg.AutoClicker),
aim:         combat.NewAimCheck(cfg.Aim),
badPacket:   pkt.NewBadPacketCheck(cfg.BadPacket),
}
}

// newPlayerDetections initialises per-player metadata from each check's
// DefaultMetadata(), mirroring Oomph's detection-registration flow.
func (m *Manager) newPlayerDetections() *playerDetections {
return &playerDetections{
speed:       m.speed.DefaultMetadata(),
fly:         m.fly.DefaultMetadata(),
noFall:      m.noFall.DefaultMetadata(),
reach:       m.reach.DefaultMetadata(),
killAura:    m.killAura.DefaultMetadata(),
autoClicker: m.autoClicker.DefaultMetadata(),
aim:         m.aim.DefaultMetadata(),
badPacket:   m.badPacket.DefaultMetadata(),
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

func (m *Manager) getDet(id uuid.UUID) *playerDetections {
m.mu.RLock()
defer m.mu.RUnlock()
return m.detections[id]
}

// ── Events ────────────────────────────────────────────────────────────────────

// OnInput is called for every PlayerAuthInput packet (server-authoritative
// movement mode). It updates simulation frame, rotation, and position, then
// runs tick-level checks: Speed, Fly, NoFall, Aim, BadPacket.
func (m *Manager) OnInput(id uuid.UUID, tick uint64, pos mgl32.Vec3, onGround bool, yaw, pitch float32) {
p := m.getPlayer(id)
det := m.getDet(id)
if p == nil || det == nil {
return
}

// BadPacket check runs before UpdateTick so it can compare old vs new tick.
if flagged, info := m.badPacket.Check(p, tick); flagged {
if det.badPacket.Fail(int64(tick)) {
m.handleViolation(p, m.badPacket, det.badPacket, info)
}
}

p.UpdateTick(tick)
p.UpdateRotation(yaw, pitch)
p.UpdatePosition(pos, onGround)

// Speed/A
if flagged, info := m.speed.Check(p); flagged {
if det.speed.Fail(int64(tick)) {
m.handleViolation(p, m.speed, det.speed, info)
}
} else {
det.speed.Pass(0.05)
}

// Fly/A
if flagged, info := m.fly.Check(p); flagged {
if det.fly.Fail(int64(tick)) {
m.handleViolation(p, m.fly, det.fly, info)
}
}

// NoFall/A
if flagged, info := m.noFall.Check(p); flagged {
if det.noFall.Fail(int64(tick)) {
m.handleViolation(p, m.noFall, det.noFall, info)
}
}

// Aim/A — three-valued return: (flagged, info, passAmount)
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

if flagged, info := m.speed.Check(p); flagged {
if det.speed.Fail(tick) {
m.handleViolation(p, m.speed, det.speed, info)
}
} else {
det.speed.Pass(0.05)
}

if flagged, info := m.fly.Check(p); flagged {
if det.fly.Fail(tick) {
m.handleViolation(p, m.fly, det.fly, info)
}
}

if flagged, info := m.noFall.Check(p); flagged {
if det.noFall.Fail(tick) {
m.handleViolation(p, m.noFall, det.noFall, info)
}
}
}

// OnAttack is called when a player sends a UseItemOnEntity attack transaction.
// targetPos is the last known position of the attacked entity.
func (m *Manager) OnAttack(attackerID, targetID uuid.UUID, targetPos mgl32.Vec3) {
p := m.getPlayer(attackerID)
det := m.getDet(attackerID)
if p == nil || det == nil {
return
}

tick := int64(p.SimFrame())

// Record click for CPS and attack tracking.
p.RecordClick()
p.RecordAttack(targetID)

// Reach/A
if flagged, info := m.reach.Check(p, targetPos); flagged {
if det.reach.Fail(tick) {
m.handleViolation(p, m.reach, det.reach, info)
}
} else {
det.reach.Pass(0.0015) // mirrors Oomph ReachA passive pass decrement
}

// KillAura/A — swing-based
if flagged, info := m.killAura.Check(p); flagged {
if det.killAura.Fail(tick) {
m.handleViolation(p, m.killAura, det.killAura, info)
}
}

// AutoClicker/A — CPS-based
if flagged, info := m.autoClicker.Check(p); flagged {
if det.autoClicker.Fail(tick) {
m.handleViolation(p, m.autoClicker, det.autoClicker, info)
}
}
}

// OnSwing is called when the client swings its arm:
//   - packet.Animate with ActionType == AnimateActionSwingArm
//   - PlayerAuthInput with InputFlagMissedSwing
//   - LevelSoundEvent with SoundType == SoundEventAttackNoDamage
//
// Matches Oomph's swing tracking across Animate, LevelSoundEvent, and MissedSwing.
func (m *Manager) OnSwing(id uuid.UUID) {
p := m.getPlayer(id)
if p == nil {
return
}
p.RecordSwing()
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// handleViolation logs the violation and kicks when the threshold is reached.
// Mirrors Oomph's FailDetection + HandlePunishment flow.
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
