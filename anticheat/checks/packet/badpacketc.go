package packet

import (
	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// BadPacketCCheck (BadPacket/C) detects PlayerAuthInput packets that have
// both the Sprinting and Sneaking flags set at the same time.
//
// In vanilla Bedrock Edition, sprinting and sneaking are mutually exclusive:
// the client clears the sprint flag when it enters sneak mode and vice versa.
// Any packet that carries both flags simultaneously originates from a modified
// client or packet injector.
//
// This mirrors GrimAC's BadPackets check for simultaneous sprint+sneak.
type BadPacketCCheck struct {
	cfg config.BadPacketCConfig
}

func NewBadPacketCCheck(cfg config.BadPacketCConfig) *BadPacketCCheck {
	return &BadPacketCCheck{cfg: cfg}
}

func (*BadPacketCCheck) Type() string    { return "BadPacket" }
func (*BadPacketCCheck) SubType() string { return "C" }
func (*BadPacketCCheck) Description() string {
	return "Detects simultaneous Sprinting+Sneaking flags, impossible in vanilla."
}
func (*BadPacketCCheck) Punishable() bool { return true }
func (c *BadPacketCCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *BadPacketCCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    1,
		MaxBuffer:     1,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates whether the current input tick has an impossible combination
// of Sprinting and Sneaking flags both set to true.
func (c *BadPacketCCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	sprinting, sneaking, _, _, _ := p.InputSnapshotFull()
	if sprinting && sneaking {
		return true, "sprint+sneak"
	}
	return false, ""
}
