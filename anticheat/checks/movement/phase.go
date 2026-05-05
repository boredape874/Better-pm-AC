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
// phaseASnapThreshold is the number of reconciler snaps within the rolling
// window that triggers a Phase/A flag under MovementAuth. A snap means the
// reconciler had to correct the client's claimed position because it was
// outside the tolerance band — more than 3 snaps in 20 ticks signals that
// the client is consistently teleporting to bad positions.
const phaseASnapThreshold = 3

type PhaseACheck struct {
	cfg       config.PhaseAConfig
	authority *config.AuthorityConfig
}

func NewPhaseACheck(cfg config.PhaseAConfig) *PhaseACheck { return &PhaseACheck{cfg: cfg} }

// SetAuthority wires the shared AuthorityConfig.
func (c *PhaseACheck) SetAuthority(a *config.AuthorityConfig) { c.authority = a }

func (*PhaseACheck) Type() string    { return "Phase" }
func (*PhaseACheck) SubType() string { return "A" }
func (*PhaseACheck) Description() string {
	return "Detects impossible position jumps in a single tick without a teleport."
}
func (*PhaseACheck) Punishable() bool { return true }
func (c *PhaseACheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *PhaseACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Even one confirmed impossible jump is a clear violation.
		FailBuffer:    1,
		MaxBuffer:     2,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// CheckLegacy is the original position-delta (3D distance) implementation
// (γ.3.4 migration: retained as fallback when MovementAuth is disabled).
func (c *PhaseACheck) CheckLegacy(p *data.Player, teleportGrace bool) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if p.IsCreative() {
		return false, ""
	}
	if p.IsGliding() {
		return false, ""
	}
	if teleportGrace {
		return false, ""
	}
	vel := p.PositionDelta()
	dist := math.Sqrt(float64(vel[0]*vel[0] + vel[1]*vel[1] + vel[2]*vel[2]))
	if dist > phaseAMaxDelta {
		return true, fmt.Sprintf("delta=%.2f max=%.1f", dist, phaseAMaxDelta)
	}
	return false, ""
}

// Check evaluates the magnitude of the current position delta. teleportGrace is
// passed in from anticheat.go: when true, the player has just been teleported by
// the server and the large delta is expected; skip the check.
// Under MovementAuth the check switches to snap-rate analysis: because
// CommittedPos is already corrected by reconcile, a phase cheat shows up as
// repeated OutcomeSnap verdicts rather than a large raw delta.
func (c *PhaseACheck) Check(p *data.Player, teleportGrace bool) (bool, string) {
	if c.authority != nil && c.authority.MovementAuth {
		return c.checkCommitted(p, teleportGrace)
	}
	return c.CheckLegacy(p, teleportGrace)
}

// checkCommitted uses the reconciler snap rate as the phase-detection signal.
// Under MovementAuth, CommittedPos is already reconciler-corrected, so a phase
// cheat shows up as repeated OutcomeSnap corrections within the rolling window
// rather than a large raw position delta. More than phaseASnapThreshold snaps
// in 20 ticks is the threshold (see data.Player.RecordSnap / SnapSnapshot).
func (c *PhaseACheck) checkCommitted(p *data.Player, teleportGrace bool) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if p.IsCreative() {
		return false, ""
	}
	if p.IsGliding() {
		return false, ""
	}
	if teleportGrace {
		return false, ""
	}
	snapCount, windowSize := p.SnapSnapshot()
	if snapCount > phaseASnapThreshold {
		return true, fmt.Sprintf("snap_rate=%d/%d_ticks threshold=%d", snapCount, windowSize, phaseASnapThreshold)
	}
	return false, ""
}
