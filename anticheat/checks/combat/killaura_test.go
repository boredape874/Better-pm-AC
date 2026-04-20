package combat

import (
	"strings"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// --- KillAura/A: swing-less attacks ---

func newKillAuraCheck() *KillAuraCheck {
	return NewKillAuraCheck(config.KillAuraConfig{
		Enabled:    true,
		Policy:     "kick",
		Violations: 10,
	})
}

func TestKillAuraAFirstAttackGraceDoesNotFlag(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(20)
	// No RecordSwing ever called — lastSwing==0, grace applies.
	c := newKillAuraCheck()
	if flagged, _ := c.Check(p); flagged {
		t.Fatal("first-attack grace failed to prevent flag")
	}
}

func TestKillAuraASwingWithinWindowDoesNotFlag(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(5)
	p.RecordSwing() // lastSwing = 5
	p.UpdateTick(14) // diff = 9, under cap (10)
	c := newKillAuraCheck()
	if flagged, info := c.Check(p); flagged {
		t.Fatalf("swing within window flagged: %s", info)
	}
}

func TestKillAuraASwingLessAttackFlags(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(5)
	p.RecordSwing() // lastSwing = 5
	p.UpdateTick(20) // diff = 15, past 10-tick cap with zero ping
	c := newKillAuraCheck()
	flagged, info := c.Check(p)
	if !flagged {
		t.Fatal("swing-less attack did not flag")
	}
	if !strings.Contains(info, "tick_diff=") || !strings.Contains(info, "max=") {
		t.Fatalf("info missing tick_diff=/max=: %q", info)
	}
}

func TestKillAuraATickWrapSafety(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(1000)
	p.RecordSwing()
	// Simulate reconnect: SimulationFrame resets to lower value.
	p.UpdateTick(5)
	c := newKillAuraCheck()
	if flagged, _ := c.Check(p); flagged {
		t.Fatal("tick-wrap path flagged instead of passing")
	}
}

func TestKillAuraAPolicyContract(t *testing.T) {
	c := newKillAuraCheck()
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}

// --- KillAura/B: attack outside FOV ---

func newKillAuraBCheck() *KillAuraBCheck {
	return NewKillAuraBCheck(config.KillAuraBConfig{
		Enabled:    true,
		Policy:     "kick",
		Violations: 10,
	})
}

// killAuraBFixture places a player at feet (0,64,0) with the supplied yaw.
// Target positions are world-space.
func killAuraBFixture(t *testing.T, yaw float32) *data.Player {
	t.Helper()
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	// UpdateRotation must be called twice to set the absolute rotation cleanly.
	p.UpdateRotation(yaw, 0)
	p.UpdateRotation(yaw, 0)
	return p
}

func TestKillAuraBInLineOfSightDoesNotFlag(t *testing.T) {
	// yaw=0 → look direction is +Z. Target at +Z should be on-axis.
	p := killAuraBFixture(t, 0)
	c := newKillAuraBCheck()
	// Eye at (0, 65.62, 0); target 3 blocks forward.
	target := mgl32.Vec3{0, 65.62, 3}
	if flagged, info := c.Check(p, target); flagged {
		t.Fatalf("on-axis attack flagged: %s", info)
	}
}

func TestKillAuraBBehindTargetFlags(t *testing.T) {
	// yaw=0 → looking +Z. Target behind at -Z is 180° off-axis.
	p := killAuraBFixture(t, 0)
	c := newKillAuraBCheck()
	target := mgl32.Vec3{0, 65.62, -3}
	flagged, info := c.Check(p, target)
	if !flagged {
		t.Fatal("behind-target attack did not flag")
	}
	if !strings.Contains(info, "angle=") || !strings.Contains(info, "max=") {
		t.Fatalf("info missing angle=/max=: %q", info)
	}
}

func TestKillAuraBPolicyContract(t *testing.T) {
	c := newKillAuraBCheck()
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}

// --- KillAura/C: multi-target per tick ---

func newKillAuraCCheck() *KillAuraCCheck {
	return NewKillAuraCCheck(config.KillAuraCConfig{
		Enabled:    true,
		Policy:     "kick",
		Violations: 10,
	})
}

func TestKillAuraCSingleTargetDoesNotFlag(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(10)
	p.RecordAttack(uuid.New())
	c := newKillAuraCCheck()
	if flagged, info := c.Check(p); flagged {
		t.Fatalf("single-target attack flagged: %s", info)
	}
}

func TestKillAuraCMultiTargetSameTickFlags(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(10)
	p.RecordAttack(uuid.New())
	p.RecordAttack(uuid.New()) // same tick → count = 2
	c := newKillAuraCCheck()
	flagged, info := c.Check(p)
	if !flagged {
		t.Fatal("two-attacks-one-tick did not flag")
	}
	if !strings.Contains(info, "targets_per_tick=") {
		t.Fatalf("info missing targets_per_tick=: %q", info)
	}
}

func TestKillAuraCMultipleAttacksAcrossTicksPass(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "tester")
	p.UpdateTick(10)
	p.RecordAttack(uuid.New())
	p.UpdateTick(11)
	p.RecordAttack(uuid.New()) // new tick → count resets to 1
	c := newKillAuraCCheck()
	if flagged, _ := c.Check(p); flagged {
		t.Fatal("two attacks in adjacent ticks flagged")
	}
}

func TestKillAuraCPolicyContract(t *testing.T) {
	c := newKillAuraCCheck()
	if c.Policy() != meta.PolicyKick {
		t.Fatalf("want PolicyKick, got %v", c.Policy())
	}
}
