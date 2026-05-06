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

// FlyCheck detects hovering and upward flight while airborne. It tracks two
// counters updated by data.Player.UpdatePosition:
//   - AirTicks:   consecutive ticks airborne since last grounding.
//   - HoverTicks: consecutive ticks where |dy| < hoverDeltaThreshold.
//
// A player is flagged only when BOTH thresholds are met, providing a robust
// false-positive-free signal even at the jump apex where Y velocity briefly
// approaches zero naturally. Uses CommittedPos delta (server-authoritative).
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
func (c *FlyCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *FlyCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    3,
		MaxBuffer:     5,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates the airborne state using committed-position vertical delta.
// deltaY = CommittedPos[1] - PrevCommittedPos[1] is server-authoritative;
// a positive deltaY after the grace period indicates impossible upward flight.
func (c *FlyCheck) Check(p *data.Player) (bool, string) {
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
	// Use the airborne counters for grace-period gating (still reliable even
	// under committed-pos mode because they are driven by committed OnGround state).
	airborne, _, airTicks, hoverTicks, _ := p.FlySnapshot()
	if !airborne {
		return false, ""
	}
	_, _, inWater, crawling, _ := p.InputSnapshotFull()
	if inWater || crawling {
		return false, ""
	}
	if p.HasTerrainCollision() {
		return false, ""
	}
	graceTicks := flyGraceTicks
	if amp, active := p.EffectAmplifier(packet.EffectJumpBoost); active {
		graceTicks += int(amp+1) * flyJumpBoostGracePerLevel
	}
	if airTicks <= graceTicks {
		return false, ""
	}
	// Use committed-position vertical delta instead of client-reported Velocity.
	deltaY := p.CommittedPos()[1] - p.PrevCommittedPos()[1]
	if deltaY > flyUpwardThreshold {
		return true, fmt.Sprintf("upward_fly air_ticks=%d deltaY=%.4f grace=%d", airTicks, deltaY, graceTicks)
	}
	if hoverTicks >= flyMinHoverTicks {
		return true, fmt.Sprintf("hover air_ticks=%d hover_ticks=%d grace=%d deltaY=%.4f", airTicks, hoverTicks, graceTicks, deltaY)
	}
	return false, ""
}
