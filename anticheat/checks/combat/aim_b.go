package combat

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// aimBMinConstPitchTicks is the number of consecutive ticks of yaw rotation
// with zero pitch change that must be observed before flagging. A human player
// occasionally keeps their pitch locked for a few ticks while panning
// horizontally; only a sustained pattern (here: 15 ticks ≈ 0.75 s) is flagged.
// This mirrors GrimAC's AimB "cinematic camera" threshold.
const aimBMinConstPitchTicks = 15

// aimBMouseOnly mirrors Aim/A: only mouse clients produce detectable locked-pitch
// rotation patterns; touch/gamepad clients legitimately move yaw in large fixed
// steps which could look like constant pitch at first glance.
const aimBInputModeMouse = uint32(1)

// AimBCheck (Aim/B) detects aimbot software that continuously rotates the
// player's yaw to track a target while keeping the pitch perfectly constant
// (locked) for an extended period.
//
// Human players naturally adjust their pitch while panning horizontally —
// targets are rarely at exactly the same elevation for more than a few ticks.
// An aimbot that only performs horizontal yaw tracking to follow a target at
// the same Y level will exhibit exactly this signature: yaw delta > 0 for many
// consecutive ticks with pitchDelta == 0 throughout.
//
// Algorithm (mirrors GrimAC AimB "constant-pitch-while-rotating" heuristic):
//  1. Skip non-mouse clients (touch/gamepad can legitimately move yaw only).
//  2. Track ConstPitchTicks in data.Player.UpdateRotation:
//     increment when |yawDelta| > 0.5° and pitchDelta == 0.
//  3. Flag when ConstPitchTicks >= aimBMinConstPitchTicks.
//
// Implements anticheat.Detection.
type AimBCheck struct {
	cfg config.AimBConfig
}

func NewAimBCheck(cfg config.AimBConfig) *AimBCheck { return &AimBCheck{cfg: cfg} }

func (*AimBCheck) Type() string    { return "Aim" }
func (*AimBCheck) SubType() string { return "B" }
func (*AimBCheck) Description() string {
	return "Detects sustained yaw rotation with locked pitch (aimbot signature, mouse only)."
}
func (*AimBCheck) Punishable() bool { return true }
func (c *AimBCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *AimBCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// The ConstPitchTicks counter already requires sustained behaviour;
		// the buffer adds a second confirmation layer.
		FailBuffer:    2,
		MaxBuffer:     3,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates whether the player is exhibiting the locked-pitch rotation
// pattern characteristic of an aimbot. Must be called after UpdateRotation.
func (c *AimBCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	// Touch and gamepad clients legitimately move yaw without adjusting pitch;
	// only apply this check to mouse clients.
	if p.GetInputMode() != aimBInputModeMouse {
		return false, ""
	}
	constTicks := p.ConstPitchSnapshot()
	if constTicks >= aimBMinConstPitchTicks {
		return true, fmt.Sprintf("const_pitch_ticks=%d", constTicks)
	}
	return false, ""
}
