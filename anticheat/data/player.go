package data

import (
"math"
"sync"
"time"

"github.com/go-gl/mathgl/mgl32"
"github.com/google/uuid"
)

// cpsWindow is the rolling time-window used to count clicks per second.
const cpsWindow = time.Second

// Player stores per-session state used by all anti-cheat checks.
// Fields are split into logical groups mirroring Oomph-AC's component model.
type Player struct {
mu sync.RWMutex

// ── Identity ────────────────────────────────────────────────────────────
UUID     uuid.UUID
Username string

// ── Simulation tick (PlayerAuthInput.Tick) ──────────────────────────────
// SimulationFrame matches Oomph's p.SimulationFrame. It is set directly
// from the Tick field of every PlayerAuthInput packet.
SimulationFrame     uint64
LastSimulationFrame uint64

// ── Position / velocity ─────────────────────────────────────────────────
Position     mgl32.Vec3
LastPosition mgl32.Vec3
OnGround     bool
LastOnGround bool

// Velocity is derived each tick from the position delta.
Velocity     mgl32.Vec3
LastVelocity mgl32.Vec3

LastMoveTime time.Time

// ── Fall tracking ───────────────────────────────────────────────────────
FallDistance float32
FallStartY   float32

// ── Rotation (Oomph: Movement().RotationDelta()) ────────────────────────
// Rotation holds [yaw, pitch] from the most recent PlayerAuthInput.
Rotation     mgl32.Vec2
LastRotation mgl32.Vec2
// RotationDelta is the absolute per-tick change in [yaw, pitch].
RotationDelta mgl32.Vec2

// ── Combat ─────────────────────────────────────────────────────────────
// LastSwingTick is the SimulationFrame of the most recent arm-swing event
// (packet.Animate ActionType==AnimateActionSwingArm or InputFlagMissedSwing).
LastSwingTick uint64

// ClickTimestamps is a rolling list of recent attack timestamps used to
// compute clicks-per-second (AutoClicker check).
ClickTimestamps []time.Time

// LastAttackTime and LastAttackTarget are kept for legacy KillAura checks.
LastAttackTime   time.Time
LastAttackTarget uuid.UUID

// ── Violation counters (legacy, kept for compatibility) ─────────────────
Violations map[string]int
}

// NewPlayer creates a fresh Player for the given UUID and username.
func NewPlayer(id uuid.UUID, username string) *Player {
return &Player{
UUID:         id,
Username:     username,
Violations:   make(map[string]int),
LastMoveTime: time.Now(),
}
}

// ── Tick ────────────────────────────────────────────────────────────────────

// UpdateTick records the latest simulation frame from PlayerAuthInput.Tick.
func (p *Player) UpdateTick(tick uint64) {
p.mu.Lock()
defer p.mu.Unlock()
p.LastSimulationFrame = p.SimulationFrame
p.SimulationFrame = tick
}

// SimFrame returns the current simulation frame (thread-safe).
func (p *Player) SimFrame() uint64 {
p.mu.RLock()
defer p.mu.RUnlock()
return p.SimulationFrame
}

// ── Rotation ────────────────────────────────────────────────────────────────

// UpdateRotation records the latest [yaw, pitch] from PlayerAuthInput and
// computes RotationDelta as the absolute per-tick change.
func (p *Player) UpdateRotation(yaw, pitch float32) {
p.mu.Lock()
defer p.mu.Unlock()
p.LastRotation = p.Rotation
p.Rotation = mgl32.Vec2{yaw, pitch}

// Wrap yaw delta into (-180, 180] to handle 360° boundary crossing.
yawDelta := yaw - p.LastRotation[0]
if yawDelta > 180 {
yawDelta -= 360
} else if yawDelta < -180 {
yawDelta += 360
}
pitchDelta := pitch - p.LastRotation[1]
p.RotationDelta = mgl32.Vec2{
float32(math.Abs(float64(yawDelta))),
float32(math.Abs(float64(pitchDelta))),
}
}

// RotationSnapshot returns the current rotation delta (yawDelta, pitchDelta)
// in absolute values, safe for use outside the lock.
func (p *Player) RotationSnapshot() (yawDelta, pitchDelta float32) {
p.mu.RLock()
defer p.mu.RUnlock()
return p.RotationDelta[0], p.RotationDelta[1]
}

// ── Position / velocity ──────────────────────────────────────────────────────

