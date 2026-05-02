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
