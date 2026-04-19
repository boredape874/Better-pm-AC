package sim

import (
	"math"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/go-gl/mathgl/mgl32"
)

// applyInput converts Forward/Strafe input into a world-space horizontal
// velocity impulse added to state.Velocity. The yaw rotation is implicit:
// inputs are already in the player-local frame, so we rotate them into world
// space here. Proxy layer sets SimInput.Forward/Strafe from PlayerAuthInput
// and must supply the yaw separately via state.OnGround etc.  Since SimState
// does not carry yaw explicitly, callers pre-rotate by passing world-space
// Forward/Strafe values (a common Oomph simplification).
//
// Multipliers applied in order:
//   - sprint (×1.30) if Sprinting && !UsingItem
//   - sneak (×0.30) if Sneaking && OnGround
//   - swim (×0.20) if Swimming
//   - item-use (×0.20) if UsingItem
//   - speed effect (×(1 + 0.2·level))
//
// The base magnitude is GroundMovement on ground, AirMovement airborne.
func applyInput(state meta.SimState, input meta.SimInput, fx effectContext) mgl32.Vec3 {
	// Stationary input → preserve current velocity, let gravity/friction
	// run unopposed. We still flow through the chain so surface flags stay
	// consistent.
	if input.Forward == 0 && input.Strafe == 0 {
		return state.Velocity
	}

	base := GroundMovement
	if !state.OnGround {
		base = AirMovement
	}

	multiplier := float32(1.0)
	switch {
	case input.Sprinting && !input.UsingItem:
		multiplier *= SprintMultiplier
	case input.Sneaking && state.OnGround:
		multiplier *= SneakMultiplier
	}
	if input.Swimming {
		multiplier *= SwimMultiplier
	}
	if input.UsingItem {
		multiplier *= UseItemMultiplier
	}
	multiplier *= fx.speedMultiplier()

	// Normalize the (Forward, Strafe) pair. Bedrock clamps the magnitude to
	// 1 (diagonal movement doesn't exceed forward speed).
	mag := float32(math.Sqrt(float64(input.Forward*input.Forward + input.Strafe*input.Strafe)))
	if mag > 1.0 {
		input.Forward /= mag
		input.Strafe /= mag
	}

	vx := input.Strafe * base * multiplier
	vz := input.Forward * base * multiplier

	// Accumulate onto existing velocity; horizontal friction in applyFriction
	// provides the decay each tick so velocity does not grow unbounded.
	out := state.Velocity
	out[0] += vx
	out[2] += vz
	return out
}
