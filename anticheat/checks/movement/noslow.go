package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// NoSlowCheck (NoSlow/A) detects players that move at full speed while using an
// item (eating, drawing a bow, raising a shield). In vanilla Bedrock Edition,
// item use reduces horizontal movement to approximately 27% of base walking
// speed. Cheats that bypass this restriction allow the player to sprint or walk
// at normal speed while using items.
//
// Detection strategy (mirrors Oomph's NoSlow check):
//  1. The proxy tracks "isUsingItem" via InputFlagStartUsingItem (set on entry)
//     and clears it when InputFlagPerformItemInteraction fires (use complete or
//     cancelled). This sticky flag is stored in data.Player.UsingItem.
//  2. On each OnInput tick where UsingItem is true, the player's horizontal
//     speed is compared against cfg.MaxItemUseSpeed (default 0.21 b/tick).
//  3. If the speed exceeds the threshold the check fails.
//
// Exemptions:
//   - Creative players (can use items without speed penalty on some servers).
//   - Players under active knockback grace (server-applied velocity).
//   - Players in water (swimming speed is subject to different rules).
//
// Implements anticheat.Detection.
type NoSlowCheck struct {
	cfg config.NoSlowConfig
}

func NewNoSlowCheck(cfg config.NoSlowConfig) *NoSlowCheck { return &NoSlowCheck{cfg: cfg} }

func (*NoSlowCheck) Type() string    { return "NoSlow" }
func (*NoSlowCheck) SubType() string { return "A" }
func (*NoSlowCheck) Description() string {
	return "Detects moving at full speed while using an item (eating, bow, shield)."
}
func (*NoSlowCheck) Punishable() bool { return true }
func (c *NoSlowCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *NoSlowCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Require three consecutive fast ticks before recording a violation
		// to avoid false positives on the brief acceleration frame at the
		// start of item use before the client has applied the speed reduction.
		FailBuffer:    3,
		MaxBuffer:     4,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates the player's horizontal speed during active item use.
func (c *NoSlowCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	// Only check while the player is actively using an item.
	_, _, inWater, _, usingItem := p.InputSnapshotFull()
	if !usingItem {
		return false, ""
	}
	if p.IsCreative() {
		return false, ""
	}
	// Server-applied knockback can push the player at speed while they happen
	// to be mid-item-use; exempt during the grace window.
	if p.HasKnockbackGrace() {
		return false, ""
	}
	// Swimming movement is exempt — underwater the speed reduction from item
	// use is less significant and the interaction is complex.
	if inWater {
		return false, ""
	}

	speed := p.HorizontalSpeed()
	max := float32(c.cfg.MaxItemUseSpeed)
	if speed > max {
		return true, fmt.Sprintf("speed=%.4f max=%.4f", speed, max)
	}
	return false, ""
}
