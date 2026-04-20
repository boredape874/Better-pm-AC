package combat

import (
	"strings"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/google/uuid"
)

func newAutoClickerCheck(maxCPS int) *AutoClickerCheck {
	return NewAutoClickerCheck(config.AutoClickerConfig{
		Enabled:    true,
		Policy:     "kick",
		MaxCPS:     maxCPS,
		Violations: 10,
	})
}

// clickFixture records n clicks in quick succession; all timestamps land within
// the 1-second CPS window so CPS() returns n exactly.
func clickFixture(t *testing.T, n int) *data.Player {
	t.Helper()
	p := data.NewPlayer(uuid.New(), "tester")
	for i := 0; i < n; i++ {
		p.RecordClick()
	}
	return p
}

func TestAutoClickerALegalCPSDoesNotFlag(t *testing.T) {
	p := clickFixture(t, 10)
	c := newAutoClickerCheck(16)
	if flagged, info := c.Check(p); flagged {
		t.Fatalf("10 CPS legal but flagged: %s", info)
	}
}

func TestAutoClickerACheatFlags(t *testing.T) {
	p := clickFixture(t, 25)
	c := newAutoClickerCheck(16)
	flagged, info := c.Check(p)
	if !flagged {
		t.Fatal("25 CPS did not flag")
	}
	if !strings.Contains(info, "cps=") || !strings.Contains(info, "max=") {
		t.Fatalf("info missing cps=/max=: %q", info)
	}
}

func TestAutoClickerABoundary(t *testing.T) {
	c := newAutoClickerCheck(16)
	// CPS==MaxCPS → pass (`> max` is strict).
	p16 := clickFixture(t, 16)
	if flagged, _ := c.Check(p16); flagged {
		t.Error("CPS=16 at cap flagged")
	}
	p17 := clickFixture(t, 17)
	if flagged, _ := c.Check(p17); !flagged {
		t.Error("CPS=17 just over cap did not flag")
	}
}

func TestAutoClickerADisabledSkips(t *testing.T) {
	p := clickFixture(t, 100)
	c := NewAutoClickerCheck(config.AutoClickerConfig{
		Enabled: false, Policy: "kick", MaxCPS: 16, Violations: 10,
	})
	if flagged, _ := c.Check(p); flagged {
		t.Fatal("disabled check still flagged")
	}
}

func TestAutoClickerAPolicyContract(t *testing.T) {
	c := newAutoClickerCheck(16)
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}
