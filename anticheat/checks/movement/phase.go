package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// phaseASnapThreshold is the number of reconciler snaps within the rolling
// window that triggers a Phase/A flag. A snap means the reconciler had to
// correct the client's claimed position because it was outside the tolerance
// band — more than 3 snaps in 20 ticks signals that the client is consistently
// teleporting to bad positions (phase/teleport cheat pattern).
const phaseASnapThreshold = 3

// PhaseACheck detects players that repeatedly cause the reconciler to snap
// their position, indicating phase or teleport cheats. Under the committed-pos
// path, phase cheats show up as repeated OutcomeSnap verdicts rather than a
// large raw delta because CommittedPos is already reconciler-corrected.
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
func (c *PhaseACheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *PhaseACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Even one confirmed impossible jump is a clear violation.
		FailBuffer:    1,
		MaxBuffer:     2,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check uses the reconciler snap rate as the phase-detection signal.
// Under committed-pos mode, CommittedPos is already reconciler-corrected, so a
// phase cheat shows up as repeated OutcomeSnap corrections within the rolling
// window rather than a large raw position delta. More than phaseASnapThreshold
// snaps in 20 ticks is the threshold (see data.Player.RecordSnap / SnapSnapshot).
func (c *PhaseACheck) Check(p *data.Player, teleportGrace bool) (bool, string) {
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
