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

// velocityFixture returns a player with the given horizontal delta and the
// required grounded posture. Velocity/A runs AFTER UpdatePosition, so the
// Player's Velocity field must equal the observed delta.
func velocityFixture(t *testing.T, delta mgl32.Vec3) *data.Player {
	t.Helper()
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(delta.Add(mgl32.Vec3{0, 64, 0}), true)
	p.SetInputFlags(false, false, false, false, false, true)
	return p
}

func newVelocityCheck() *VelocityCheck {
	return NewVelocityCheck(config.VelocityConfig{Enabled: true, Policy: "kick", Violations: 5})
}

// TestVelocityALegalAbsorptionDoesNotFlag: a 0.5 b/tick knockback in +X
// with the player subsequently moving at 0.4 b/tick in +X (80% of applied)
// is well above the 15% minimum ratio → no flag.
func TestVelocityALegalAbsorptionDoesNotFlag(t *testing.T) {
	p := velocityFixture(t, mgl32.Vec3{0.4, 0, 0})
	kb := mgl32.Vec2{0.5, 0}
	if flagged, info := newVelocityCheck().Check(p, kb); flagged {
		t.Fatalf("80%% absorption flagged: %s", info)
	}
}

// TestVelocityACheatFlags: a 0.5 b/tick knockback in +X but the player
// stays put (0 velocity) means the Anti-KB cheat fully suppressed the
// impulse. Projection onto kb direction = 0, which is < 15% * 0.5 = 0.075.
func TestVelocityACheatFlags(t *testing.T) {
	p := velocityFixture(t, mgl32.Vec3{0, 0, 0})
	kb := mgl32.Vec2{0.5, 0}
	flagged, info := newVelocityCheck().Check(p, kb)
	if !flagged {
		t.Fatal("fully absorbed knockback did not flag")
	}
	if !strings.Contains(info, "projection=") || !strings.Contains(info, "min=") {
		t.Fatalf("info missing projection/min: %q", info)
	}
}

// TestVelocityABelowMinKBSkips: a 0.05 b/tick applied impulse is below the
// velocityAMinKB (0.1) threshold — too small to produce a reliable signal.
// The check should not flag regardless of player velocity.
func TestVelocityABelowMinKBSkips(t *testing.T) {
	p := velocityFixture(t, mgl32.Vec3{0, 0, 0})
	kb := mgl32.Vec2{0.05, 0}
	if flagged, _ := newVelocityCheck().Check(p, kb); flagged {
		t.Fatal("sub-threshold knockback should not flag")
	}
}

// TestVelocityABoundaryAtMinRatio: projection exactly at 15% × 0.5 = 0.075
// must NOT flag (`projection < minExpected` is strict). 0.07 must flag.
func TestVelocityABoundaryAtMinRatio(t *testing.T) {
	c := newVelocityCheck()
	kb := mgl32.Vec2{0.5, 0}
	if flagged, info := c.Check(velocityFixture(t, mgl32.Vec3{0.075, 0, 0}), kb); flagged {
		t.Fatalf("projection exactly at min ratio flagged: %s", info)
	}
	if flagged, _ := c.Check(velocityFixture(t, mgl32.Vec3{0.07, 0, 0}), kb); !flagged {
		t.Fatal("projection below min ratio did not flag")
	}
}

func TestVelocityAPolicyContract(t *testing.T) {
	c := NewVelocityCheck(config.VelocityConfig{Enabled: true, Policy: "kick", Violations: 5})
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}
