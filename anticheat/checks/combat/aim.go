package combat

import (
	"fmt"
	"math"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// inputModeMouse matches packet.InputModeMouse from gophertunnel (= 1).
// We inline the constant to avoid importing the packet package here.
const inputModeMouse = uint32(1)

// AimCheck (AimA) detects suspiciously "round" yaw deltas that indicate
// aim-assistance software. Algorithm mirrors Oomph's AimA exactly:
//  1. Skip non-mouse clients entirely (touch / gamepad rotate differently
//     and produce naturally "round" deltas — Oomph: if InputMode != Mouse { return }).
//  2. Take the absolute yaw delta this tick.
//  3. Round it to 1 decimal (r1) and 5 decimals (r2).
//  4. If |r2 - r1| <= 3e-5 the delta is suspiciously round → fail.
type AimCheck struct {
	cfg config.AimConfig
}

func NewAimCheck(cfg config.AimConfig) *AimCheck { return &AimCheck{cfg: cfg} }

func (*AimCheck) Type() string    { return "Aim" }
func (*AimCheck) SubType() string { return "A" }
func (*AimCheck) Description() string {
	return "Checks for unnaturally round yaw rotation deltas (mouse clients only)."
}
func (*AimCheck) Punishable() bool { return true }
func (c *AimCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *AimCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    5,
		MaxBuffer:     5,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates the yaw delta from the latest PlayerAuthInput.
// Returns (flagged, debugInfo, passAmount).
func (c *AimCheck) Check(p *data.Player) (flagged bool, info string, passAmount float64) {
	if !c.cfg.Enabled {
		return false, "", 0
	}
	// Oomph AimA: "This check will only apply to players rotating their camera
	// with a mouse." Touch and gamepad clients legitimately produce quantised
	// rotations that would generate false positives here.
	if p.GetInputMode() != inputModeMouse {
		return false, "", 0
	}

	yawDelta, _ := p.RotationSnapshot()
	if yawDelta < 1e-3 {
		// Player is not moving their camera horizontally; no information to
		// evaluate, but the buffer should still slowly decay to avoid
		// accumulating towards a false positive during stationary periods.
		return false, "", 0.05
	}

	r1 := round32(yawDelta, 1)
	r2 := round32(yawDelta, 5)
	diff := float32(math.Abs(float64(r2 - r1)))

	if diff <= 3e-5 {
		return true, fmt.Sprintf("yaw_delta=%.5f r1=%.1f diff=%.6f", yawDelta, r1, diff), 0
	}
	return false, "", 0.1
}

// round32 rounds a float32 to the given number of decimal places.
func round32(val float32, precision int) float32 {
	pwr := math.Pow10(precision)
	return float32(math.Round(float64(val)*pwr) / pwr)
}
