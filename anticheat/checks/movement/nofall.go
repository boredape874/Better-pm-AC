package movement

import (
	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
)

const noFallCheckName = "NoFall"

// NoFallCheck detects players that take no fall damage after falling a
// significant distance. It flags the player when they land (transition to
// onGround) after falling more than the vanilla safe-fall height (3 blocks).
type NoFallCheck struct {
	cfg config.NoFallConfig
}

// NewNoFallCheck creates a new NoFallCheck with the given configuration.
func NewNoFallCheck(cfg config.NoFallConfig) *NoFallCheck {
	return &NoFallCheck{cfg: cfg}
}

// Name returns the human-readable check name.
func (c *NoFallCheck) Name() string { return noFallCheckName }

// Check inspects the player's fall-distance state.
// It returns true and the violation count when a violation is detected.
func (c *NoFallCheck) Check(p *data.Player) (flagged bool, violations int) {
	if !c.cfg.Enabled {
		return false, 0
	}

	justLanded, fallDist := p.NoFallSnapshot()
	// Minecraft vanilla: fall damage starts after 3 blocks fall distance.
	const safeFallBlocks = float32(3.0)
	if justLanded && fallDist > safeFallBlocks {
		violations = p.AddViolation(noFallCheckName)
		return true, violations
	}

	return false, 0
}