// UpdatePosition records a new position and derives velocity in blocks/second.
func (p *Player) UpdatePosition(pos mgl32.Vec3, onGround bool) {
p.mu.Lock()
defer p.mu.Unlock()

now := time.Now()
dt := now.Sub(p.LastMoveTime).Seconds()
if dt <= 0 {
dt = 0.05 // guard against zero division
}

p.LastPosition = p.Position
p.LastOnGround = p.OnGround
p.LastVelocity = p.Velocity

delta := pos.Sub(p.Position)
p.Velocity = mgl32.Vec3{
delta[0] / float32(dt),
delta[1] / float32(dt),
delta[2] / float32(dt),
}

p.Position = pos
p.OnGround = onGround
p.LastMoveTime = now

// Fall distance tracking.
if !onGround && pos[1] < p.LastPosition[1] {
if p.FallStartY == 0 {
p.FallStartY = p.LastPosition[1]
}
p.FallDistance = p.FallStartY - pos[1]
} else if onGround {
p.FallDistance = 0
p.FallStartY = 0
}
}

// HorizontalSpeed returns the horizontal speed (XZ plane) in blocks/second.
func (p *Player) HorizontalSpeed() float32 {
p.mu.RLock()
defer p.mu.RUnlock()
return mgl32.Vec2{p.Velocity[0], p.Velocity[2]}.Len()
}

// NoFallSnapshot returns whether the player just landed and the fall distance.
func (p *Player) NoFallSnapshot() (justLanded bool, fallDistance float32) {
p.mu.RLock()
defer p.mu.RUnlock()
return p.OnGround && !p.LastOnGround, p.FallDistance
}

// FlySnapshot returns whether the player is airborne and their Y velocity.
func (p *Player) FlySnapshot() (airborne bool, yVelPerSec float32) {
p.mu.RLock()
defer p.mu.RUnlock()
return !p.OnGround, p.Velocity[1]
}

// CurrentPosition returns the player's current position (thread-safe).
func (p *Player) CurrentPosition() mgl32.Vec3 {
p.mu.RLock()
defer p.mu.RUnlock()
return p.Position
}

// ── Swing ────────────────────────────────────────────────────────────────────

// RecordSwing updates LastSwingTick to the current SimulationFrame.
// Called on packet.Animate (SwingArm) and InputFlagMissedSwing.
func (p *Player) RecordSwing() {
p.mu.Lock()
defer p.mu.Unlock()
p.LastSwingTick = p.SimulationFrame
}

// SwingTick returns the simulation frame of the last recorded arm swing.
func (p *Player) SwingTick() uint64 {
p.mu.RLock()
defer p.mu.RUnlock()
return p.LastSwingTick
}

// ── Clicks / CPS ─────────────────────────────────────────────────────────────

// RecordClick appends the current time to the rolling click-timestamp list and
// prunes entries older than cpsWindow.
func (p *Player) RecordClick() {
p.mu.Lock()
defer p.mu.Unlock()
now := time.Now()
cutoff := now.Add(-cpsWindow)
start := 0
for start < len(p.ClickTimestamps) && p.ClickTimestamps[start].Before(cutoff) {
start++
}
p.ClickTimestamps = append(p.ClickTimestamps[start:], now)
}

// CPS returns the number of clicks recorded in the last second.
func (p *Player) CPS() int {
p.mu.RLock()
defer p.mu.RUnlock()
cutoff := time.Now().Add(-cpsWindow)
count := 0
for _, t := range p.ClickTimestamps {
if !t.Before(cutoff) {
count++
}
}
return count
}

// ── Legacy combat helpers ────────────────────────────────────────────────────

// LastAttackInfo returns the time and target UUID of the most recent attack.
func (p *Player) LastAttackInfo() (time.Time, uuid.UUID) {
p.mu.RLock()
defer p.mu.RUnlock()
return p.LastAttackTime, p.LastAttackTarget
}

// RecordAttack records the time and target of the most recent attack.
func (p *Player) RecordAttack(target uuid.UUID) {
p.mu.Lock()
defer p.mu.Unlock()
p.LastAttackTime = time.Now()
p.LastAttackTarget = target
}

// ── Legacy violation counters ────────────────────────────────────────────────

// AddViolation increments the legacy violation counter and returns the new total.
func (p *Player) AddViolation(checkName string) int {
p.mu.Lock()
defer p.mu.Unlock()
p.Violations[checkName]++
return p.Violations[checkName]
}

// ResetViolations resets the legacy counter for a specific check.
func (p *Player) ResetViolations(checkName string) {
p.mu.Lock()
defer p.mu.Unlock()
p.Violations[checkName] = 0
}

// ViolationCount returns the current legacy violation count for a check.
func (p *Player) ViolationCount(checkName string) int {
p.mu.RLock()
defer p.mu.RUnlock()
return p.Violations[checkName]
}
