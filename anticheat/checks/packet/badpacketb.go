package packet

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// BadPacketBCheck (BadPacket/B) detects PlayerAuthInput packets whose Pitch
// field is outside the valid range of [-90, 90] degrees.  A legitimate Bedrock
// client clamps the pitch to this range before sending; any value outside it
// indicates a modified client or packet injector.
//
// This mirrors GrimAC's BadPackets check C, which flags implausible pitch values.
type BadPacketBCheck struct {
	cfg config.BadPacketBConfig
}

func NewBadPacketBCheck(cfg config.BadPacketBConfig) *BadPacketBCheck {
	return &BadPacketBCheck{cfg: cfg}
}

func (*BadPacketBCheck) Type() string    { return "BadPacket" }
func (*BadPacketBCheck) SubType() string { return "B" }
func (*BadPacketBCheck) Description() string {
	return "Checks that PlayerAuthInput pitch is within the valid [-90, 90] range."
}
func (*BadPacketBCheck) Punishable() bool { return true }

func (c *BadPacketBCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    1,
		MaxBuffer:     1,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates whether the pitch value from the current PlayerAuthInput
// packet is within the range a legitimate client would send.
func (c *BadPacketBCheck) Check(p *data.Player, pitch float32) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if pitch < -90 || pitch > 90 {
		return true, fmt.Sprintf("pitch=%.4f", pitch)
	}
	return false, ""
}
