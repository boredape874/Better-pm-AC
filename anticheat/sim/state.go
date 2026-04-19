package sim

import (
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/go-gl/mathgl/mgl32"
)

// NewState returns a zero SimState positioned at pos, standing on ground.
// Useful as the initial seed for a player session's simulation chain.
func NewState(pos mgl32.Vec3) meta.SimState {
	return meta.SimState{Position: pos, OnGround: true}
}

// emptyInput is a handy zero-input for tests and idle ticks.
func emptyInput() meta.SimInput { return meta.SimInput{} }

// effectAmp returns the amplifier for effect id in input.Effects, or 0 if
// absent. The Bedrock protocol passes amplifier+1 (level 1 → value 1), so the
// caller uses the returned number directly as a "level" count.
func effectAmp(input meta.SimInput, id int32) int32 {
	if input.Effects == nil {
		return 0
	}
	return input.Effects[id]
}
