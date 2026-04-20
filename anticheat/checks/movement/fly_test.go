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

// flyFixture returns a player who has been airborne for `airTicks` with the
// given per-tick Y delta, established by replaying UpdatePosition the right
// number of times. The first tick is a ground baseline; from tick 2 onward
// the player is in the air. Legal free-fall would have yDelta ≈ -0.08 at
// tick 2 and progressively more negative, but for Fly/A's hover/upward
// signals a constant yDelta is exactly the signal we want to fabricate.
func flyFixture(t *testing.T, airTicks int, yDelta float32) *data.Player {
	t.Helper()
	p := data.NewPlayer(uuid.New(), "tester")
	// Ground baseline (twice so LastOnGround is also true).
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)

	// Airborne ticks: start from Y=64 and decrement by yDelta each tick
	// (decrement because Bedrock Y-down means falling is negative yDelta
	// arithmetic: newY = prevY + yDelta).
	y := float32(64)
	for i := 0; i < airTicks; i++ {
		y += yDelta
		p.UpdatePosition(mgl32.Vec3{0, y, 0}, false)
	}
	// No terrain contact, not sneaking/crawling/swimming — just plain air.
	p.SetInputFlags(false, false, false, false, false, false)
	return p
}

func newFlyCheck() *FlyCheck {
	return NewFlyCheck(config.FlyConfig{Enabled: true, Policy: "kick", Violations: 5})
}

// TestFlyALegalJumpArcDoesNotFlag: 6 airborne ticks (within the 8-tick grace)
// with progressive falling Y delta must not flag.
func TestFlyALegalJumpArcDoesNotFlag(t *testing.T) {
	// Construct an arc manually rather than via flyFixture so yDelta varies.
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.SetInputFlags(false, false, false, false, false, false)
	// Jump profile: +0.42 (jump), then gravity*airDrag decay.
	yVel := float32(0.42)
	y := float32(64)
	for i := 0; i < 6; i++ {
		y += yVel
		p.UpdatePosition(mgl32.Vec3{0, y, 0}, false)
		yVel = (yVel - 0.08) * 0.98
	}

	c := newFlyCheck()
	flagged, info := c.Check(p)
	if flagged {
		t.Fatalf("legal jump arc flagged: %s", info)
	}
}

// TestFlyAHoverFlags: 20 airborne ticks with yDelta ≈ 0 clearly exceeds the
// grace and hover-ticks thresholds — must flag with a "hover" info string.
func TestFlyAHoverFlags(t *testing.T) {
	p := flyFixture(t, 20, 0.001) // sub-threshold Y delta → HoverTicks accrue
	c := newFlyCheck()

	flagged, info := c.Check(p)
	if !flagged {
		t.Fatal("20-tick hover at yDelta≈0 did not flag")
	}
	if !strings.Contains(info, "hover") {
		t.Fatalf("expected 'hover' branch in info, got %q", info)
	}
}

// TestFlyAUpwardFlyFlags: the grace is 8 ticks; after that a sustained
// POSITIVE yDelta is impossible in vanilla and must flag the upward-fly
// branch.
func TestFlyAUpwardFlyFlags(t *testing.T) {
	p := flyFixture(t, 12, 0.10) // still rising at tick 12 — no vanilla arc does this
	c := newFlyCheck()

	flagged, info := c.Check(p)
	if !flagged {
		t.Fatal("sustained upward airborne motion past grace did not flag")
	}
	if !strings.Contains(info, "upward_fly") {
		t.Fatalf("expected 'upward_fly' branch in info, got %q", info)
	}
}

// TestFlyACreativeExempt: creative flight is legal; must not flag regardless
// of airborne state.
func TestFlyACreativeExempt(t *testing.T) {
	p := flyFixture(t, 30, 0.0) // worst-case hover
	p.SetGameMode(1)            // Creative
	c := newFlyCheck()

	if flagged, info := c.Check(p); flagged {
		t.Fatalf("creative player flagged: %s", info)
	}
}

// TestFlyAPolicyContract: default config maps to kick, and the full enum is
// round-trippable.
func TestFlyAPolicyContract(t *testing.T) {
	cases := []struct {
		in   string
		want meta.MitigatePolicy
	}{
		{"kick", meta.PolicyKick},
		{"server_filter", meta.PolicyServerFilter},
		{"client_rubberband", meta.PolicyClientRubberband},
		{"none", meta.PolicyNone},
		{"", meta.PolicyKick},
	}
	for _, tc := range cases {
		c := NewFlyCheck(config.FlyConfig{Enabled: true, Policy: tc.in, Violations: 5})
		if got := c.Policy(); got != tc.want {
			t.Errorf("Policy(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
