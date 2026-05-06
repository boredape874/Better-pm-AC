package sim

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

func TestStepGravityFromAir(t *testing.T) {
	in := StepInput{
		PrevPos:  mgl32.Vec3{0, 64, 0},
		Velocity: mgl32.Vec3{0, 0, 0},
		OnGround: false,
	}
	out := Step(in)
	if out.Velocity.Y() >= 0 {
		t.Fatalf("Step from air should apply gravity, got vy=%f", out.Velocity.Y())
	}
}

func TestStepIsPure(t *testing.T) {
	in := StepInput{
		PrevPos:  mgl32.Vec3{0, 64, 0},
		Velocity: mgl32.Vec3{0.1, 0, 0},
	}
	a := Step(in)
	b := Step(in)
	if a != b {
		t.Fatalf("Step is not pure: %+v vs %+v", a, b)
	}
}

// TestLavaDragLowerTerminalVelocity verifies that falling in lava converges to
// a lower terminal Y velocity than falling in water, because LavaDrag (0.5) <
// WaterDrag (0.8).
func TestLavaDragLowerTerminalVelocity(t *testing.T) {
	base := StepInput{
		PrevPos:  mgl32.Vec3{0, 80, 0},
		Velocity: mgl32.Vec3{0, 0, 0},
		OnGround: false,
	}

	// Run 20 ticks in water.
	waterIn := base
	waterIn.Fluid = FluidWater
	for i := 0; i < 20; i++ {
		out := Step(waterIn)
		waterIn.Velocity = out.Velocity
		waterIn.PrevPos = out.ExpectedPos
	}
	waterTerminalY := waterIn.Velocity[1]

	// Run 20 ticks in lava.
	lavaIn := base
	lavaIn.Fluid = FluidLava
	for i := 0; i < 20; i++ {
		out := Step(lavaIn)
		lavaIn.Velocity = out.Velocity
		lavaIn.PrevPos = out.ExpectedPos
	}
	lavaTerminalY := lavaIn.Velocity[1]

	// Both are negative (falling). LavaDrag (0.5) < WaterDrag (0.8) so lava
	// applies more deceleration per tick: terminal = -gravity/(1-drag).
	// lava terminal ≈ -0.16, water terminal ≈ -0.40.
	// Lava terminal velocity is less negative (lower absolute speed).
	if lavaTerminalY <= waterTerminalY {
		t.Fatalf("lava terminal vy=%.4f should be less negative (lower speed) than water terminal vy=%.4f",
			lavaTerminalY, waterTerminalY)
	}
}

// TestBubbleColumnUpAddsVelocity checks that BubbleUp adds +BubbleColumnUpForce
// per tick.
func TestBubbleColumnUpAddsVelocity(t *testing.T) {
	in := StepInput{
		PrevPos:   mgl32.Vec3{0, 64, 0},
		Velocity:  mgl32.Vec3{0, 0, 0},
		OnGround:  false,
		Fluid:     FluidWater,
		BubbleUp:  true,
	}
	out := Step(in)
	// After one tick with BubbleUp: gravity lowers Y then drag, then +0.04 boost.
	// Net effect: Y vel should be higher than without bubble.
	inNoBubble := in
	inNoBubble.BubbleUp = false
	outNoBubble := Step(inNoBubble)
	if out.Velocity[1] <= outNoBubble.Velocity[1] {
		t.Fatalf("BubbleUp should increase Y velocity: bubble=%.4f no-bubble=%.4f",
			out.Velocity[1], outNoBubble.Velocity[1])
	}
}

// TestBubbleColumnDownSubtractsVelocity checks that BubbleDown subtracts
// BubbleColumnDownForce per tick.
func TestBubbleColumnDownSubtractsVelocity(t *testing.T) {
	in := StepInput{
		PrevPos:    mgl32.Vec3{0, 64, 0},
		Velocity:   mgl32.Vec3{0, 0, 0},
		OnGround:   false,
		Fluid:      FluidWater,
		BubbleDown: true,
	}
	out := Step(in)
	inNoBubble := in
	inNoBubble.BubbleDown = false
	outNoBubble := Step(inNoBubble)
	if out.Velocity[1] >= outNoBubble.Velocity[1] {
		t.Fatalf("BubbleDown should decrease Y velocity: bubble=%.4f no-bubble=%.4f",
			out.Velocity[1], outNoBubble.Velocity[1])
	}
}
