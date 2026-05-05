package movement

import (
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// TestScenario1_SpeedMicro checks that a player moving at 0.42 b/tick
// horizontally on the ground is flagged by Speed/A (default MaxSpeed 0.4).
func TestScenario1_SpeedMicro(t *testing.T) {
	// prev pos: 0,64,0 → cur pos: 0.42,64,0 → horizontal delta = 0.42 b/tick
	cur := mgl32.Vec3{0.42, 64, 0}
	// Create fresh player with two ground ticks then a fast tick.
	p2 := data.NewPlayer(uuid.New(), "speed-micro")
	p2.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p2.Commit(mgl32.Vec3{0, 64, 0})
	p2.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)
	p2.Commit(mgl32.Vec3{0, 64, 0})
	p2.UpdatePosition(cur, true)
	p2.Commit(cur)

	c := NewSpeedCheck(config.SpeedConfig{
		Enabled:    true,
		Policy:     "kick",
		MaxSpeed:   0.40,
		Violations: 10,
	})
	flagged, info := c.Check(p2)
	if !flagged {
		t.Fatalf("Speed/A should flag 0.42 b/tick with max=0.40; info=%q", info)
	}
}

// TestScenario2_FlyHover checks that a player hovering in mid-air (deltaY ≈ 0
// while off-ground past grace ticks, no jump) is flagged by Fly/A.
func TestScenario2_FlyHover(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "fly-hover")
	// Put player in the air for more than flyGraceTicks + flyMinHoverTicks ticks
	// at a near-zero Y delta so HoverTicks accumulates.
	ground := mgl32.Vec3{0, 64, 0}
	airPos := mgl32.Vec3{0, 65, 0}
	p.UpdatePosition(ground, true)
	p.Commit(ground)
	// Lift off.
	p.UpdatePosition(airPos, false)
	p.Commit(airPos)
	// Simulate flyGraceTicks + flyMinHoverTicks more ticks at constant height.
	hover := airPos
	for i := 0; i < flyGraceTicks+flyMinHoverTicks+1; i++ {
		p.UpdatePosition(hover, false)
		p.Commit(hover)
	}

	c := NewFlyCheck(config.FlyConfig{
		Enabled:    true,
		Policy:     "kick",
		Violations: 5,
	})
	flagged, info := c.Check(p)
	if !flagged {
		t.Fatalf("Fly/A should flag hover after grace period; info=%q", info)
	}
}

// TestScenario3_NoFallLavaImmune checks that a player landing after a fall
// greater than 3 blocks is flagged by NoFall/A.
func TestScenario3_NoFallLavaImmune(t *testing.T) {
	// Simulate a proper fall arc with FallDistance tracking.
	p2 := data.NewPlayer(uuid.New(), "nofall-lava")
	start := mgl32.Vec3{0, 70, 0}
	p2.UpdatePosition(start, false) // posInitialised
	p2.Commit(start)
	// Fall ticks (descending, airborne).
	for i := 1; i <= 5; i++ {
		falling := mgl32.Vec3{0, 70 - float32(i), 0}
		p2.UpdatePosition(falling, false)
		p2.Commit(falling)
	}
	// Landing tick: transition airborne → ground. FallDistance ≥ 5 blocks.
	land := mgl32.Vec3{0, 64, 0}
	prevCommitted := p2.CommittedPos()
	p2.UpdatePosition(land, true)
	// LastFallDistance is now set by UpdatePosition tracking (5 blocks).
	// Also commit so committedFallDist = prevCommitted[1] - land[1].
	p2.Commit(land)
	_ = prevCommitted

	c := NewNoFallCheck(config.NoFallConfig{
		Enabled:    true,
		Policy:     "kick",
		Violations: 5,
	})
	flagged, info := c.Check(p2)
	if !flagged {
		t.Fatalf("NoFall/A should flag 5+ block fall without damage; info=%q", info)
	}
}

// TestScenario4_PhaseSnapRate checks that a player with a high snap rate
// (> phaseASnapThreshold) is flagged by Phase/A.
func TestScenario4_PhaseSnapRate(t *testing.T) {
	p := data.NewPlayer(uuid.New(), "phase-snap")
	// Simulate more than phaseASnapThreshold snaps in the rolling window.
	for i := 0; i < phaseASnapThreshold+2; i++ {
		p.RecordSnap()
	}

	c := NewPhaseACheck(config.PhaseAConfig{
		Enabled:    true,
		Policy:     "kick",
		Violations: 3,
	})
	flagged, info := c.Check(p, false /* no teleportGrace */)
	if !flagged {
		t.Fatalf("Phase/A should flag snap_rate=%d > threshold=%d; info=%q",
			phaseASnapThreshold+2, phaseASnapThreshold, info)
	}
}

// TestScenario5_NoSlowEatSprint checks that a player sprinting at near-walking
// speed while using an item is flagged by NoSlow/A.
func TestScenario5_NoSlowEatSprint(t *testing.T) {
	// Sprint speed with MaxItemUseSpeed=0.21: 0.42 b/tick should flag.
	prev := mgl32.Vec3{0, 64, 0}
	cur := mgl32.Vec3{0.42, 64, 0}

	p := data.NewPlayer(uuid.New(), "noslow-eatSprint")
	p.UpdatePosition(prev, true)
	p.Commit(prev)
	p.UpdatePosition(prev, true)
	p.Commit(prev)
	p.UpdatePosition(cur, true)
	p.Commit(cur)
	// Mark player as using item and sprinting.
	p.SetInputFlags(true /*sprinting*/, false, false, false, true /*usingItem*/, true)

	c := NewNoSlowCheck(config.NoSlowConfig{
		Enabled:         true,
		Policy:          "kick",
		MaxItemUseSpeed: 0.21,
		Violations:      8,
	})
	flagged, info := c.Check(p)
	if !flagged {
		t.Fatalf("NoSlow/A should flag sprint-speed (0.42) while using item (max=0.21); info=%q", info)
	}
}
