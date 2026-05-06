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
// Uses CommittedPos delta (server-authoritative) to prevent cheats that report
// a low client velocity while actually moving at full speed.
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

// Check evaluates the player's horizontal speed during active item use using
// CommittedPos delta. This prevents cheats that report a low client velocity
// while actually moving at full speed.
func (c *NoSlowCheck) Check(p *data.Player) (bool, string) {
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
		return true, fmt.Sprintf("speed=%.4f max=%.4f", speed, max)
	}
	return false, ""
}
