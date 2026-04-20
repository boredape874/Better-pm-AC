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

// speedFixture wires a Player into the state Speed/A expects to see on a
// ground-movement tick: two position updates so Velocity is non-zero and
// equal to dx blocks/tick horizontally, ground contact, and the requested
// sprint/sneak/crawl/usingItem input flags. The returned player has already
// consumed the teleport grace for its first position so Speed/A's early-outs
// (IsCreative / knockback / airborne / just-landed) are all false unless the
// caller sets them.
func speedFixture(t *testing.T, dx float32, sprinting, sneaking, crawling, usingItem bool) *data.Player {
	t.Helper()
	p := data.NewPlayer(uuid.New(), "tester")

	// First tick: establish baseline position and on-ground state. Do it
	// twice so LastOnGround is also true (IsJustLanded only fires on the
	// ground→air transition tick).
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)

	// Second tick: horizontal displacement dx in +X. Velocity = delta = dx.
	p.UpdatePosition(mgl32.Vec3{dx, 64, 0}, true)

	// Input flags — terrainCollision=true because Speed/A expects a grounded
	// player to be in contact with terrain; false here would only matter for
	// Fly/A-style checks.
	p.SetInputFlags(sprinting, sneaking, false, crawling, usingItem, true)
	return p
}

func newSpeedCheck(maxSpeed float64) *SpeedCheck {
	return NewSpeedCheck(config.SpeedConfig{
		Enabled:    true,
		Policy:     "kick",
		MaxSpeed:   maxSpeed,
		Violations: 10,
	})
}

// TestSpeedALegalSprintDoesNotFlag: a sprinting player moving at exactly the
// sprint-adjusted limit must not flag. MaxSpeed=0.20, sprint multiplier 1.30 →
// effective cap 0.26 b/tick. A 0.25 b/tick step is inside the cap.
func TestSpeedALegalSprintDoesNotFlag(t *testing.T) {
	p := speedFixture(t, 0.25, true, false, false, false)
	c := newSpeedCheck(0.20)

	flagged, info := c.Check(p)
	if flagged {
		t.Fatalf("legal sprint flagged: %s", info)
	}
}

// TestSpeedACheatFlags: a walking player hitting 0.50 b/tick (well above the
// 0.20 base limit, and above the 0.26 sprint cap) must flag. The info string
// should contain the measured speed so log correlation works.
func TestSpeedACheatFlags(t *testing.T) {
	p := speedFixture(t, 0.50, false, false, false, false)
	c := newSpeedCheck(0.20)

	flagged, info := c.Check(p)
	if !flagged {
		t.Fatal("speed-hack delta 0.50 b/tick did not flag under 0.20 b/tick walking cap")
	}
	if !strings.Contains(info, "speed=") || !strings.Contains(info, "max=") {
		t.Fatalf("info string missing speed/max fields: %q", info)
	}
}

// TestSpeedABoundarySneakExactlyAtLimit: the sneak multiplier (0.30) combined
// with base 0.20 gives an effective cap of 0.06 b/tick. A delta of exactly
// 0.06 must not flag (> is strict), and a delta just above (0.07) must flag.
func TestSpeedABoundarySneakExactlyAtLimit(t *testing.T) {
	c := newSpeedCheck(0.20)

	// Floating-point comparison: the measured delta must be strictly <= limit.
	// Use 0.059 to sit just under the 0.06 cap accounting for mgl32 precision.
	pLegal := speedFixture(t, 0.059, false, true, false, false)
	if flagged, info := c.Check(pLegal); flagged {
		t.Fatalf("sneaking just under cap flagged: %s", info)
	}

	pCheat := speedFixture(t, 0.08, false, true, false, false)
	if flagged, _ := c.Check(pCheat); !flagged {
		t.Fatal("sneaking above cap (0.08 > 0.06) did not flag")
	}
}

// TestSpeedAPolicyContract: the Policy() return is determined by the config
// string. Missing/unknown values must fall back to PolicyKick (the design's
// default) so an operator who forgets to set policy still gets enforcement.
func TestSpeedAPolicyContract(t *testing.T) {
	cases := []struct {
		in   string
		want meta.MitigatePolicy
	}{
		{"kick", meta.PolicyKick},
		{"none", meta.PolicyNone},
		{"client_rubberband", meta.PolicyClientRubberband},
		{"server_filter", meta.PolicyServerFilter},
		{"", meta.PolicyKick},        // default
		{"gibberish", meta.PolicyKick}, // invalid → default
	}
	for _, tc := range cases {
		c := NewSpeedCheck(config.SpeedConfig{Enabled: true, Policy: tc.in, MaxSpeed: 0.2, Violations: 10})
		if got := c.Policy(); got != tc.want {
			t.Errorf("Policy(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
