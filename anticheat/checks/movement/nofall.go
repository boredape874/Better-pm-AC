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
// Uses CommittedPos Y values to prevent cheats that manipulate the reported Y
// to suppress fall-distance accumulation. Implements anticheat.Detection.
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

// Check evaluates the player's fall-landing transition using the committed Y
// position to derive fall distance, preventing cheats that manipulate the
// reported Y to suppress fall damage.
func (c *NoFallCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	// Still use NoFallSnapshot for the landing-frame trigger (driven by
	// server-side OnGround state, not the client's report).
	justLanded, fallDist := p.NoFallSnapshot()
	if !justLanded {
		return false, ""
	}
	// Re-derive fall distance from committed Y delta on this landing tick.
	committedFallDist := p.PrevCommittedPos()[1] - p.CommittedPos()[1]
	// Use the larger of the two measures to avoid false negatives.
	if committedFallDist > fallDist {
		fallDist = committedFallDist
	}
	if fallDist <= noFallDamageThreshold {
		return false, ""
	}
	_, _, inWater, _, _ := p.InputSnapshotFull()
	if inWater {
		return false, ""
	}
	if p.HasRecentWaterExit() {
		return false, ""
	}
	if _, hasSlowFall := p.EffectAmplifier(packet.EffectSlowFalling); hasSlowFall {
		return false, ""
	}
	return true, fmt.Sprintf("fall_dist=%.2f threshold=%.1f", fallDist, noFallDamageThreshold)
}
