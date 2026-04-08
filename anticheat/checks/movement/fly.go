package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// flyGraceTicks is the number of consecutive airborne ticks that are always
// exempted from the fly check. This covers the full natural jump arc:
// - Bedrock jump peak is around tick 5-6 (Y velocity ~0.02 b/tick at apex).
// - After the peak the player falls with growing negative Y velocity.
// - A normal ground-level jump lasts ~12 ticks; 20 ticks is well clear.
//
// This mirrors Oomph's simulationIsReliable() which refuses to issue
// corrections for the first several ticks after a state change.
const flyGraceTicks = 20

// flyMinHoverTicks is the minimum number of consecutive ticks with near-zero
// Y displacement that must be observed (after the grace period) before flagging.
// This prevents a single anomalous packet from causing a false positive.
const flyMinHoverTicks = 5

// FlyCheck detects players that hover in mid-air without falling.
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
	airborne, _, airTicks, hoverTicks := p.FlySnapshot()
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
	if airTicks <= flyGraceTicks {
		return false, ""
	}
	// Flag when the Y displacement has been near zero for enough ticks to rule
	// out a jump apex or other transient near-zero Y-velocity scenario.
	if hoverTicks >= flyMinHoverTicks {
		return true, fmt.Sprintf("air_ticks=%d hover_ticks=%d", airTicks, hoverTicks)
	}
	return false, ""
}
