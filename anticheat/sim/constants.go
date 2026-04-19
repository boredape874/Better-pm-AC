package sim

// Physics constants cross-referenced with docs/physics-constants.md.
// All values are per-tick at 20 TPS. Changing any of these must update the
// companion reference doc.

const (
	// Vertical
	Gravity        float32 = 0.08
	AirDragY       float32 = 0.98
	JumpVel        float32 = 0.42
	JumpBoostStep  float32 = 0.10
	SlowFallCap    float32 = 0.01
	LevitationStep float32 = 0.05

	// Horizontal
	GroundMovement    float32 = 0.10
	AirMovement       float32 = 0.02
	SprintMultiplier  float32 = 1.30
	SneakMultiplier   float32 = 0.30
	UseItemMultiplier float32 = 0.20
	SwimMultiplier    float32 = 0.20
	SpeedEffectStep   float32 = 0.20

	// Friction (surface factor; airborne uses baseline 0.91)
	DefaultFriction  float32 = 0.60
	IceFriction      float32 = 0.98
	SlimeFriction    float32 = 0.80
	SoulSandFriction float32 = 0.40
	HoneyFriction    float32 = 0.40
	CobwebFriction   float32 = 0.05
	BaseFriction     float32 = 0.91

	// Collision
	StepHeight     float32 = 0.60
	BBoxWidth      float32 = 0.60
	BBoxHeight     float32 = 1.80
	SneakBBoxHei   float32 = 1.50
	SwimBBoxHeight float32 = 0.60

	// Fluid
	WaterDrag      float32 = 0.80
	LavaDrag       float32 = 0.50
	WaterSwimBoost float32 = 0.04
	BuoyancyY      float32 = 0.04

	// Climbable
	ClimbUp         float32 = 0.20
	ClimbDown       float32 = 0.15
	ScaffoldClimbUp float32 = 0.50
)

// Effect IDs for the SimInput.Effects map. Values match Bedrock protocol.
const (
	EffectSpeed       int32 = 1
	EffectJumpBoost   int32 = 8
	EffectSlowFalling int32 = 27
	EffectLevitation  int32 = 24
	EffectSlowness    int32 = 2
)
