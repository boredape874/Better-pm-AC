package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// speedBAirMultiplier is the factor applied to the ground MaxSpeed limit when
// checking aerial movement. In vanilla Bedrock Edition a player cannot accelerate
// horizontally while in the air — they can only carry momentum from the ground.
// The peak horizontal speed during a sprint-jump is roughly the same as the
// ground sprint speed; we allow 1.1× tolerance for the brief acceleration frame
// at jump initiation and to absorb minor floating-point / lag differences.
const speedBAirMultiplier = float32(1.10)

// speedBGraceTicks is the number of airborne ticks to skip before applying the
// aerial speed check. 3 ticks is sufficient to absorb the initial jump frame
// without giving fly-speed hacks a free window. Oomph uses a similarly short
// grace before its aerial speed validation engages.
const speedBGraceTicks = 3

// SpeedBCheck detects players that maintain or increase horizontal velocity
// while airborne beyond what vanilla physics permit.
//
// Speed/A is restricted to ground movement; Speed/B covers the air gap.
// A common speed hack variant keeps the player airborne (AirTicks > 0) while
// still moving at ground sprint speed or higher — Speed/A never fires because
// the player is never detected as on-ground.
//
// Algorithm (mirrors Oomph's aerial speed validation):
//  1. Skip if AirTicks < speedBGraceTicks (jump initiation / knockback).
//  2. Compute effective limit = MaxSpeed × sprint multiplier × speedBAirMultiplier.
//  3. Adjust for Speed potion effect (same formula as Speed/A).
//  4. Flag when horizontal speed > effective limit.
//
// Implements anticheat.Detection.
type SpeedBCheck struct {
	cfg       config.SpeedBConfig
	authority *config.AuthorityConfig
}

func NewSpeedBCheck(cfg config.SpeedBConfig) *SpeedBCheck { return &SpeedBCheck{cfg: cfg} }

// SetAuthority wires the shared AuthorityConfig.
func (c *SpeedBCheck) SetAuthority(a *config.AuthorityConfig) { c.authority = a }

func (*SpeedBCheck) Type() string    { return "Speed" }
func (*SpeedBCheck) SubType() string { return "B" }
func (*SpeedBCheck) Description() string {
	return "Detects excessive horizontal speed while airborne (blocks/tick)."
}
func (*SpeedBCheck) Punishable() bool { return true }
func (c *SpeedBCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *SpeedBCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    3,
		MaxBuffer:     5,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// CheckLegacy is the original aerial-speed implementation
// (γ.3.7 migration: retained as fallback when MovementAuth is disabled).
func (c *SpeedBCheck) CheckLegacy(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if p.IsCreative() || p.IsGliding() {
		return false, ""
	}
	if p.HasKnockbackGrace() {
		return false, ""
	}
	_, _, inWater, crawling, _ := p.InputSnapshotFull()
	if inWater || crawling {
		return false, ""
	}
	_, _, airTicks, _, _ := p.FlySnapshot()
	if airTicks < speedBGraceTicks {
		return false, ""
	}
	speed := p.HorizontalSpeed()
	maxSpeed := float32(c.cfg.MaxSpeed) * speedBAirMultiplier
	sprinting, _, _, _, _ := p.InputSnapshotFull()
	if sprinting {
		maxSpeed *= sprintSpeedMultiplier
	}
	if amp, active := p.EffectAmplifier(packet.EffectSpeed); active {
		maxSpeed *= 1.0 + speedEffectBonus*float32(amp+1)
	}
	if amp, active := p.EffectAmplifier(packet.EffectSlowness); active {
		maxSpeed *= 1.0 - slownessSpeedPenalty*float32(amp+1)
		if maxSpeed < 0 {
			maxSpeed = 0
		}
	}
	if speed > maxSpeed {
		return true, fmt.Sprintf("air_speed=%.4f max=%.4f air_ticks=%d", speed, maxSpeed, airTicks)
	}
	return false, ""
}

// Check evaluates horizontal speed while the player is airborne.
// When MovementAuth is enabled, committed-position XZ delta is used.
func (c *SpeedBCheck) Check(p *data.Player) (bool, string) {
	if c.authority != nil && c.authority.MovementAuth {
		return c.checkCommitted(p)
	}
	return c.CheckLegacy(p)
}

// checkCommitted uses CommittedPos XZ delta for aerial speed detection.
func (c *SpeedBCheck) checkCommitted(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if p.IsCreative() || p.IsGliding() {
		return false, ""
	}
	if p.HasKnockbackGrace() {
		return false, ""
	}
	_, _, inWater, crawling, _ := p.InputSnapshotFull()
	if inWater || crawling {
		return false, ""
	}
	_, _, airTicks, _, _ := p.FlySnapshot()
	if airTicks < speedBGraceTicks {
		return false, ""
	}

	// Horizontal delta from committed positions (server-authoritative).
	delta := p.CommittedPos().Sub(p.PrevCommittedPos())
	speed := mgl32.Vec2{delta[0], delta[2]}.Len()
	maxSpeed := float32(c.cfg.MaxSpeed) * speedBAirMultiplier

	sprinting, _, _, _, _ := p.InputSnapshotFull()
	if sprinting {
		maxSpeed *= sprintSpeedMultiplier
	}
	if amp, active := p.EffectAmplifier(packet.EffectSpeed); active {
		maxSpeed *= 1.0 + speedEffectBonus*float32(amp+1)
	}
	if amp, active := p.EffectAmplifier(packet.EffectSlowness); active {
		maxSpeed *= 1.0 - slownessSpeedPenalty*float32(amp+1)
		if maxSpeed < 0 {
			maxSpeed = 0
		}
	}

	if speed > maxSpeed {
		return true, fmt.Sprintf("committed_air_speed=%.4f max=%.4f air_ticks=%d", speed, maxSpeed, airTicks)
	}
	return false, ""
}
