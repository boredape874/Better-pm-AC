package combat

import (
"fmt"
"math"

"github.com/boredape874/Better-pm-AC/anticheat/meta"
"github.com/boredape874/Better-pm-AC/anticheat/data"
"github.com/boredape874/Better-pm-AC/config"
)

// AimCheck (AimA) detects suspiciously "round" yaw deltas that indicate
// aim-assistance software or aim-bots. Human mouse movement naturally produces
// irrational floating-point deltas; bots often rotate in exact multiples of
// clean fractions.
//
// Algorithm mirrors Oomph's AimA:
//  1. Take the absolute yaw delta this tick.
//  2. Round it to 1 decimal (r1) and to 5 decimals (r2).
//  3. If |r2 - r1| ≤ 3e-5 the delta is suspiciously "round" → fail.
//
// Implements anticheat.Detection.
type AimCheck struct {
cfg config.AimConfig
}

func NewAimCheck(cfg config.AimConfig) *AimCheck { return &AimCheck{cfg: cfg} }

func (*AimCheck) Type() string        { return "Aim" }
func (*AimCheck) SubType() string     { return "A" }
func (*AimCheck) Description() string { return "Checks for unnaturally round yaw rotation deltas." }
func (*AimCheck) Punishable() bool    { return true }

func (c *AimCheck) DefaultMetadata() *meta.DetectionMetadata {
return &meta.DetectionMetadata{
FailBuffer:    5,
MaxBuffer:     5,
MaxViolations: float64(c.cfg.Violations),
}
}

func (*AimCheck) Name() string { return "Aim/A" }

// Check evaluates the yaw delta from the latest PlayerAuthInput.
// Returns (flagged, debugInfo), plus a passAmount to reduce the buffer when
// the delta is clean.
func (c *AimCheck) Check(p *data.Player) (flagged bool, info string, passAmount float64) {
if !c.cfg.Enabled {
return false, "", 0
}
yawDelta, _ := p.RotationSnapshot()
if yawDelta < 1e-3 {
// Ignore near-zero rotation (player not turning).
return false, "", 0
}

r1 := round32(yawDelta, 1)
r2 := round32(yawDelta, 5)
diff := float32(math.Abs(float64(r2 - r1)))

if diff <= 3e-5 {
return true, fmt.Sprintf("yaw_delta=%.5f r1=%.1f diff=%.6f", yawDelta, r1, diff), 0
}
// Clean delta — signal the caller to pass 0.1 buffer credit (Oomph value).
return false, "", 0.1
}

// round32 rounds a float32 to the given number of decimal places.
func round32(val float32, precision int) float32 {
pwr := math.Pow10(precision)
return float32(math.Round(float64(val)*pwr) / pwr)
}
