package movement

import (
	"fmt"
	"math"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
)

// velocityAMinKB is the minimum horizontal magnitude (blocks/tick) of a
// server-applied velocity that Velocity/A will actually check. Knockback from
// a single hit in vanilla Bedrock is typically 0.4–0.6 b/tick horizontally;
// smaller values (e.g. slight bump from a launch pad) are too weak to produce
// a reliable signal and would only increase false positives.
const velocityAMinKB = float32(0.1)

// VelocityAMinKB is the minimum horizontal magnitude (blocks/tick) of a
// server-applied velocity that Velocity/A will actually check. Exported so the
// anticheat manager can skip the check entirely when the pending knockback is
// below this threshold without constructing a Vec2 unnecessarily.
const VelocityAMinKB = velocityAMinKB

// velocityAMinRatio is the minimum fraction of the applied horizontal knockback
// speed that must be reflected in the player's movement on the first tick after
// the knockback grace window expires. A legitimate player will carry at least
// this fraction of the original impulse (reduced by air resistance and friction);
// an Anti-KB client suppresses the velocity entirely, producing a ratio near 0.
//
// 0.15 (15 %) is deliberately conservative: it accommodates frames where the
// player is on the ground (friction decays horizontal speed ~80 % per tick) and
// multi-tick propagation delay between server and client. Oomph uses a similar
// fraction-based approach in its velocity-check component.
const velocityAMinRatio = float32(0.15)

// VelocityCheck (Velocity/A) detects Anti-KB cheats that suppress or cancel
// the horizontal velocity applied by server-sent SetActorMotion /
// MotionPredictionHints packets.
//
// Algorithm (mirrors Oomph's velocity-check approach):
//  1. When RecordKnockback is called (proxy detected SetActorMotion / MPH),
//     the applied XZ velocity is stored in player.pendingKnockback.
//  2. On the first tick of each OnInput cycle, Check reads and clears
//     pendingKnockback via KnockbackSnapshot().
//  3. If the pending horizontal magnitude exceeds velocityAMinKB and the
//     player's current horizontal velocity (blocks/tick) in the direction of
//     the knockback is less than velocityAMinRatio × applied magnitude,
//     the player is flagged.
//
// The check runs AFTER UpdatePosition so p.HorizontalSpeed() reflects the
// current tick's positional delta, which is the first tick the knockback
// effect should be visible in the client's reported position.
//
// Implements anticheat.Detection.
type VelocityCheck struct {
	cfg config.VelocityConfig
}

func NewVelocityCheck(cfg config.VelocityConfig) *VelocityCheck {
	return &VelocityCheck{cfg: cfg}
}

func (*VelocityCheck) Type() string    { return "Velocity" }
func (*VelocityCheck) SubType() string { return "A" }
func (*VelocityCheck) Description() string {
	return "Detects Anti-KB cheats that suppress server-applied horizontal velocity."
}
func (*VelocityCheck) Punishable() bool { return true }
func (c *VelocityCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *VelocityCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Two consecutive knockback absorptions are required before a violation
		// is counted. A single absorbed knockback could be a network edge case.
		FailBuffer:    2,
		MaxBuffer:     3,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates whether the player absorbed the most-recently recorded
// server-applied knockback. kb is the XZ velocity vector from the last
// SetActorMotion / MotionPredictionHints packet (already consumed from the
// player's pendingKnockback field by the caller before invoking Check).
func (c *VelocityCheck) Check(p *data.Player, kb mgl32.Vec2) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	appliedMag := float32(math.Sqrt(float64(kb[0]*kb[0] + kb[1]*kb[1])))
	if appliedMag < velocityAMinKB {
		// Applied impulse too small to produce a meaningful signal.
		return false, ""
	}

	// Creative players can teleport and have unusual movement; exempt.
	if p.IsCreative() {
		return false, ""
	}
	// Gliding players absorb knockback very differently due to elytra thrust.
	if p.IsGliding() {
		return false, ""
	}
	// Water and crawling both alter knockback physics significantly: water drag
	// exponentially decays horizontal velocity so the ratio-based threshold
	// becomes unreliable; crawling physics are non-standard. Exempt both to
	// avoid false positives (mirrors Oomph's water-state exemption in velocity
	// absorption validation).
	_, _, inWater, crawling, _ := p.InputSnapshotFull()
	if inWater || crawling {
		return false, ""
	}

	// Measure the player's horizontal speed on the current tick.
	// For direction-aware comparison, also project the velocity onto the
	// knockback direction and check both magnitude and projection.
	vel := p.PositionDelta()
	velXZ := mgl32.Vec2{vel[0], vel[2]}
	playerMag := float32(math.Sqrt(float64(velXZ[0]*velXZ[0] + velXZ[1]*velXZ[1])))

	// Compute the dot product of the player's velocity and the knockback
	// direction. If the projection onto the knockback direction is less than
	// velocityAMinRatio × appliedMag, the player has suppressed the knockback.
	kbDir := kb.Normalize()
	projection := velXZ.Dot(kbDir)

	minExpected := appliedMag * velocityAMinRatio
	if projection < minExpected {
		return true, fmt.Sprintf(
			"kb=%.3f player_spd=%.3f projection=%.3f min=%.3f",
			appliedMag, playerMag, projection, minExpected,
		)
	}
	return false, ""
}
