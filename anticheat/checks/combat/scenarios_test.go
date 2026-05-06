package combat

import (
	"testing"

	ac_combat "github.com/boredape874/Better-pm-AC/anticheat/combat"
	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// TestScenario1_Reach6Blocks checks that a player attacking a target 6.5 blocks
// away is flagged by Reach/A when MaxReach is 3.0. The snapshots path is used:
// a single bbox is placed at distance 6.5 blocks (ray T ≈ 5.9 > maxReach=3.0).
func TestScenario1_Reach6Blocks(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "reach-cheater")
	// Player standing at feet (0, 64, 0), eye at (0, 65.62, 0).
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	// Look direction: yaw=0, pitch=0 → +Z axis.
	p.UpdateRotation(0, 0)
	p.UpdateRotation(0, 0)

	// Bbox for target at (0, 64, 6.5): distance from eye (0,65.62,0) along +Z.
	// The ray from eye along +Z will enter the bbox at roughly Z = 6.5 - 0.3 = 6.2.
	snapshots := []ac_combat.BBox{
		{Pos: mgl32.Vec3{0, 64, 6.5}, HalfWidth: 0.3, Height: 1.8},
	}

	c := NewReachCheck(config.ReachConfig{
		Enabled:    true,
		Policy:     "kick",
		MaxReach:   3.0,
		Violations: 10,
	})

	// Provide target foot pos and snapshots.
	targetPos := mgl32.Vec3{0, 64, 6.5}
	flagged, info := c.Check(p, targetPos, snapshots...)
	if !flagged {
		t.Fatalf("Reach/A should flag 6.5-block attack (max=3.0); info=%q", info)
	}
}

// TestScenario2_KillAuraNoSwing checks that a player whose ray hits an entity
// bbox but who has not registered a swing in the [T-1, T+1] window is flagged
// by KillAura/A.
func TestScenario2_KillAuraNoSwing(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "killaura-noswing")
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	// Look direction +Z.
	p.UpdateRotation(0, 0)
	p.UpdateRotation(0, 0)
	// Set simulation tick to 50.
	p.UpdateTick(50)
	// Record a swing at tick 5 — far outside [T-1, T+1] = [49, 51].
	p.UpdateTick(5)
	p.RecordSwing()
	p.UpdateTick(50)

	// Target bbox directly in line of sight, 2 blocks ahead.
	snapshots := []ac_combat.BBox{
		{Pos: mgl32.Vec3{0, 64, 2}, HalfWidth: 0.3, Height: 1.8},
	}

	c := NewKillAuraCheck(config.KillAuraConfig{
		Enabled:    true,
		Policy:     "kick",
		Violations: 10,
	})

	flagged, info := c.Check(p, snapshots...)
	if !flagged {
		t.Fatalf("KillAura/A should flag swing-less raycast hit; info=%q", info)
	}
	_ = info
}

// TestScenario3_KillAuraBehindBack checks that a player attacking a target
// directly behind them (>90° off look axis) is flagged by KillAura/B.
func TestScenario3_KillAuraBehindBack(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "killaura-behind")
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	// yaw=0 → looking +Z. Target at -Z is 180° behind.
	p.UpdateRotation(0, 0)
	p.UpdateRotation(0, 0)

	// Target 3 blocks directly behind the player (−Z direction).
	targetPos := mgl32.Vec3{0, 65.62, -3}

	c := NewKillAuraBCheck(config.KillAuraBConfig{
		Enabled:    true,
		Policy:     "kick",
		Violations: 10,
	})

	flagged, info := c.Check(p, targetPos)
	if !flagged {
		t.Fatalf("KillAura/B should flag behind-back attack (180°>90°); info=%q", info)
	}
	_ = info
}
