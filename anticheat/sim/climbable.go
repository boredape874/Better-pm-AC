package sim

import (
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/go-gl/mathgl/mgl32"
)

// applyClimbable clamps Y velocity to the climb model while the player is
// touching a ladder / vine / scaffolding. Sneak on a non-scaffolding
// climbable freezes vertical motion ("sticky" descent). Jump or upward input
// overrides gravity with ClimbUp.
func applyClimbable(state meta.SimState, input meta.SimInput) mgl32.Vec3 {
	v := state.Velocity
	up := ClimbUp
	if state.OnScaffolding {
		up = ScaffoldClimbUp
	}

	// Downward clamp: gravity would pull us past ClimbDown — cap it.
	if v[1] < -ClimbDown {
		v[1] = -ClimbDown
	}
	if input.Sneaking && !state.OnScaffolding {
		v[1] = 0
	}
	if input.Jumping {
		v[1] = up
	}
	return v
}
