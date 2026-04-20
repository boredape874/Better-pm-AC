package combat

import (
	"strings"
	"testing"
	"time"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// reachFixture returns a player standing at feet=(0, 64, 0). Reach/A adds the
// 1.62-block eye offset internally before measuring, so the target position
// below measures from eye=(0, 65.62, 0).
func reachFixture(t *testing.T) *data.Player {
	t.Helper()
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	return p
}

func newReachCheck(maxReach float64) *ReachCheck {
	return NewReachCheck(config.ReachConfig{
		Enabled:    true,
		Policy:     "kick",
		MaxReach:   maxReach,
		Violations: 10,
	})
}

func TestReachALegalInRangeDoesNotFlag(t *testing.T) {
	p := reachFixture(t)
	c := newReachCheck(3.0)
	// Target 2.5 blocks forward at eye height → dist=2.5 < 3.0.
	target := mgl32.Vec3{0, 65.62, 2.5}
	flagged, info := c.Check(p, target)
	if flagged {
		t.Fatalf("in-range attack flagged: %s", info)
	}
}

func TestReachACheatFlags(t *testing.T) {
	p := reachFixture(t)
	c := newReachCheck(3.0)
	target := mgl32.Vec3{0, 65.62, 6.0}
	flagged, info := c.Check(p, target)
	if !flagged {
		t.Fatal("6-block attack did not flag")
	}
	if !strings.Contains(info, "dist=") || !strings.Contains(info, "max=") {
		t.Fatalf("info missing dist=/max=: %q", info)
	}
}

func TestReachABoundaryAtCap(t *testing.T) {
	p := reachFixture(t)
	c := newReachCheck(3.0)
	// Exactly 3.0 blocks away at eye height → dist == max → pass (strict `>`).
	if flagged, _ := c.Check(p, mgl32.Vec3{0, 65.62, 3.0}); flagged {
		t.Error("dist=3.0 at cap flagged")
	}
	// Just past → flags.
	if flagged, _ := c.Check(p, mgl32.Vec3{0, 65.62, 3.2}); !flagged {
		t.Error("dist=3.2 just over cap did not flag")
	}
}

func TestReachAPingCompensationWidensWindow(t *testing.T) {
	p := reachFixture(t)
	c := newReachCheck(3.0)
	// 3.5 blocks away: without ping comp this flags (3.5 > 3.0).
	if flagged, _ := c.Check(p, mgl32.Vec3{0, 65.62, 3.5}); !flagged {
		t.Fatal("3.5-block with zero ping should flag")
	}
	// 200 ms RTT → one-way 100 ms = 2 ticks × 0.3 = +0.6 block → cap 3.6.
	p.SetLatency(200 * time.Millisecond)
	if flagged, info := c.Check(p, mgl32.Vec3{0, 65.62, 3.5}); flagged {
		t.Fatalf("3.5-block with 200ms ping should pass: %s", info)
	}
}

func TestReachAPolicyContract(t *testing.T) {
	c := newReachCheck(3.0)
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}
