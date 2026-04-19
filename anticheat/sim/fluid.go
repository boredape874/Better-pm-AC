package sim

import (
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/go-gl/mathgl/mgl32"
)

// applyFluid replaces horizontal / vertical drag with the fluid drag
// constants while the player is submerged. Buoyancy grants a small upward
// impulse when the head is in water (Bedrock's "swim up" behaviour). Lava
// uses the same model with heavier drag.
//
// We treat Water and Lava the same way at β — both trigger InLiquid — and
// differentiate via drag alone. γ can split if lava-specific behaviours
// (fire damage tick, slower swim) become relevant.
func applyFluid(state meta.SimState, input meta.SimInput) mgl32.Vec3 {
	v := state.Velocity
	v[0] *= WaterDrag
	v[2] *= WaterDrag
	v[1] *= WaterDrag

	// Buoyancy / swim-up. Jumping while submerged lifts the player.
	if input.Jumping {
		v[1] += BuoyancyY
	}
	// Sprint-swim adds a small forward impulse along current direction.
	if input.Swimming {
		v[0] *= 1 + WaterSwimBoost
		v[2] *= 1 + WaterSwimBoost
	}
	return v
}
