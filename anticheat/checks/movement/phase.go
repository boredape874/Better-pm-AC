package movement

import (
	"fmt"
	"math"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// phaseAMaxDelta is the maximum distance (in blocks) a player may move in a
// single simulation tick without a server-acknowledged teleport.  Any larger
// jump is physically impossible under vanilla Bedrock physics:
//   - Maximum sprint-jump horizontal speed  ≈ 0.7 b/tick × 1.3 ≈ 0.91 b/tick
//   - Maximum free-fall vertical speed      ≈ 3.9 b/tick (terminal velocity)
//   - 3D resultant at worst case            ≈ sqrt(0.91² + 3.9²) ≈ 4.0 b/tick
//
// We set the limit to 6 blocks to absorb generous server lag (≈ 2 extra
// ticks worth of movement) while still catching blatant Phase/Teleport hacks
// that jump 10+ blocks in one packet.
const phaseAMaxDelta = float64(6.0)

// PhaseACheck detects players that displace by more than phaseAMaxDelta blocks
// in a single tick without a server-issued teleport (InputFlagHandledTeleport).
//
// This catches:
//   - Phase hacks that move through walls by teleporting short distances
//     faster than the server can detect.
//   - "TP-aura" bots that teleport to targets before attacking.
//   - Any client that manipulates its reported position discontinuously.
//
// False positives are suppressed by:
//   - Not flagging when TeleportGrace was consumed this tick (proxy sets this
//     when InputFlagHandledTeleport is observed, anticheat consumes it before
//     calling Check).
//   - Using a conservative 6-block limit (10×+ beyond sprint-jump).
//   - Requiring posInitialised (first tick is always skipped).
//
// Implements anticheat.Detection.
type PhaseACheck struct {
	cfg config.PhaseAConfig
}

func NewPhaseACheck(cfg config.PhaseAConfig) *PhaseACheck { return &PhaseACheck{cfg: cfg} }

func (*PhaseACheck) Type() string    { return "Phase" }
func (*PhaseACheck) SubType() string { return "A" }
func (*PhaseACheck) Description() string {
	return "Detects impossible position jumps in a single tick without a teleport."
}
func (*PhaseACheck) Punishable() bool { return true }

func (c *PhaseACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Even one confirmed impossible jump is a clear violation.
		FailBuffer:    1,
		MaxBuffer:     2,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates the magnitude of the current position delta. teleportGrace is
// passed in from anticheat.go: when true, the player has just been teleported by
// the server and the large delta is expected; skip the check.
func (c *PhaseACheck) Check(p *data.Player, teleportGrace bool) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	// Creative players can fly at any speed; some servers also issue creative
	// teleports. Exempt entirely.
	if p.IsCreative() {
		return false, ""
	}
	// Server-sent teleport this tick — the large delta is expected.
	if teleportGrace {
		return false, ""
	}

	vel := p.PositionDelta()
	dist := math.Sqrt(float64(vel[0]*vel[0]+vel[1]*vel[1]+vel[2]*vel[2]))

	if dist > phaseAMaxDelta {
		return true, fmt.Sprintf("delta=%.2f max=%.1f", dist, phaseAMaxDelta)
	}
	return false, ""
}
