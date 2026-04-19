package sim

import "github.com/boredape874/Better-pm-AC/anticheat/meta"

// effectContext folds the subset of potion effects the physics steps care
// about into scalar multipliers and offsets. Resolving once per tick keeps
// the hot path branch-free.
type effectContext struct {
	SpeedLevel     int32
	JumpBoostLevel int32
	SlowFalling    bool
	Levitation     int32
}

func resolveEffects(input meta.SimInput) effectContext {
	return effectContext{
		SpeedLevel:     effectAmp(input, EffectSpeed),
		JumpBoostLevel: effectAmp(input, EffectJumpBoost),
		SlowFalling:    effectAmp(input, EffectSlowFalling) > 0,
		Levitation:     effectAmp(input, EffectLevitation),
	}
}

// speedMultiplier returns the horizontal multiplier from Speed effect.
// level 0 → 1.0 (no effect); each level adds SpeedEffectStep.
func (f effectContext) speedMultiplier() float32 {
	if f.SpeedLevel <= 0 {
		return 1.0
	}
	return 1.0 + SpeedEffectStep*float32(f.SpeedLevel)
}

// jumpBoost returns the extra y-velocity granted by JumpBoost levels.
func (f effectContext) jumpBoost() float32 {
	if f.JumpBoostLevel <= 0 {
		return 0
	}
	return JumpBoostStep * float32(f.JumpBoostLevel)
}
