package movement

import (
	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
)

const speedCheckName = "Speed"

// SpeedCheck flags players whose horizontal speed exceeds the configured limit.
type SpeedCheck struct {
	cfg config.SpeedConfig
}

// NewSpeedCheck creates a new SpeedCheck with the given configuration.
func NewSpeedCheck(cfg config.SpeedConfig) *SpeedCheck {
	return &SpeedCheck{cfg: cfg}
}

// Name returns the human-readable check name.
func (c *SpeedCheck) Name() string { return speedCheckName }

// Check evaluates the player's current horizontal speed.
// It returns true and the violation count when a violation is detected.
func (c *SpeedCheck) Check(p *data.Player) (flagged bool, violations int) {
	if !c.cfg.Enabled {
		return false, 0
	}

	speed := p.HorizontalSpeed()
	// Convert configured max speed (blocks/tick at 20 tps) to blocks/second.
	maxSpeedPerSec := float32(c.cfg.MaxSpeed * 20)

	if speed > maxSpeedPerSec {
		violations = p.AddViolation(speedCheckName)
		return true, violations
	}

	return false, 0
}
