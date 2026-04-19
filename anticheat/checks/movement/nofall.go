package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// noFallDamageThreshold is the minimum fall distance (blocks) at which vanilla
// inflicts fall damage. Falls below this distance are ignored.
const noFallDamageThreshold = float32(3.0)

// NoFallCheck detects players that land after falling more than 3 blocks
// without triggering fall damage, indicating NoFall or anti-damage cheats.
// Implements anticheat.Detection.
type NoFallCheck struct {
	cfg config.NoFallConfig
}

func NewNoFallCheck(cfg config.NoFallConfig) *NoFallCheck { return &NoFallCheck{cfg: cfg} }

func (*NoFallCheck) Type() string    { return "NoFall" }
func (*NoFallCheck) SubType() string { return "A" }
func (*NoFallCheck) Description() string {
	return "Detects landing after a significant fall without damage."
}
func (*NoFallCheck) Punishable() bool { return true }
func (c *NoFallCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *NoFallCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    1,
		MaxBuffer:     2,
		MaxViolations: float64(c.cfg.Violations),
	}
}

func (*NoFallCheck) Name() string { return "NoFall/A" }

// Check evaluates the player's fall-landing transition.
// Players who are (or were recently) swimming are exempt because water absorbs
// fall damage regardless of fall distance.
func (c *NoFallCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	justLanded, fallDist := p.NoFallSnapshot()
	if !justLanded || fallDist <= noFallDamageThreshold {
		return false, ""
	}
	// Exempt players who are currently in water — water absorbs fall damage
	// in vanilla Bedrock Edition regardless of fall distance.
	_, _, inWater, _, _ := p.InputSnapshotFull()
	if inWater {
		return false, ""
	}
	// Exempt players who recently exited water within the grace window.
	// A player can fall inside water and land on the ground just after
	// exiting; the accumulated fall distance is not meaningful for damage.
	if p.HasRecentWaterExit() {
		return false, ""
	}
	// Exempt players with the Slow Falling effect. In Bedrock Edition, Slow
	// Falling completely negates fall damage regardless of fall distance, so
	// a player who lands after a long fall under this effect should not flag.
	// Mirrors Oomph's effect-exemption logic in the NoFall check.
	if _, hasSlowFall := p.EffectAmplifier(packet.EffectSlowFalling); hasSlowFall {
		return false, ""
	}
	return true, fmt.Sprintf("fall_dist=%.2f threshold=%.1f", fallDist, noFallDamageThreshold)
}
