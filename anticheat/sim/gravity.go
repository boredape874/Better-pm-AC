package sim

import (
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/go-gl/mathgl/mgl32"
)

// applyVertical applies gravity and Y drag. Levitation replaces gravity;
// SlowFalling caps downward velocity instead of multiplying.
func applyVertical(state meta.SimState, input meta.SimInput, fx effectContext) mgl32.Vec3 {
	v := state.Velocity
	// Levitation wins over gravity — replace, don't stack.
	if fx.Levitation > 0 {
		target := LevitationStep * float32(fx.Levitation)
		// Ease toward target upward velocity (Bedrock uses (target - v) * 0.2
		// per tick; we match that to smooth hover onset).
		v[1] += (target - v[1]) * 0.2
		return v
	}

	if fx.SlowFalling && v[1] < 0 {
		// Clamp descent speed to SlowFallCap; drag still applies afterward.
		if v[1] < -SlowFallCap {
			v[1] = -SlowFallCap
		}
	} else {
		v[1] -= Gravity
	}
	v[1] *= AirDragY
	return v
}
