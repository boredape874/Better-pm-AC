package combat

import (
	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

const reachCheckName = "Reach"

// ReachCheck flags players whose attack target is farther away than the
// configured maximum reach distance.
type ReachCheck struct {
	cfg config.ReachConfig
}

// NewReachCheck creates a new ReachCheck with the given configuration.
func NewReachCheck(cfg config.ReachConfig) *ReachCheck {
	return &ReachCheck{cfg: cfg}
}

// Name returns the human-readable check name.
func (c *ReachCheck) Name() string { return reachCheckName }

// Check evaluates the distance between the attacker and the target at the time
// of an attack. targetPos is the last known position of the attacked entity.
func (c *ReachCheck) Check(p *data.Player, targetPos mgl32.Vec3) (flagged bool, violations int) {
	if !c.cfg.Enabled {
		return false, 0
	}

	attackerPos := p.CurrentPosition()
	dist := attackerPos.Sub(targetPos).Len()

	if dist > float32(c.cfg.MaxReach) {
		violations = p.AddViolation(reachCheckName)
		return true, violations
	}

	return false, 0
}

// CheckByID is a convenience wrapper when you have a player map.
func (c *ReachCheck) CheckByID(
	attacker *data.Player,
	_ uuid.UUID,
	targetPos mgl32.Vec3,
) (flagged bool, violations int) {
	return c.Check(attacker, targetPos)
}
