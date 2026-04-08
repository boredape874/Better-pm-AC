package data

import (
	"sync"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// Player stores per-session state used by all anti-cheat checks.
type Player struct {
	mu sync.RWMutex

	// Identity
	UUID     uuid.UUID
	Username string

	// Position history
	Position     mgl32.Vec3
	LastPosition mgl32.Vec3
	OnGround     bool
	LastOnGround bool

	// Velocity (derived each tick from position delta)
	Velocity     mgl32.Vec3
	LastVelocity mgl32.Vec3

	// Timestamps
	LastMoveTime time.Time

	// Fall tracking
	FallDistance float32
	FallStartY   float32

	// Combat tracking
	LastAttackTime time.Time
	LastAttackTarget uuid.UUID

	// Violation counters (per-check)
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

// AddViolation atomically increments the violation counter for checkName and
// returns the new total.
func (p *Player) AddViolation(checkName string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Violations[checkName]++
	return p.Violations[checkName]
}

// ResetViolations resets the violation counter for a specific check.
func (p *Player) ResetViolations(checkName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Violations[checkName] = 0
}

// ViolationCount returns the current violation count for a check.
func (p *Player) ViolationCount(checkName string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Violations[checkName]
}

// UpdatePosition records a new position and derives velocity.
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

	// Fall distance tracking
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
	vx := p.Velocity[0]
	vz := p.Velocity[2]
	return float32(mgl32.Vec2{vx, vz}.Len())
}

// NoFallSnapshot returns whether the player just landed and how far they fell.
func (p *Player) NoFallSnapshot() (justLanded bool, fallDistance float32) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	justLanded = p.OnGround && !p.LastOnGround
	return justLanded, p.FallDistance
}

// FlySnapshot returns whether the player is airborne and their current Y
// velocity (blocks/second). Both values are read under the player lock.
func (p *Player) FlySnapshot() (airborne bool, yVelPerSec float32) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.OnGround, p.Velocity[1]
}

// CurrentPosition returns the player's current position.
func (p *Player) CurrentPosition() mgl32.Vec3 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Position
}

// LastAttackInfo returns the time and target UUID of the most recent attack.
func (p *Player) LastAttackInfo() (time.Time, uuid.UUID) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.LastAttackTime, p.LastAttackTarget
}

// RecordAttack records the time and target of the last attack.
func (p *Player) RecordAttack(target uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LastAttackTime = time.Now()
	p.LastAttackTarget = target
}
