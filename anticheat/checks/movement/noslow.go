package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
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
	cfg       config.NoSlowConfig
	authority *config.AuthorityConfig
}

func NewNoSlowCheck(cfg config.NoSlowConfig) *NoSlowCheck { return &NoSlowCheck{cfg: cfg} }

// SetAuthority wires the shared AuthorityConfig.
func (c *NoSlowCheck) SetAuthority(a *config.AuthorityConfig) { c.authority = a }

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

// CheckLegacy is the original horizontal-speed-during-item-use implementation
// (γ.3.5 migration: retained as fallback when MovementAuth is disabled).
func (c *NoSlowCheck) CheckLegacy(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	_, _, inWater, _, usingItem := p.InputSnapshotFull()
	if !usingItem {
		return false, ""
	}
	if p.IsCreative() {
		return false, ""
	}
	if p.HasKnockbackGrace() {
		return false, ""
	}
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

// Check evaluates the player's horizontal speed during active item use.
// When MovementAuth is enabled, the speed is derived from the committed-position
// delta to prevent cheats that manipulate the reported velocity.
func (c *NoSlowCheck) Check(p *data.Player) (bool, string) {
	if c.authority != nil && c.authority.MovementAuth {
		return c.checkCommitted(p)
	}
	return c.CheckLegacy(p)
}

// checkCommitted uses CommittedPos delta for item-use speed enforcement.
// This prevents cheats that report a low client velocity while actually
// moving at full speed (the committed delta reflects reconciler truth).
func (c *NoSlowCheck) checkCommitted(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	_, _, inWater, _, usingItem := p.InputSnapshotFull()
	if !usingItem {
		return false, ""
	}
	if p.IsCreative() {
		return false, ""
	}
	if p.HasKnockbackGrace() {
		return false, ""
	}
	if inWater {
		return false, ""
	}

	// Horizontal delta from committed positions (server-authoritative).
	delta := p.CommittedPos().Sub(p.PrevCommittedPos())
	speed := mgl32.Vec2{delta[0], delta[2]}.Len()
	max := float32(c.cfg.MaxItemUseSpeed)
	if speed > max {
		return true, fmt.Sprintf("committed_speed=%.4f max=%.4f", speed, max)
	}
	return false, ""
}
