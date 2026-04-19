package combat

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// AutoClickerBCheck (AutoClickerB) detects players whose inter-click interval
// standard deviation is suspiciously low, indicating automated clicking.
//
// Human players produce naturally irregular click timings with a standard
// deviation typically above 15 ms. Autoclicker software clicks with mechanical
// precision and produces std dev values close to 0 ms (often < 5 ms).
//
// Algorithm (mirrors GrimAC AutoClickerB):
//  1. Collect inter-click intervals (ms) from the rolling one-second window.
//  2. Compute the standard deviation of those intervals.
//  3. Flag when std dev < StdDevThreshold AND at least MinSamples intervals
//     have been observed, ensuring statistical significance.
//
// Implements anticheat.Detection.
type AutoClickerBCheck struct {
	cfg config.AutoClickerBConfig
}

func NewAutoClickerBCheck(cfg config.AutoClickerBConfig) *AutoClickerBCheck {
	return &AutoClickerBCheck{cfg: cfg}
}

func (*AutoClickerBCheck) Type() string    { return "AutoClicker" }
func (*AutoClickerBCheck) SubType() string { return "B" }
func (*AutoClickerBCheck) Description() string {
	return "Checks for unnaturally consistent click intervals (autoclicker precision)."
}
func (*AutoClickerBCheck) Punishable() bool { return true }
func (*AutoClickerBCheck) Policy() meta.MitigatePolicy { return meta.PolicyKick }

func (c *AutoClickerBCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    3,
		MaxBuffer:     5,
		MaxViolations: float64(c.cfg.Violations),
		TrustDuration: 30 * 20,
	}
}

// Check evaluates the statistical regularity of the player's click timing.
func (c *AutoClickerBCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	stdDev, n := p.ClickIntervalStdDev()
	if n < c.cfg.MinSamples {
		return false, ""
	}
	if stdDev < c.cfg.StdDevThreshold {
		return true, fmt.Sprintf("std_dev=%.2fms samples=%d threshold=%.1fms", stdDev, n, c.cfg.StdDevThreshold)
	}
	return false, ""
}
