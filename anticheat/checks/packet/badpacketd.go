package packet

import (
	"fmt"
	"math"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
)

// BadPacketDCheck (BadPacket/D) flags PlayerAuthInput packets whose position
// vector contains a NaN or infinite component.
//
// A legitimate Bedrock client never sends NaN or Inf coordinates; these values
// can only originate from a packet injector or heavily modified client.
// Checking for them is free (no per-player state required) and prevents NaN
// from propagating into speed/fly/reach calculations where it could cause
// false positives or silent panics in downstream arithmetic.
//
// Mirrors GrimAC BadPacketsJ ("checks for NaN/Infinite positions").
type BadPacketDCheck struct {
	cfg config.BadPacketDConfig
}

func NewBadPacketDCheck(cfg config.BadPacketDConfig) *BadPacketDCheck {
	return &BadPacketDCheck{cfg: cfg}
}

func (*BadPacketDCheck) Type() string    { return "BadPacket" }
func (*BadPacketDCheck) SubType() string { return "D" }
func (*BadPacketDCheck) Description() string {
	return "Detects NaN or infinite position values in PlayerAuthInput."
}
func (*BadPacketDCheck) Punishable() bool { return true }

func (c *BadPacketDCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    1,
		MaxBuffer:     1,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check returns true if any component of pos is NaN or infinite.
func (c *BadPacketDCheck) Check(pos mgl32.Vec3) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	for i, v := range [3]float32{pos[0], pos[1], pos[2]} {
		f := float64(v)
		if math.IsNaN(f) {
			return true, fmt.Sprintf("axis=%d NaN", i)
		}
		if math.IsInf(f, 0) {
			return true, fmt.Sprintf("axis=%d Inf", i)
		}
	}
	return false, ""
}
