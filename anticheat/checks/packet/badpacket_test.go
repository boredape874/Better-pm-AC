package packet

import (
	"math"
	"strings"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// --- BadPacket/B: pitch range ---

func TestBadPacketBLegalPitchDoesNotFlag(t *testing.T) {
	c := NewBadPacketBCheck(config.BadPacketBConfig{Enabled: true, Policy: "kick", Violations: 5})
	p := data.NewPlayer(uuid.New(), "tester")
	for _, pitch := range []float32{-90, -45, 0, 45, 90} {
		if flagged, info := c.Check(p, pitch); flagged {
			t.Errorf("pitch=%v legal but flagged: %s", pitch, info)
		}
	}
}

func TestBadPacketBOutOfRangeFlags(t *testing.T) {
	c := NewBadPacketBCheck(config.BadPacketBConfig{Enabled: true, Policy: "kick", Violations: 5})
	p := data.NewPlayer(uuid.New(), "tester")
	for _, pitch := range []float32{-90.01, 90.01, 180, -180, 9999} {
		flagged, info := c.Check(p, pitch)
		if !flagged {
			t.Errorf("pitch=%v out-of-range did not flag", pitch)
		}
		if flagged && !strings.Contains(info, "pitch=") {
			t.Errorf("info missing pitch: %q", info)
		}
	}
}

func TestBadPacketBBoundary(t *testing.T) {
	c := NewBadPacketBCheck(config.BadPacketBConfig{Enabled: true, Policy: "kick", Violations: 5})
	p := data.NewPlayer(uuid.New(), "tester")
	// ±90 is inclusive and must pass.
	if flagged, _ := c.Check(p, 90); flagged {
		t.Error("pitch=90 inclusive boundary flagged")
	}
	if flagged, _ := c.Check(p, -90); flagged {
		t.Error("pitch=-90 inclusive boundary flagged")
	}
	// Just past the bounds flags.
	if flagged, _ := c.Check(p, 90.0001); !flagged {
		t.Error("pitch=90.0001 just over upper bound did not flag")
	}
}

func TestBadPacketBPolicyContract(t *testing.T) {
	c := NewBadPacketBCheck(config.BadPacketBConfig{Enabled: true, Policy: "kick", Violations: 5})
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}

// --- BadPacket/C: sprint+sneak ---

func TestBadPacketCLegalFlags(t *testing.T) {
	c := NewBadPacketCCheck(config.BadPacketCConfig{Enabled: true, Policy: "kick", Violations: 5})
	p := data.NewPlayer(uuid.New(), "tester")

	cases := []struct{ sprint, sneak bool }{
		{false, false},
		{true, false},
		{false, true},
	}
	for _, tc := range cases {
		p.SetInputFlags(tc.sprint, tc.sneak, false, false, false, true)
		if flagged, info := c.Check(p); flagged {
			t.Errorf("legal sprint=%v sneak=%v flagged: %s", tc.sprint, tc.sneak, info)
		}
	}
}

func TestBadPacketCImpossibleFlagsFire(t *testing.T) {
	c := NewBadPacketCCheck(config.BadPacketCConfig{Enabled: true, Policy: "kick", Violations: 5})
	p := data.NewPlayer(uuid.New(), "tester")
	p.SetInputFlags(true, true, false, false, false, true) // sprint+sneak
	flagged, info := c.Check(p)
	if !flagged {
		t.Fatal("sprint+sneak did not flag")
	}
	if info != "sprint+sneak" {
		t.Fatalf("expected info=%q, got %q", "sprint+sneak", info)
	}
}

func TestBadPacketCDisabledSkips(t *testing.T) {
	c := NewBadPacketCCheck(config.BadPacketCConfig{Enabled: false, Policy: "kick", Violations: 5})
	p := data.NewPlayer(uuid.New(), "tester")
	p.SetInputFlags(true, true, false, false, false, true)
	if flagged, _ := c.Check(p); flagged {
		t.Fatal("disabled check still flagged")
	}
}

// --- BadPacket/D: NaN / Inf position ---

func TestBadPacketDLegalPositionDoesNotFlag(t *testing.T) {
	c := NewBadPacketDCheck(config.BadPacketDConfig{Enabled: true, Policy: "kick", Violations: 5})
	cases := []mgl32.Vec3{
		{0, 0, 0},
		{100, 64, -50},
		{1e6, 1e6, 1e6}, // large but finite
	}
	for _, pos := range cases {
		if flagged, info := c.Check(pos); flagged {
			t.Errorf("legal pos %v flagged: %s", pos, info)
		}
	}
}

func TestBadPacketDNaNAndInfFlag(t *testing.T) {
	c := NewBadPacketDCheck(config.BadPacketDConfig{Enabled: true, Policy: "kick", Violations: 5})
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	neg := float32(math.Inf(-1))

	cases := []struct {
		name string
		pos  mgl32.Vec3
		want string
	}{
		{"x=NaN", mgl32.Vec3{nan, 0, 0}, "NaN"},
		{"y=NaN", mgl32.Vec3{0, nan, 0}, "NaN"},
		{"z=NaN", mgl32.Vec3{0, 0, nan}, "NaN"},
		{"x=Inf", mgl32.Vec3{inf, 0, 0}, "Inf"},
		{"y=-Inf", mgl32.Vec3{0, neg, 0}, "Inf"},
	}
	for _, tc := range cases {
		flagged, info := c.Check(tc.pos)
		if !flagged {
			t.Errorf("%s: did not flag", tc.name)
		}
		if !strings.Contains(info, tc.want) {
			t.Errorf("%s: info %q missing %q", tc.name, info, tc.want)
		}
	}
}

func TestBadPacketDPolicyContract(t *testing.T) {
	c := NewBadPacketDCheck(config.BadPacketDConfig{Enabled: true, Policy: "kick", Violations: 5})
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}
