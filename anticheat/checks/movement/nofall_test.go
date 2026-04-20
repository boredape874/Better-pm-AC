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

// fallAndLand constructs a Player that has fallen `dropBlocks` blocks and is
// now on the ground. The drop is split into 0.25-block increments so that
// FallDistance tracks smoothly through the final airborne tick — only
// airborne frames update FallDistance, so coarser steps round the reported
// distance down to the last pre-landing airborne Y.
func fallAndLand(t *testing.T, dropBlocks float32) *data.Player {
	t.Helper()
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdatePosition(mgl32.Vec3{0, 100, 0}, false)
	const step = 0.25
	for y := float32(100 - step); y > 100-dropBlocks+step/2; y -= step {
		p.UpdatePosition(mgl32.Vec3{0, y, 0}, false)
	}
	p.UpdatePosition(mgl32.Vec3{0, 100 - dropBlocks, 0}, true)
	p.SetInputFlags(false, false, false, false, false, true)
	return p
}

func newNoFallCheck() *NoFallCheck {
	return NewNoFallCheck(config.NoFallConfig{Enabled: true, Policy: "kick", Violations: 5})
}

// TestNoFallALegalShortFallDoesNotFlag: a 2-block fall is below the
// damage threshold — vanilla inflicts no damage and the check must stay quiet.
func TestNoFallALegalShortFallDoesNotFlag(t *testing.T) {
	p := fallAndLand(t, 2)
	c := newNoFallCheck()
	if flagged, info := c.Check(p); flagged {
		t.Fatalf("2-block fall flagged: %s", info)
	}
}

// TestNoFallACheatFlags: a 10-block fall that arrives with fallDistance > 3
// and no damage-absorbing exemption must flag.
func TestNoFallACheatFlags(t *testing.T) {
	p := fallAndLand(t, 10)
	c := newNoFallCheck()
	flagged, info := c.Check(p)
	if !flagged {
		t.Fatal("10-block fall did not flag")
	}
	if !strings.Contains(info, "fall_dist=") {
		t.Fatalf("info missing fall_dist: %q", info)
	}
}

// TestNoFallABoundaryExactlyAtThreshold: fall_dist == 3 is the cutoff; the
// check uses `<=` so exactly 3 must NOT flag. 3.01 must flag.
func TestNoFallABoundaryExactlyAtThreshold(t *testing.T) {
	c := newNoFallCheck()
	if flagged, _ := c.Check(fallAndLand(t, 3)); flagged {
		t.Fatal("3-block fall (exactly at threshold) flagged; expected pass")
	}
	if flagged, _ := c.Check(fallAndLand(t, 3.5)); !flagged {
		t.Fatal("3.5-block fall did not flag")
	}
}

// TestNoFallAPolicyContract: default mapping is kick per the built-in config.
func TestNoFallAPolicyContract(t *testing.T) {
	cases := []struct {
		in   string
		want meta.MitigatePolicy
	}{
		{"kick", meta.PolicyKick},
		{"none", meta.PolicyNone},
		{"", meta.PolicyKick},
	}
	for _, tc := range cases {
		c := NewNoFallCheck(config.NoFallConfig{Enabled: true, Policy: tc.in, Violations: 5})
		if got := c.Policy(); got != tc.want {
			t.Errorf("Policy(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
