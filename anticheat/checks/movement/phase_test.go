package movement

import (
	"strings"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/google/uuid"
)

// snapFixture produces a player with the given number of reconciler snaps
// recorded (simulating the Phase/A committed-pos signal path).
func snapFixture(n int) *data.Player {
	p := data.NewPlayer(uuid.New(), "tester")
	for i := 0; i < n; i++ {
		p.RecordSnap()
	}
	return p
}

func newPhaseACheck() *PhaseACheck {
	return NewPhaseACheck(config.PhaseAConfig{Enabled: true, Policy: "kick", Violations: 3})
}

// TestPhaseALegalSnapRateDoesNotFlag: zero snaps should not flag.
func TestPhaseALegalSnapRateDoesNotFlag(t *testing.T) {
	p := snapFixture(0)
	if flagged, info := newPhaseACheck().Check(p, false); flagged {
		t.Fatalf("zero snaps flagged: %s", info)
	}
}

// TestPhaseATeleportCheatFlags: more than phaseASnapThreshold snaps flags.
func TestPhaseATeleportCheatFlags(t *testing.T) {
	p := snapFixture(phaseASnapThreshold + 2)
	flagged, info := newPhaseACheck().Check(p, false)
	if !flagged {
		t.Fatal("10-block jump in one tick did not flag")
	}
	if !strings.Contains(info, "snap_rate=") {
		t.Fatalf("info missing snap_rate field: %q", info)
	}
}

// TestPhaseABoundaryWithinLimit: exactly at threshold does not flag; one over does.
func TestPhaseABoundaryWithinLimit(t *testing.T) {
	c := newPhaseACheck()
	// Exactly at threshold (not strictly greater) → no flag.
	if flagged, _ := c.Check(snapFixture(phaseASnapThreshold), false); flagged {
		t.Fatal("snap count at threshold should not flag (check is strictly greater)")
	}
	// One above threshold → flag.
	if flagged, _ := c.Check(snapFixture(phaseASnapThreshold+1), false); !flagged {
		t.Fatal("snap count above threshold did not flag")
	}
}

// TestPhaseATeleportGraceSkipsCheck: teleport grace must suppress Phase/A.
func TestPhaseATeleportGraceSkipsCheck(t *testing.T) {
	p := snapFixture(phaseASnapThreshold + 5)
	if flagged, _ := newPhaseACheck().Check(p, true); flagged {
		t.Fatal("teleport-grace should suppress Phase/A")
	}
}

func TestPhaseAPolicyContract(t *testing.T) {
	c := NewPhaseACheck(config.PhaseAConfig{Enabled: true, Policy: "kick", Violations: 3})
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}
