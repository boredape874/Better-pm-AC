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

// itemUseFixture returns a player with usingItem=true and the given
// horizontal displacement per tick.
func itemUseFixture(t *testing.T, dx float32) *data.Player {
	t.Helper()
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(mgl32.Vec3{dx, 64, 0}, true)
	// Flags: not sprinting, not sneaking, not in water, not crawling, USING ITEM.
	p.SetInputFlags(false, false, false, false, true, true)
	return p
}

func newNoSlowCheck() *NoSlowCheck {
	return NewNoSlowCheck(config.NoSlowConfig{
		Enabled:         true,
		Policy:          "kick",
		MaxItemUseSpeed: 0.21,
		Violations:      8,
	})
}

// TestNoSlowALegalItemUseDoesNotFlag: 0.18 b/tick while using an item is
// below the 0.21 cap → no flag.
func TestNoSlowALegalItemUseDoesNotFlag(t *testing.T) {
	p := itemUseFixture(t, 0.18)
	if flagged, info := newNoSlowCheck().Check(p); flagged {
		t.Fatalf("legal item-use speed flagged: %s", info)
	}
}

// TestNoSlowACheatFlags: 0.50 b/tick while using an item is way over the
// 0.21 cap and is the signature of a NoSlow cheat (full sprint while
// eating/bowing/shielding).
func TestNoSlowACheatFlags(t *testing.T) {
	p := itemUseFixture(t, 0.50)
	flagged, info := newNoSlowCheck().Check(p)
	if !flagged {
		t.Fatal("0.50 b/tick during item use did not flag")
	}
	if !strings.Contains(info, "speed=") || !strings.Contains(info, "max=") {
		t.Fatalf("info missing speed/max: %q", info)
	}
}

// TestNoSlowABoundaryJustUnder: 0.205 b/tick just under the 0.21 cap → pass.
// 0.215 just over → flag.
func TestNoSlowABoundaryJustUnder(t *testing.T) {
	c := newNoSlowCheck()
	if flagged, _ := c.Check(itemUseFixture(t, 0.205)); flagged {
		t.Fatal("0.205 b/tick under cap flagged")
	}
	if flagged, _ := c.Check(itemUseFixture(t, 0.215)); !flagged {
		t.Fatal("0.215 b/tick over cap did not flag")
	}
}

// TestNoSlowANotUsingItemSkips: without the usingItem flag, speed is
// irrelevant to NoSlow/A — even a 5 b/tick delta must not flag here (that
// would be caught by Speed/A or Phase/A, not this check).
func TestNoSlowANotUsingItemSkips(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p.UpdatePosition(mgl32.Vec3{5, 64, 0}, true)
	p.SetInputFlags(true, false, false, false, false, true) // sprinting, NOT using item

	if flagged, _ := newNoSlowCheck().Check(p); flagged {
		t.Fatal("NoSlow/A flagged a player who was not using an item")
	}
}

func TestNoSlowAPolicyContract(t *testing.T) {
	c := NewNoSlowCheck(config.NoSlowConfig{Enabled: true, Policy: "kick", MaxItemUseSpeed: 0.21, Violations: 8})
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}
