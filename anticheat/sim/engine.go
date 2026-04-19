package sim

import (
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
)

// Engine replays Bedrock player physics tick by tick. It is pure:
// Step(prev, input, world) returns the next state without mutating prev.
//
// Order of operations per tick mirrors Bedrock client source as documented
// by Oomph and the Minecraft wiki:
//
//  1. Apply potion effects (multiplicative modifiers stored for later use)
//  2. Apply horizontal input (walk / sprint / sneak / swim / item-use)
//  3. Apply jump impulse (sets yVel if Jumping && OnGround)
//  4. Apply vertical forces (gravity, air drag, slow-fall/levitation)
//  5. Apply fluid effects (buoyancy, water drag) if InLiquid
//  6. Apply climbable effects if OnClimbable
//  7. Sweep collision against blocks (Y first, then X, then Z)
//  8. Apply horizontal friction (surface + air)
//  9. Update surface flags by querying blocks at new position
type Engine struct{}

// NewEngine returns a default engine. The engine is stateless; one instance
// can be shared across all players.
func NewEngine() *Engine { return &Engine{} }

// Step computes the next state from prev given input and a world view.
func (e *Engine) Step(prev meta.SimState, input meta.SimInput, world meta.WorldTracker) meta.SimState {
	next := prev
	// 1. potion effects parameterise downstream steps — resolve them once.
	fx := resolveEffects(input)

	// 2. horizontal input
	next.Velocity = applyInput(next, input, fx)

	// 3. jump impulse
	next.Velocity = applyJump(next, input, fx)

	// 4. vertical (gravity, drag, slow-fall, levitation)
	next.Velocity = applyVertical(next, input, fx)

	// 5. fluid effects
	if next.InLiquid {
		next.Velocity = applyFluid(next, input)
	}

	// 6. climbable
	if next.OnClimbable {
		next.Velocity = applyClimbable(next, input)
	}

	// 7. collision sweep
	next = sweepCollision(next, world)

	// 8. friction (horizontal) — ground uses block friction, airborne uses
	//    the base 0.91 constant.
	next.Velocity = applyFriction(next)

	// 9. refresh surface flags based on new position
	refreshSurfaces(&next, world)

	return next
}

// compile-time contract check
var _ meta.SimEngine = (*Engine)(nil)
