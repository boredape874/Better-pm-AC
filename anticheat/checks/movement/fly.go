package movement

import (
	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
)

const flyCheckName = "Fly"

// FlyCheck detects players that remain airborne with near-zero vertical
// velocity — i.e., hovering — which indicates the use of a fly hack.
type FlyCheck struct {
	cfg config.FlyConfig
}

// NewFlyCheck creates a new FlyCheck with the given configuration.
func NewFlyCheck(cfg config.FlyConfig) *FlyCheck {
	return &FlyCheck{cfg: cfg}
}

// Name returns the human-readable check name.
func (c *FlyCheck) Name() string { return flyCheckName }

// Check evaluates the player's vertical motion.
// It returns true and the violation count when a violation is detected.
func (c *FlyCheck) Check(p *data.Player) (flagged bool, violations int) {
	if !c.cfg.Enabled {
		return false, 0
	}

	airborne, yVel := p.FlySnapshot()
	if !airborne {
		return false, 0
	}

	// A very small absolute Y velocity (blocks/second) while airborne signals
	// hovering. The threshold is kept intentionally low — a legitimately
	// falling player will have yVel < -hoverThreshold within one tick.
	const hoverThreshold = float32(0.08) // blocks/second
	if yVel > -hoverThreshold && yVel < hoverThreshold {
		violations = p.AddViolation(flyCheckName)
		return true, violations
	}

	return false, 0
}
