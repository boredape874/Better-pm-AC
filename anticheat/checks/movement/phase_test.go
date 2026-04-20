package movement

import (
	"strings"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// phaseFixture produces a player whose last position delta is the given
// offset from the origin. The first position is the origin so
// posInitialised becomes true on the second update; the third call sets the
// intended delta. Ground state is set to true to match most scenarios.
func phaseFixture(t *testing.T, delta mgl32.Vec3) *data.Player {
	t.Helper()
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(delta.Add(mgl32.Vec3{0, 64, 0}), true)
	return p
}

func newPhaseACheck() *PhaseACheck {
	return NewPhaseACheck(config.PhaseAConfig{Enabled: true, Policy: "kick", Violations: 3})
}

// TestPhaseALegalSprintJumpDoesNotFlag: a sprint-jump produces at most
// ~1 b/tick horizontal + ~0.4 b/tick vertical ≈ 1.1 b/tick 3D; well under 6.
func TestPhaseALegalSprintJumpDoesNotFlag(t *testing.T) {
	p := phaseFixture(t, mgl32.Vec3{0.91, 0.42, 0})
	if flagged, info := newPhaseACheck().Check(p, false); flagged {
		t.Fatalf("legal sprint-jump flagged: %s", info)
	}
}

// TestPhaseATeleportCheatFlags: a 10-block jump in one tick is physically
// impossible without a teleport.
func TestPhaseATeleportCheatFlags(t *testing.T) {
	p := phaseFixture(t, mgl32.Vec3{10, 0, 0})
	flagged, info := newPhaseACheck().Check(p, false)
	if !flagged {
		t.Fatal("10-block jump in one tick did not flag")
	}
	if !strings.Contains(info, "delta=") {
		t.Fatalf("info missing delta field: %q", info)
	}
}

// TestPhaseABoundaryWithinLimit: at exactly the 6-block limit (3-4-5
// triangle = 5 blocks) → pass. 7-block delta → flag.
func TestPhaseABoundaryWithinLimit(t *testing.T) {
	c := newPhaseACheck()
	if flagged, _ := c.Check(phaseFixture(t, mgl32.Vec3{3, 0, 4}), false); flagged {
		t.Fatal("5-block diagonal (within 6.0 cap) flagged")
	}
	if flagged, _ := c.Check(phaseFixture(t, mgl32.Vec3{7, 0, 0}), false); !flagged {
		t.Fatal("7-block delta above 6.0 cap did not flag")
	}
}

// TestPhaseATeleportGraceSkipsCheck: even a 100-block delta must NOT flag
// while teleportGrace is true — the server just moved the player.
func TestPhaseATeleportGraceSkipsCheck(t *testing.T) {
	p := phaseFixture(t, mgl32.Vec3{100, 0, 0})
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
