package movement

import (
"fmt"

"github.com/boredape874/Better-pm-AC/anticheat/data"
"github.com/boredape874/Better-pm-AC/anticheat/meta"
"github.com/boredape874/Better-pm-AC/config"
)

// SpeedCheck flags players whose horizontal movement exceeds the configured
// limit per tick. Velocity is now a raw positional delta (blocks/tick) rather
// than a wall-clock-derived blocks/second value, matching how Oomph computes
// displacement: it compares the positional delta from one PlayerAuthInput to
// the next against its simulated expectation.
type SpeedCheck struct {
cfg config.SpeedConfig
}

func NewSpeedCheck(cfg config.SpeedConfig) *SpeedCheck { return &SpeedCheck{cfg: cfg} }

func (*SpeedCheck) Type() string        { return "Speed" }
func (*SpeedCheck) SubType() string     { return "A" }
func (*SpeedCheck) Description() string { return "Detects horizontal movement exceeding vanilla limits (blocks/tick)." }
func (*SpeedCheck) Punishable() bool    { return true }

func (c *SpeedCheck) DefaultMetadata() *meta.DetectionMetadata {
return &meta.DetectionMetadata{
FailBuffer:    2,
MaxBuffer:     4,
MaxViolations: float64(c.cfg.Violations),
}
}

// Check evaluates the player's horizontal displacement in blocks/tick.
// The config MaxSpeed field is already expressed in blocks/tick (default 0.7),
// so no unit conversion is required.
func (c *SpeedCheck) Check(p *data.Player) (bool, string) {
if !c.cfg.Enabled {
return false, ""
}
speed := p.HorizontalSpeed() // blocks/tick
maxSpeed := float32(c.cfg.MaxSpeed)
if speed > maxSpeed {
return true, fmt.Sprintf("speed=%.4f max=%.4f", speed, maxSpeed)
}
return false, ""
}
