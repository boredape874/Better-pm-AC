package combat

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
)

// attackerEyeHeight is the vertical offset from a player's feet position to
// their eye level in Bedrock Edition (matches proxy.playerEyeHeight).
// Vanilla reach is measured from the attacker's eye position, not their feet.
const attackerEyeHeight = float32(1.62)

// ReachCheck detects attacks on entities that are beyond the configured reach
// distance, indicating Reach or long-sword cheats.
// Implements anticheat.Detection.
type ReachCheck struct {
	cfg config.ReachConfig
}

func NewReachCheck(cfg config.ReachConfig) *ReachCheck { return &ReachCheck{cfg: cfg} }

func (*ReachCheck) Type() string        { return "Reach" }
func (*ReachCheck) SubType() string     { return "A" }
func (*ReachCheck) Description() string { return "Checks if combat reach exceeds the vanilla value." }
func (*ReachCheck) Punishable() bool    { return true }

func (c *ReachCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Matching Oomph ReachA: buffer must reach 1.01 before a violation is
		// counted, and decays naturally at 0.0015 per passing tick.
		FailBuffer:    1.01,
		MaxBuffer:     1.5,
		MaxViolations: float64(c.cfg.Violations),
		TrustDuration: 60 * 20, // 60 s × 20 tps = 1200 ticks
	}
}

func (*ReachCheck) Name() string { return "Reach/A" }

// Check evaluates the distance between the attacker's eye position and the
// target entity's feet position.
//
// The attacker's position stored in data.Player is feet-level (proxy.go
// subtracts playerEyeHeight from the eye-level PlayerAuthInput coordinates).
// We add attackerEyeHeight back here so that the distance measurement matches
// vanilla's eye-to-target calculation rather than feet-to-target, which would
// permit ~1.6 more blocks of reach than intended.
func (c *ReachCheck) Check(p *data.Player, targetPos mgl32.Vec3) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	feetPos := p.CurrentPosition()
	// Convert stored feet position back to eye level for accurate reach check.
	eyePos := mgl32.Vec3{feetPos[0], feetPos[1] + attackerEyeHeight, feetPos[2]}
	dist := eyePos.Sub(targetPos).Len()
	if dist > float32(c.cfg.MaxReach) {
		return true, fmt.Sprintf("dist=%.3f max=%.1f", dist, c.cfg.MaxReach)
	}
	return false, ""
}
