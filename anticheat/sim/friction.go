package sim

import (
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/go-gl/mathgl/mgl32"
)

// applyFriction multiplies horizontal velocity by the per-tick friction
// factor. On ground the factor is blockFriction × BaseFriction; airborne it
// is just BaseFriction. Surface flags on state pick the block-specific
// value.
func applyFriction(state meta.SimState) mgl32.Vec3 {
	v := state.Velocity
	factor := BaseFriction
	if state.OnGround {
		factor = BaseFriction * groundFriction(state)
	}
	v[0] *= factor
	v[2] *= factor
	return v
}

// groundFriction picks the block-specific friction. Surfaces are ranked by
// specificity: more slippery surfaces (ice) take precedence over less
// slippery ones when multiple flags are set.
func groundFriction(state meta.SimState) float32 {
	switch {
	case state.InCobweb:
		return CobwebFriction
	case state.OnIce:
		return IceFriction
	case state.OnSlime:
		return SlimeFriction
	case state.OnHoney:
		return HoneyFriction
	case state.OnSoulSand:
		return SoulSandFriction
	default:
		return DefaultFriction
	}
}
