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
// Y position is still descending faster than noFallBSpeedThreshold. Uses
// CommittedPos Y delta (server-authoritative) for OnGround-spoof detection.
//
// Implements anticheat.Detection.
type NoFallBCheck struct {
	cfg config.NoFallBConfig
}

func NewNoFallBCheck(cfg config.NoFallBConfig) *NoFallBCheck {
	return &NoFallBCheck{cfg: cfg}
}

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

// Check evaluates whether the player is continuously spoofing OnGround=true
// while their Y position is descending at speed. Uses CommittedPos Y delta
// (server-authoritative) as the falling signal.
func (c *NoFallBCheck) Check(p *data.Player) (bool, string) {
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
	// accumulate. Exempt during the knockback grace window.
	if p.HasKnockbackGrace() {
		return false, ""
	}

	groundFallTicks, _, onGround := p.GroundFallSnapshot()

	// Only applicable when the client claims to be on the ground.
	if !onGround {
		return false, ""
	}

	// Use committed Y delta as the falling signal.
	committedDeltaY := p.CommittedPos()[1] - p.PrevCommittedPos()[1]

	if groundFallTicks >= noFallBMinSpoofTicks {
		return true, fmt.Sprintf("ground_fall_ticks=%d committed_deltaY=%.4f", groundFallTicks, committedDeltaY)
	}
	return false, ""
}
