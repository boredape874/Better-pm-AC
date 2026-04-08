package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// flyGraceTicks is the base number of consecutive airborne ticks that are
// always exempted from the fly check. This covers the natural jump arc without
// being overly permissive:
//   - Bedrock jump peak is around tick 5-6 (Y velocity ~0.02 b/tick at apex).
//   - A normal ground-level jump lasts ~12 ticks total.
//   - 8 ticks covers the ascending phase where Y velocity transitions from
//     positive to near-zero; beyond this the player should be falling, not hovering.
//
// Reduced from 20 to 8 to match Oomph's tighter simulationIsReliable() window
// and flag fly hacks that hover during the early airborne phase.
const flyGraceTicks = 8

// flyJumpBoostGracePerLevel is the additional grace ticks granted per level of
// the JumpBoost effect. JumpBoost I extends the jump arc by roughly 5 ticks;
// each subsequent level adds another 5 ticks on top.
// Without this adjustment the check produces false positives when a player
// with JumpBoost has a longer-than-normal jump arc.
const flyJumpBoostGracePerLevel = 5

// flyMinHoverTicks is the minimum number of consecutive ticks with near-zero
// Y displacement that must be observed (after the grace period) before flagging.
// Reduced from 5 to 3: once the grace period has passed the player should be
// falling at a clearly measurable rate; 3 ticks of hover is already anomalous.
const flyMinHoverTicks = 3

// flyUpwardThreshold is the minimum positive Y delta (blocks/tick) that is
// considered "rising" for the purpose of upward-fly detection. This matches
// the hoverDeltaThreshold in data/player.go: values below this are treated as
// near-zero by both hover and upward-fly checks.
const flyUpwardThreshold = float32(0.005)
// It tracks two counters updated by data.Player.UpdatePosition:
//   - AirTicks:   consecutive ticks airborne since last grounding.
//   - HoverTicks: consecutive ticks where |dy| < hoverDeltaThreshold.
//
// A player is flagged only when BOTH thresholds are met, providing a robust
// false-positive-free signal even at the jump apex where Y velocity briefly
// approaches zero naturally.
type FlyCheck struct {
	cfg config.FlyConfig
}

func NewFlyCheck(cfg config.FlyConfig) *FlyCheck { return &FlyCheck{cfg: cfg} }

func (*FlyCheck) Type() string    { return "Fly" }
func (*FlyCheck) SubType() string { return "A" }
func (*FlyCheck) Description() string {
	return "Detects hovering via sustained near-zero Y delta while airborne."
}
func (*FlyCheck) Punishable() bool { return true }

func (c *FlyCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    3,
		MaxBuffer:     5,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates the airborne state using tick-based counters.
func (c *FlyCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	// Creative players can legitimately fly; exempt them entirely.
	if p.IsCreative() {
		return false, ""
	}
	// Players gliding with an elytra are legitimately airborne without falling;
	// their Y velocity is sustained by horizontal momentum, not a cheat.
	if p.IsGliding() {
		return false, ""
	}
	// Players with the Slow Falling effect fall at terminal velocity of ~0.005
	// blocks/tick which is at or below hoverDeltaThreshold.  This causes
	// HoverTicks to accumulate even though the player is legitimately sinking;
	// exempt them entirely to avoid false positives.  Mirrors Oomph's behaviour
	// of skipping the hover check when a gravity-modifying effect is active.
	if _, hasSlowFall := p.EffectAmplifier(packet.EffectSlowFalling); hasSlowFall {
		return false, ""
	}
	// Server-applied knockback (SetActorMotion / MotionPredictionHints) launches
	// the player into the air; the resulting airborne phase is legitimate.  Skip
	// the check until the knockback grace window expires to avoid false positives
	// on players who are knocked upward for many ticks. Mirrors Oomph's motion-
	// update exemption for externally applied velocities.
	if p.HasKnockbackGrace() {
		return false, ""
	}
	// Players who recently exited water may briefly exhibit hover-like Y deltas
	// while transitioning from the water surface to the ground.  Exempt during
	// the same grace window used by NoFall/A (mirrors Oomph's water-exit grace).
	if p.HasRecentWaterExit() {
		return false, ""
	}
	airborne, yDelta, airTicks, hoverTicks := p.FlySnapshot()
	if !airborne {
		return false, ""
	}
	// Players who are swimming have near-zero Y velocity by design; exempt them
	// to avoid false positives when treading water or swimming horizontally.
	_, _, inWater := p.InputSnapshot()
	if inWater {
		return false, ""
	}
	// Grace period: skip the entire jump arc before starting to inspect.
	// Extend the grace period proportionally when JumpBoost is active, since
	// the effect increases jump height and thus arc duration. Oomph accounts
	// for this by lengthening the simulation-reliable window for JumpBoost.
	graceTicks := flyGraceTicks
	if amp, active := p.EffectAmplifier(packet.EffectJumpBoost); active {
		graceTicks += int(amp+1) * flyJumpBoostGracePerLevel
	}
	if airTicks <= graceTicks {
		return false, ""
	}
	// Upward-fly detection (GrimAC "Fly type 2"):
	// After the grace period, a legitimate player must be falling (yDelta < 0).
	// If Y delta is still above the hover threshold, the player is continuing to
	// rise, which is only possible with a fly cheat — vanilla jump velocity has
	// already decayed to negative by this point.
	if yDelta > flyUpwardThreshold {
		return true, fmt.Sprintf("upward_fly air_ticks=%d y_delta=%.4f grace=%d", airTicks, yDelta, graceTicks)
	}
	// Hover-fly detection: flag when the Y displacement has been near zero for
	// enough ticks to rule out a jump apex or other transient near-zero Y-velocity
	// scenario.
	if hoverTicks >= flyMinHoverTicks {
		return true, fmt.Sprintf("hover air_ticks=%d hover_ticks=%d grace=%d", airTicks, hoverTicks, graceTicks)
	}
	return false, ""
}
