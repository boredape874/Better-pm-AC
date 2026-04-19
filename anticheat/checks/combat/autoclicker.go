package combat

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// AutoClickerCheck (AutoClickerA) detects players clicking faster than the
// configured CPS limit, matching Oomph's AutoclickerA detection strategy.
// CPS is measured over a rolling 1-second window tracked in player.ClickTimestamps.
// Implements anticheat.Detection.
type AutoClickerCheck struct {
	cfg config.AutoClickerConfig
}

func NewAutoClickerCheck(cfg config.AutoClickerConfig) *AutoClickerCheck {
	return &AutoClickerCheck{cfg: cfg}
}

func (*AutoClickerCheck) Type() string    { return "AutoClicker" }
func (*AutoClickerCheck) SubType() string { return "A" }
func (*AutoClickerCheck) Description() string {
	return "Checks if the player is clicking above the configured CPS limit."
}
func (*AutoClickerCheck) Punishable() bool { return true }
func (c *AutoClickerCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *AutoClickerCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    4,
		MaxBuffer:     4,
		MaxViolations: float64(c.cfg.Violations),
		// Violations decay after 30 s (600 ticks at 20 TPS) of clean play
		// so that transient burst-click patterns don't accumulate permanently.
		TrustDuration: 30 * 20,
	}
}

func (*AutoClickerCheck) Name() string { return "AutoClicker/A" }

// Check evaluates whether the player's CPS exceeds the configured limit.
func (c *AutoClickerCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	cps := p.CPS()
	if cps > c.cfg.MaxCPS {
		return true, fmt.Sprintf("cps=%d max=%d", cps, c.cfg.MaxCPS)
	}
	return false, ""
}
