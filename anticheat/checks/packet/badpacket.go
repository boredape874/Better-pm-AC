package packet

import (
"github.com/boredape874/Better-pm-AC/anticheat/meta"
"github.com/boredape874/Better-pm-AC/anticheat/data"
"github.com/boredape874/Better-pm-AC/config"
)

// BadPacketCheck (BadPacketA) detects a PlayerAuthInput packet whose Tick
// field is 0 after a non-zero tick has already been seen. This mirrors
// Oomph's BadPacketA detection and catches clients that reset or spoof their
// simulation frame counter.
// Implements anticheat.Detection.
type BadPacketCheck struct {
cfg config.BadPacketConfig
}

func NewBadPacketCheck(cfg config.BadPacketConfig) *BadPacketCheck {
return &BadPacketCheck{cfg: cfg}
}

func (*BadPacketCheck) Type() string        { return "BadPacket" }
func (*BadPacketCheck) SubType() string     { return "A" }
func (*BadPacketCheck) Description() string { return "Checks if a player's simulation frame is valid." }
func (*BadPacketCheck) Punishable() bool    { return true }

func (c *BadPacketCheck) DefaultMetadata() *meta.DetectionMetadata {
return &meta.DetectionMetadata{
FailBuffer:    1,
MaxBuffer:     1,
MaxViolations: float64(c.cfg.Violations),
}
}

func (*BadPacketCheck) Name() string { return "BadPacket/A" }

// Check evaluates the simulation frame transition.
// tick is the Tick field from the latest PlayerAuthInput.
func (c *BadPacketCheck) Check(p *data.Player, tick uint64) (bool, string) {
if !c.cfg.Enabled {
return false, ""
}
prev := p.SimulationFrame // read before UpdateTick was called — the old value
if prev != 0 && tick == 0 {
return true, "tick_reset"
}
return false, ""
}
