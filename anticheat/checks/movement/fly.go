package movement

import (
"github.com/boredape874/Better-pm-AC/anticheat/meta"
"github.com/boredape874/Better-pm-AC/anticheat/data"
"github.com/boredape874/Better-pm-AC/config"
)

// FlyCheck detects players hovering in mid-air (Y velocity near zero while
// airborne), indicating use of a Fly cheat.
// Implements anticheat.Detection.
type FlyCheck struct {
cfg config.FlyConfig
}

func NewFlyCheck(cfg config.FlyConfig) *FlyCheck { return &FlyCheck{cfg: cfg} }

func (*FlyCheck) Type() string        { return "Fly" }
func (*FlyCheck) SubType() string     { return "A" }
func (*FlyCheck) Description() string { return "Detects hovering in mid-air with near-zero Y velocity." }
func (*FlyCheck) Punishable() bool    { return true }

func (c *FlyCheck) DefaultMetadata() *meta.DetectionMetadata {
return &meta.DetectionMetadata{
FailBuffer:    3,
MaxBuffer:     5,
MaxViolations: float64(c.cfg.Violations),
}
}

func (*FlyCheck) Name() string { return "Fly/A" }

// Check evaluates the player's vertical motion while airborne.
func (c *FlyCheck) Check(p *data.Player) (bool, string) {
if !c.cfg.Enabled {
return false, ""
}
airborne, yVel := p.FlySnapshot()
if !airborne {
return false, ""
}
// A very small absolute Y velocity (blocks/second) while airborne signals
// hovering. A legitimately falling player accumulates ~0.08 b/s per tick
// under normal gravity, so they will quickly exceed this threshold.
const hoverThreshold = float32(0.08) // blocks/second
if yVel > -hoverThreshold && yVel < hoverThreshold {
return true, "hover_yvel=near_zero"
}
return false, ""
}
