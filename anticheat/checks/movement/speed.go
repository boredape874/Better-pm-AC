package movement

import (
"fmt"

"github.com/boredape874/Better-pm-AC/anticheat/meta"
"github.com/boredape874/Better-pm-AC/anticheat/data"
"github.com/boredape874/Better-pm-AC/config"
)

// SpeedCheck flags players whose horizontal speed exceeds the configured limit.
// Implements anticheat.Detection.
type SpeedCheck struct {
cfg config.SpeedConfig
}

func NewSpeedCheck(cfg config.SpeedConfig) *SpeedCheck { return &SpeedCheck{cfg: cfg} }

func (*SpeedCheck) Type() string        { return "Speed" }
func (*SpeedCheck) SubType() string     { return "A" }
func (*SpeedCheck) Description() string { return "Detects horizontal movement speed exceeding vanilla limits." }
func (*SpeedCheck) Punishable() bool    { return true }

func (c *SpeedCheck) DefaultMetadata() *meta.DetectionMetadata {
return &meta.DetectionMetadata{
FailBuffer:    2,
MaxBuffer:     4,
MaxViolations: float64(c.cfg.Violations),
}
}

// Name returns the human-readable check name (kept for legacy log calls).
func (*SpeedCheck) Name() string { return "Speed/A" }

// Check evaluates the player's horizontal speed.
// Returns (flagged, debugInfo).
func (c *SpeedCheck) Check(p *data.Player) (bool, string) {
if !c.cfg.Enabled {
return false, ""
}
speed := p.HorizontalSpeed()
// Convert blocks/tick (config) to blocks/second (20 TPS).
maxSpeedPerSec := float32(c.cfg.MaxSpeed * 20)
if speed > maxSpeedPerSec {
return true, fmt.Sprintf("speed=%.3f max=%.3f", speed, maxSpeedPerSec)
}
return false, ""
}
