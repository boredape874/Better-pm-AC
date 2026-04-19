package sim

import (
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/go-gl/mathgl/mgl32"
)

// applyJump sets Y velocity to JumpVel when the player jumps from the
// ground. JumpBoost adds JumpBoostStep per level. Sprint-jump adds a small
// horizontal boost (forward-direction) matching Bedrock behaviour.
func applyJump(state meta.SimState, input meta.SimInput, fx effectContext) mgl32.Vec3 {
	if !input.Jumping || !state.OnGround {
		return state.Velocity
	}
	v := state.Velocity
	v[1] = JumpVel + fx.jumpBoost()

	// Sprint-jump: +0.2 × forward direction horizontal boost. The proxy
	// supplies Forward/Strafe already yaw-rotated into world space via
	// applyInput, so we derive the horizontal component from existing
	// velocity direction. For β we use the simpler Oomph approximation:
	// multiply horizontal by 1.2 when sprinting.
	if input.Sprinting && !input.UsingItem {
		v[0] *= 1.2
		v[2] *= 1.2
	}
	return v
}
