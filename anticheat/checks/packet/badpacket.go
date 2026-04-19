package packet

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
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
func (c *BadPacketCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

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
	// Use the thread-safe getter. UpdateTick has not been called yet at this
	// point, so SimFrame() returns the previous tick value — exactly what we
	// need to compare against the incoming tick.
	prev := p.SimFrame()

	// Case 1 (Oomph BadPacketA): tick reset to 0 after a non-zero tick.
	if prev != 0 && tick == 0 {
		return true, "tick_reset"
	}
	// Case 2: tick going backwards — indicates spoofed or replayed packets.
	if prev != 0 && tick < prev {
		return true, fmt.Sprintf("tick_regression prev=%d new=%d", prev, tick)
	}
	// Case 3: tick jumped by more than 200 ticks (~10 s at 20 TPS) in one
	// packet — no legitimate client should produce this.
	if prev != 0 && tick > prev+200 {
		return true, fmt.Sprintf("tick_jump prev=%d new=%d diff=%d", prev, tick, tick-prev)
	}
	return false, ""
}
