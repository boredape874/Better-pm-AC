package combat

import (
	"strings"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/google/uuid"
)

func newAimCheck() *AimCheck {
	return NewAimCheck(config.AimConfig{
		Enabled:    true,
		Policy:     "kick",
		Violations: 10,
	})
}

// aimFixture builds a mouse-input player whose last-tick yaw delta is yawDelta.
func aimFixture(t *testing.T, yawDelta float32, mouse bool) *data.Player {
	t.Helper()
	p := data.NewPlayer(uuid.New(), "tester")
	if mouse {
		p.SetInputMode(1) // packet.InputModeMouse
	} else {
		p.SetInputMode(2) // touch
	}
	// Two UpdateRotation calls: establishes a baseline then produces the
	// requested delta on the second call.
	p.UpdateRotation(0, 0)
	p.UpdateRotation(yawDelta, 0)
	return p
}

func TestAimANonMouseSkips(t *testing.T) {
	p := aimFixture(t, 2.0, false)
	c := newAimCheck()
	flagged, _, _ := c.Check(p)
	if flagged {
		t.Fatal("non-mouse client flagged")
	}
}

func TestAimARoundYawDeltaFlags(t *testing.T) {
	// Exactly 2.0° delta — rounds identically at 1 and 5 decimals → flag.
	p := aimFixture(t, 2.0, true)
	c := newAimCheck()
	flagged, info, _ := c.Check(p)
	if !flagged {
		t.Fatal("round yaw delta did not flag")
	}
	if !strings.Contains(info, "yaw_delta=") || !strings.Contains(info, "diff=") {
		t.Fatalf("info missing yaw_delta=/diff=: %q", info)
	}
}

func TestAimANaturalYawDeltaDoesNotFlag(t *testing.T) {
	// 2.34567° — r1=2.3, r2=2.34567, diff=0.04567 >> 3e-5 → pass.
	p := aimFixture(t, 2.34567, true)
	c := newAimCheck()
	flagged, info, _ := c.Check(p)
	if flagged {
		t.Fatalf("natural yaw delta flagged: %s", info)
	}
}

func TestAimAIdleYawSkips(t *testing.T) {
	// 0.0005° delta is below the 1e-3 idle gate → pass.
	p := aimFixture(t, 0.0005, true)
	c := newAimCheck()
	flagged, _, _ := c.Check(p)
	if flagged {
		t.Fatal("idle yaw flagged")
	}
}

func TestAimAPolicyContract(t *testing.T) {
	c := newAimCheck()
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
	_ = data.NewPlayer // keep import if refactored later
}
