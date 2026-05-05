package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// noFallBMinSpoofTicks is the number of consecutive suspicious OnGround ticks
// required before flagging. A single tick with OnGround=true and fast downward
// velocity is possible at the moment of landing; requiring multiple consecutive
// ticks eliminates that one-tick false positive.
const noFallBMinSpoofTicks = 4

// NoFallBCheck detects players that continuously send OnGround=true while their
// Y position is still descending faster than noFallBSpeedThreshold. This pattern
// is the signature of a "persistent OnGround spoof" NoFall cheat variant: the
// client sets the OnGround flag on every packet so the server never accumulates
// a fall distance, preventing the NoFall/A check from triggering.
//
// Algorithm (mirrors GrimAC's OnGround-spoof detection):
//  1. Every tick where OnGround=true AND yDelta < -noFallBSpeedThreshold,
//     increment GroundFallTicks (tracked in data.Player.UpdatePosition).
//  2. If GroundFallTicks reaches noFallBMinSpoofTicks, flag.
//  3. GroundFallTicks resets whenever the player is genuinely airborne or
//     their Y delta is not significantly negative.
//
// Implements anticheat.Detection.
type NoFallBCheck struct {
	cfg       config.NoFallBConfig
	authority *config.AuthorityConfig
}

func NewNoFallBCheck(cfg config.NoFallBConfig) *NoFallBCheck {
	return &NoFallBCheck{cfg: cfg}
}

// SetAuthority wires the shared AuthorityConfig.
func (c *NoFallBCheck) SetAuthority(a *config.AuthorityConfig) { c.authority = a }

func (*NoFallBCheck) Type() string    { return "NoFall" }
func (*NoFallBCheck) SubType() string { return "B" }
func (*NoFallBCheck) Description() string {
	return "Detects persistent OnGround=true while Y position is falling (OnGround spoof)."
}
func (*NoFallBCheck) Punishable() bool { return true }
func (c *NoFallBCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *NoFallBCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    2,
		MaxBuffer:     3,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// CheckLegacy is the original GroundFallTicks-based implementation of NoFall/B
// (γ.3.7 migration: retained as fallback when MovementAuth is disabled).
func (c *NoFallBCheck) CheckLegacy(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if p.IsCreative() {
		return false, ""
	}
	if p.IsGliding() {
		return false, ""
	}
	_, _, inWater, _, _ := p.InputSnapshotFull()
	if inWater {
		return false, ""
	}
	if p.HasRecentWaterExit() {
		return false, ""
	}
	// Server-applied knockback can push the player downward while they are
	// genuinely on (or very near) the ground, causing GroundFallTicks to
	// accumulate. Exempt during the knockback grace window to avoid falsely
	// flagging players who were hit near the ground. Mirrors Oomph's motion-
	// update exemption for externally applied velocities.
	if p.HasKnockbackGrace() {
		return false, ""
	}

	groundFallTicks, yDelta, onGround := p.GroundFallSnapshot()

	// Only applicable when the client claims to be on the ground.
	if !onGround {
		return false, ""
	}

	if groundFallTicks >= noFallBMinSpoofTicks {
		return true, fmt.Sprintf("ground_fall_ticks=%d y_delta=%.4f", groundFallTicks, yDelta)
	}
	return false, ""
}

// Check evaluates whether the player is continuously spoofing OnGround=true
// while their Y position is descending at speed. When MovementAuth is enabled,
// committed Y delta is used instead of client-reported Velocity[1].
func (c *NoFallBCheck) Check(p *data.Player) (bool, string) {
	if c.authority != nil && c.authority.MovementAuth {
		return c.checkCommitted(p)
	}
	return c.CheckLegacy(p)
}

// checkCommitted uses CommittedPos Y delta for OnGround-spoof detection.
// The committed Y delta is server-authoritative; using it prevents cheats
// that manipulate the reported Y while still falling.
func (c *NoFallBCheck) checkCommitted(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if p.IsCreative() {
		return false, ""
	}
	if p.IsGliding() {
		return false, ""
	}
	_, _, inWater, _, _ := p.InputSnapshotFull()
	if inWater {
		return false, ""
	}
	if p.HasRecentWaterExit() {
		return false, ""
	}
	if p.HasKnockbackGrace() {
		return false, ""
	}

	groundFallTicks, _, onGround := p.GroundFallSnapshot()
	if !onGround {
		return false, ""
	}

	// Use committed Y delta as the falling signal.
	committedDeltaY := p.CommittedPos()[1] - p.PrevCommittedPos()[1]

	if groundFallTicks >= noFallBMinSpoofTicks {
		return true, fmt.Sprintf("committed_ground_fall_ticks=%d committed_deltaY=%.4f", groundFallTicks, committedDeltaY)
	}
	return false, ""
}
