package sim

import (
	"math"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/go-gl/mathgl/mgl32"
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

// Effects is the public alias for the internal effectContext. Exporting it
// lets external callers (e.g. property-based tests in adjacent packages)
// build a StepInput without having to bridge through meta.SimInput. It is a
// thin POD struct — see effects.go for the field semantics.
type Effects = effectContext

// StepInput is the immutable input to the deterministic single-tick simulator.
// All physics state needed for one tick must arrive in this struct — Step()
// must not read any package-level globals. This is what makes property-based
// testing of the sim viable.
type StepInput struct {
	PrevPos      mgl32.Vec3
	Velocity     mgl32.Vec3
	OnGround     bool
	Effects      Effects // existing struct from effects.go
	InputForward float32
	InputStrafe  float32
	Sprint       bool
	Sneak        bool
	Jump         bool
	InLiquid     bool // γ.5 — currently ignored by Step()
	OnClimbable  bool // γ.5 — currently ignored by Step()
}

// StepOutput is the result of one simulation tick.
type StepOutput struct {
	ExpectedPos mgl32.Vec3
	Velocity    mgl32.Vec3
}

// Step advances the simulation by exactly one 50 ms tick using the rules
// already encoded in this package (gravity, friction, jump, walk, fluid).
// It is pure: same input → same output, no shared state.
//
// γ.1 stub: assembles existing helpers without world / collision context.
// γ.5 will add small-Δ collision and lava-specific drag here. The current
// shape is sufficient for shadow-mode plumbing.
//
// γ.1 stub does NOT yet handle fluid (water/lava drag) or climbable surfaces
// (ladders, vines). Callers feeding swimming or laddered players will see
// disagreement vs. the legacy Engine.Step. γ.5 will add applyFluid /
// applyClimbable here, at which point StepInput.InLiquid and
// StepInput.OnClimbable will become live inputs.
//
// WARNING: callers must NOT set Sprint=true (or pass Jump while Sprint=true)
// when the player is using an item. The legacy sim disables sprint-speed and
// the sprint-jump 1.2× boost during item use; γ.1 walkVelOnly / jumpVelOnly do
// NOT replicate that guard because UsingItem is not yet on StepInput. γ.5
// plumbs it.
func Step(in StepInput) StepOutput {
	v := in.Velocity
	if !in.OnGround {
		v = gravityVelOnly(v, in.Effects)
	}
	if in.Jump && in.OnGround {
		v = jumpVelOnly(v, in.Effects, in.Sprint)
	}
	v = walkVelOnly(v, in.InputForward, in.InputStrafe, in.Sprint, in.Sneak, in.OnGround, in.Effects)
	v = frictionVelOnly(v, in.OnGround)
	pos := in.PrevPos.Add(v)
	return StepOutput{ExpectedPos: pos, Velocity: v}
}

// gravityVelOnly is the velocity-only inner core of applyVertical: applies
// gravity and Y drag without consulting any external state. Levitation /
// SlowFalling come from Effects.
func gravityVelOnly(v mgl32.Vec3, fx Effects) mgl32.Vec3 {
	if fx.Levitation > 0 {
		target := LevitationStep * float32(fx.Levitation)
		v[1] += (target - v[1]) * 0.2
		return v
	}
	if fx.SlowFalling && v[1] < 0 {
		if v[1] < -SlowFallCap {
			v[1] = -SlowFallCap
		}
	} else {
		v[1] -= Gravity
	}
	v[1] *= AirDragY
	return v
}

// jumpVelOnly is the velocity-only inner core of applyJump. Caller is
// responsible for the OnGround && Jump precondition; this helper unconditionally
// applies the jump impulse plus optional sprint-jump horizontal boost.
//
// Sprint-guard divergence: legacy applyInput / applyJump disable the sprint
// multiplier (and the sprint-jump 1.2× boost) when UsingItem is set. This
// inner core does NOT — UsingItem will be plumbed in γ.5. Until then, callers
// must clear Sprint for item-using players.
func jumpVelOnly(v mgl32.Vec3, fx Effects, sprint bool) mgl32.Vec3 {
	v[1] = JumpVel + fx.jumpBoost()
	if sprint {
		v[0] *= 1.2
		v[2] *= 1.2
	}
	return v
}

// walkVelOnly is the velocity-only inner core of applyInput. Strafe maps to
// world X, forward to world Z (callers pre-rotate by yaw — same convention as
// applyInput). UsingItem / Swimming are not yet plumbed through StepInput;
// γ.5 will extend the field set as needed.
//
// Sprint-guard divergence: legacy applyInput / applyJump disable the sprint
// multiplier (and the sprint-jump 1.2× boost) when UsingItem is set. This
// inner core does NOT — UsingItem will be plumbed in γ.5. Until then, callers
// must clear Sprint for item-using players.
func walkVelOnly(v mgl32.Vec3, forward, strafe float32, sprint, sneak, onGround bool, fx Effects) mgl32.Vec3 {
	if forward == 0 && strafe == 0 {
		return v
	}
	base := GroundMovement
	if !onGround {
		base = AirMovement
	}
	multiplier := float32(1.0)
	switch {
	case sprint:
		multiplier *= SprintMultiplier
	case sneak && onGround:
		multiplier *= SneakMultiplier
	}
	multiplier *= fx.speedMultiplier()

	// Clamp diagonal magnitude to 1 — match applyInput.
	mag := float32(math.Sqrt(float64(forward*forward + strafe*strafe)))
	if mag > 1.0 {
		forward /= mag
		strafe /= mag
	}
	v[0] += strafe * base * multiplier
	v[2] += forward * base * multiplier
	return v
}

// frictionVelOnly is the velocity-only inner core of applyFriction with no
// surface flags wired in (γ.5 will add them). Ground friction collapses to
// DefaultFriction × BaseFriction; airborne is just BaseFriction.
func frictionVelOnly(v mgl32.Vec3, onGround bool) mgl32.Vec3 {
	factor := BaseFriction
	if onGround {
		factor = BaseFriction * DefaultFriction
	}
	v[0] *= factor
	v[2] *= factor
	return v
}
