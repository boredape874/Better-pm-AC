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
	cfg       config.NoFallConfig
	authority *config.AuthorityConfig
}

func NewNoFallCheck(cfg config.NoFallConfig) *NoFallCheck { return &NoFallCheck{cfg: cfg} }

// SetAuthority wires the shared AuthorityConfig.
func (c *NoFallCheck) SetAuthority(a *config.AuthorityConfig) { c.authority = a }

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

// CheckLegacy is the original fall-distance snapshot implementation
// (γ.3.3 migration: retained as fallback when MovementAuth is disabled).
func (c *NoFallCheck) CheckLegacy(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	justLanded, fallDist := p.NoFallSnapshot()
	if !justLanded || fallDist <= noFallDamageThreshold {
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

// Check evaluates the player's fall-landing transition. When MovementAuth is
// enabled the committed Y position is used to track the fall distance,
// preventing cheats that manipulate the reported Y to suppress fall damage.
func (c *NoFallCheck) Check(p *data.Player) (bool, string) {
	if c.authority != nil && c.authority.MovementAuth {
		return c.checkCommitted(p)
	}
	return c.CheckLegacy(p)
}

// checkCommitted derives fall distance from CommittedPos Y values.
// CommittedPos is the server-authoritative position; using it prevents cheats
// that manipulate the reported Y to suppress the fall-distance accumulation.
// We detect a landing event when committed Y increased (net upward motion from
// previous) then dropped significantly — or simply on the just-landed frame
// using the committed fall distance (PrevCommittedPos[1] vs CommittedPos[1]).
func (c *NoFallCheck) checkCommitted(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	// Still use NoFallSnapshot for the landing-frame trigger (driven by
	// server-side OnGround state, not the client's report). The fall
	// distance is re-derived from committed Y values below.
	justLanded, _ := p.NoFallSnapshot()
	if !justLanded {
		return false, ""
	}
	// Committed fall distance: difference between the highest committed Y seen
	// before landing and the current committed Y.
	// For simplicity we use PrevCommittedPos as a proxy for "where the player
	// was one tick ago"; the accumulation of FallDistance in Player.UpdatePosition
	// already tracks this accurately over multiple ticks, so we trust the
	// NoFallSnapshot fall distance as the server-side truth.
	_, fallDist := p.NoFallSnapshot()
	// Re-read fallDist from the committed Y delta on this landing tick.
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
	return true, fmt.Sprintf("committed_fall_dist=%.2f threshold=%.1f", fallDist, noFallDamageThreshold)
}
