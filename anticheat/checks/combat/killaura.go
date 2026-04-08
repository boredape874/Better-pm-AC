package combat

import (
	"time"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/google/uuid"
)

const killAuraCheckName = "KillAura"

// KillAuraCheck detects abnormally fast attack patterns that indicate automated
// combat (KillAura / AimBot).
//
// Strategy: flag attacks that arrive faster than the minimum vanilla CPS cap
// (vanilla max ≈ 16 CPS on 20-tps server → one hit every ~62 ms).
type KillAuraCheck struct {
	cfg config.KillAuraConfig
}

// NewKillAuraCheck creates a new KillAuraCheck with the given configuration.
func NewKillAuraCheck(cfg config.KillAuraConfig) *KillAuraCheck {
	return &KillAuraCheck{cfg: cfg}
}

// Name returns the human-readable check name.
func (c *KillAuraCheck) Name() string { return killAuraCheckName }

// Check evaluates whether the latest attack arrived suspiciously soon after the
// previous one. target is the UUID of the attacked entity.
func (c *KillAuraCheck) Check(p *data.Player, target uuid.UUID) (flagged bool, violations int) {
	if !c.cfg.Enabled {
		return false, 0
	}

	lastTime, lastTarget := p.LastAttackInfo()

	// Minimum interval between attacks (vanilla ~62 ms at max CPS).
	const minAttackInterval = 50 * time.Millisecond

	now := time.Now()
	elapsed := now.Sub(lastTime)

	// Flag if attacks come in faster than vanilla allows.
	if !lastTime.IsZero() && elapsed < minAttackInterval {
		violations = p.AddViolation(killAuraCheckName)
		return true, violations
	}

	// Flag multi-target attacks within a single tick (AimBot heuristic).
	if !lastTime.IsZero() && elapsed < minAttackInterval && lastTarget != target {
		violations = p.AddViolation(killAuraCheckName)
		return true, violations
	}

	// Record this attack.
	p.RecordAttack(target)
	return false, 0
}
