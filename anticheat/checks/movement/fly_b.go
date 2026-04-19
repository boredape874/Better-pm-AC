package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// flyBGraceTicks is the minimum number of consecutive airborne ticks required
// before Fly/B begins evaluating gravity. This is longer than Fly/A's grace
// (8 ticks) because:
//  1. Gravity violation counting is a per-tick accumulator; we need the
//     player to have been airborne long enough that any initial impulse
//     (jump, knockback) has had time to be absorbed.
//  2. SlimeBlock bounces, ladders, water-exit transitions, and head-bumping
//     produce short Y velocity reversals that pass within 15 ticks.
//
// 20 ticks = 1 second of airborne time mirrors Oomph's requirement that the
// simulation has been running reliably for a full second before gravity checks.
const flyBGraceTicks = 20

// flyBMinViolTicks is the number of consecutive gravity-violation ticks that
// must be observed after the grace period before Fly/B flags. Three ticks of
// anomalous Y velocity after a full second airborne is a strong signal; any
// terrain interaction (ladders, vines, slime) would have caused a ground
// detection or horizontal collision long before this threshold is reached.
const flyBMinViolTicks = 5

// FlyBCheck (Fly/B) detects players whose Y velocity does not decrease at the
// rate predicted by vanilla Bedrock gravity after the initial jump arc. This
// catches float / anti-gravity cheats that keep the player airborne without
// triggering the Fly/A hover threshold (which requires near-zero Y delta).
//
// Detection strategy (mirrors Oomph's gravity-simulation validation):
//   - On each airborne tick, data.Player.UpdatePosition computes the predicted
//     Y delta for this tick from the previous Y delta using the vanilla formula:
//       predictedY = (prevYDelta − 0.08) × 0.98
//     and increments GravViolTicks if the actual Y delta exceeds this prediction
//     by more than the tolerance (0.03 b/tick).
//   - Fly/B reads GravViolTicks from FlySnapshot and flags when it reaches
//     flyBMinViolTicks after the flyBGraceTicks grace period.
//
// Exemptions (shared with Fly/A):
//   - Creative mode
//   - Gliding (elytra)
//   - Slow Falling effect
//   - Levitation effect
//   - JumpBoost effect (extends grace period proportionally)
//   - Active knockback grace
//   - Recent water exit
//   - Actively in water or crawling
//   - HorizontalCollision/VerticalCollision flag (player is touching terrain;
//     ladders/vines/walls prevent standard gravity from applying)
//
// Implements anticheat.Detection.
type FlyBCheck struct {
	cfg config.FlyBConfig
}

func NewFlyBCheck(cfg config.FlyBConfig) *FlyBCheck { return &FlyBCheck{cfg: cfg} }

func (*FlyBCheck) Type() string    { return "Fly" }
func (*FlyBCheck) SubType() string { return "B" }
func (*FlyBCheck) Description() string {
	return "Detects gravity bypass: Y velocity not decreasing at the expected physics rate."
}
func (*FlyBCheck) Punishable() bool { return true }
func (c *FlyBCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *FlyBCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Require two consecutive check failures (each comprising flyBMinViolTicks
		// gravity violations) before recording a violation to absorb edge cases.
		FailBuffer:    2,
		MaxBuffer:     3,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates whether the player's Y velocity is following the vanilla
// gravity curve. Must be called after UpdatePosition.
func (c *FlyBCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if p.IsCreative() {
		return false, ""
	}
	if p.IsGliding() {
		return false, ""
	}
	if _, hasSlowFall := p.EffectAmplifier(packet.EffectSlowFalling); hasSlowFall {
		return false, ""
	}
	if _, hasLevitation := p.EffectAmplifier(packet.EffectLevitation); hasLevitation {
		return false, ""
	}
	if p.HasKnockbackGrace() {
		return false, ""
	}
	if p.HasRecentWaterExit() {
		return false, ""
	}

	airborne, yDelta, airTicks, _, gravViolTicks := p.FlySnapshot()
	if !airborne {
		return false, ""
	}

	_, _, inWater, crawling, _ := p.InputSnapshotFull()
	if inWater || crawling {
		return false, ""
	}

	// Exempt if the client reports horizontal or vertical terrain collision.
	// This covers ladders, vines, walls, and other climbable/collidable surfaces
	// where vanilla physics deviate significantly from free-fall gravity.
	if p.HasTerrainCollision() {
		return false, ""
	}

	// Extend the grace period proportionally for JumpBoost (same as Fly/A).
	graceTicks := flyBGraceTicks
	if amp, active := p.EffectAmplifier(packet.EffectJumpBoost); active {
		graceTicks += int(amp+1) * flyJumpBoostGracePerLevel
	}
	if airTicks <= graceTicks {
		return false, ""
	}

	if gravViolTicks >= flyBMinViolTicks {
		return true, fmt.Sprintf("grav_viol_ticks=%d air_ticks=%d y_delta=%.4f", gravViolTicks, airTicks, yDelta)
	}
	return false, ""
}
