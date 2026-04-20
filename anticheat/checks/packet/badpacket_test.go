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
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// fakeBitset implements interface{ Load(int) bool } for BadPacket/E tests.
type fakeBitset map[int]bool

func (f fakeBitset) Load(bit int) bool { return f[bit] }

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

// --- BadPacket/A: tick transitions ---

func newBadPacketACheck() *BadPacketCheck {
	return NewBadPacketCheck(config.BadPacketConfig{Enabled: true, Policy: "kick", Violations: 1})
}

func TestBadPacketAFirstPacketGraceDoesNotFlag(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	// prev == 0 (never UpdateTick'd) — all branches are gated on prev != 0.
	c := newBadPacketACheck()
	if flagged, _ := c.Check(p, 0); flagged {
		t.Fatal("first-packet tick=0 flagged")
	}
	if flagged, _ := c.Check(p, 100); flagged {
		t.Fatal("first-packet tick=100 flagged")
	}
}

func TestBadPacketAMonotonicDoesNotFlag(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(50)
	c := newBadPacketACheck()
	if flagged, _ := c.Check(p, 51); flagged {
		t.Fatal("monotonic tick=prev+1 flagged")
	}
	if flagged, _ := c.Check(p, 200); flagged {
		t.Fatal("large-but-under-jump tick=prev+150 flagged")
	}
}

func TestBadPacketATickResetFlags(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(50)
	c := newBadPacketACheck()
	flagged, info := c.Check(p, 0)
	if !flagged {
		t.Fatal("tick reset did not flag")
	}
	if info != "tick_reset" {
		t.Fatalf("want info=tick_reset, got %q", info)
	}
}

func TestBadPacketATickRegressionFlags(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(50)
	c := newBadPacketACheck()
	flagged, info := c.Check(p, 20)
	if !flagged {
		t.Fatal("tick regression did not flag")
	}
	if !strings.Contains(info, "tick_regression") {
		t.Fatalf("info missing tick_regression: %q", info)
	}
}

func TestBadPacketATickJumpFlags(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(100)
	c := newBadPacketACheck()
	flagged, info := c.Check(p, 400) // diff = 300 > 200
	if !flagged {
		t.Fatal("tick jump did not flag")
	}
	if !strings.Contains(info, "tick_jump") {
		t.Fatalf("info missing tick_jump: %q", info)
	}
}

func TestBadPacketAPolicyContract(t *testing.T) {
	c := newBadPacketACheck()
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}

// --- BadPacket/E: contradictory flag pairs ---

func newBadPacketECheck() *BadPacketECheck {
	return NewBadPacketECheck(config.BadPacketEConfig{Enabled: true, Policy: "kick", Violations: 1})
}

func TestBadPacketELegalFlagsDoesNotFlag(t *testing.T) {
	// Only StartSprinting set — no contradiction.
	bits := fakeBitset{packet.InputFlagStartSprinting: true}
	c := newBadPacketECheck()
	if flagged, info := c.Check(bits); flagged {
		t.Fatalf("legal flags flagged: %s", info)
	}
}

func TestBadPacketESprintPairFlags(t *testing.T) {
	bits := fakeBitset{
		packet.InputFlagStartSprinting: true,
		packet.InputFlagStopSprinting:  true,
	}
	c := newBadPacketECheck()
	flagged, info := c.Check(bits)
	if !flagged {
		t.Fatal("sprint start+stop did not flag")
	}
	if !strings.Contains(info, "start+stop_sprint") {
		t.Fatalf("info missing start+stop_sprint: %q", info)
	}
}

func TestBadPacketEMultiplePairsInOneInfo(t *testing.T) {
	bits := fakeBitset{
		packet.InputFlagStartSprinting: true,
		packet.InputFlagStopSprinting:  true,
		packet.InputFlagStartSneaking:  true,
		packet.InputFlagStopSneaking:   true,
	}
	c := newBadPacketECheck()
	flagged, info := c.Check(bits)
	if !flagged {
		t.Fatal("double contradiction did not flag")
	}
	if !strings.Contains(info, "start+stop_sprint") || !strings.Contains(info, "start+stop_sneak") {
		t.Fatalf("info missing one of the pairs: %q", info)
	}
	if !strings.Contains(info, ",") {
		t.Fatalf("info missing comma separator: %q", info)
	}
}

func TestBadPacketEDisabledSkips(t *testing.T) {
	bits := fakeBitset{
		packet.InputFlagStartSprinting: true,
		packet.InputFlagStopSprinting:  true,
	}
	c := NewBadPacketECheck(config.BadPacketEConfig{Enabled: false, Policy: "kick", Violations: 1})
	if flagged, _ := c.Check(bits); flagged {
		t.Fatal("disabled check still flagged")
	}
}

func TestBadPacketEPolicyContract(t *testing.T) {
	c := newBadPacketECheck()
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}
