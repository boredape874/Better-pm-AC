// Interface contracts shared across anti-cheat subsystems.
//
// Implementations live in their own packages:
//
//	WorldTracker       → anticheat/world
//	SimEngine          → anticheat/sim
//	EntityRewind       → anticheat/entity
//	AckSystem          → anticheat/ack
//	MitigateDispatcher → anticheat/mitigate
//
// These contracts are the boundary between AI owners and must not change
// without a Proposal. See docs/plans/2026-04-19-anticheat-overhaul-design.md §11.
package meta

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// WorldTracker is the server-authoritative block view fed by LevelChunk and
// SubChunk packets. Checks query it for block state and collision BBoxes at a
// position. Implemented by anticheat/world.
type WorldTracker interface {
	HandleLevelChunk(pk *packet.LevelChunk) error
	HandleSubChunk(pk *packet.SubChunk) error
	HandleBlockUpdate(pos cube.Pos, rid uint32) error
	Block(pos cube.Pos) world.Block
	BlockBBoxes(pos cube.Pos) []cube.BBox
	ChunkLoaded(x, z int32) bool
	Close() error
}

// SimInput is one tick of player input fed into SimEngine.Step. Effects maps
// Bedrock effect IDs to amplifier (e.g. speed → +1).
type SimInput struct {
	Forward, Strafe float32
	Jumping         bool
	Sprinting       bool
	Sneaking        bool
	Swimming        bool
	UsingItem       bool
	GlideStart      bool
	GlideStop       bool
	Riptiding       bool
	Effects         map[int32]int32
}

// SimState is the full physics state the engine advances tick-by-tick.
// Position/Velocity are world-space; all booleans describe the block or fluid
// the player is currently interacting with.
type SimState struct {
	Position      mgl32.Vec3
	Velocity      mgl32.Vec3
	OnGround      bool
	InLiquid      bool
	InCobweb      bool
	InPowderSnow  bool
	OnClimbable   bool
	OnSlime       bool
	OnIce         bool
	OnHoney       bool
	OnSoulSand    bool
	OnScaffolding bool
	Gliding       bool
	Riptiding     bool
}

// SimEngine replays Bedrock player physics server-side. Step is pure: given a
// previous state, one tick of input, and a world view, it returns the next
// state. Implemented by anticheat/sim.
type SimEngine interface {
	Step(prev SimState, input SimInput, world WorldTracker) SimState
}

// EntitySnapshot is one tick of an entity's pose, used for reach rewind.
type EntitySnapshot struct {
	Tick     uint64
	Position mgl32.Vec3
	BBox     cube.BBox
	Rotation mgl32.Vec2
}

// EntityRewind stores a ring buffer of entity snapshots so combat checks can
// validate a swing against where the target was at the attacker's tick, not
// where it is now. Implemented by anticheat/entity.
type EntityRewind interface {
	Record(rid uint64, tick uint64, pos mgl32.Vec3, bbox cube.BBox, rot mgl32.Vec2)
	At(rid uint64, tick uint64) (EntitySnapshot, bool)
	Purge(rid uint64)
}

// AckCallback fires when the client confirms a NetworkStackLatency round-trip
// for the tick the callback was registered on.
type AckCallback func(tick uint64)

// AckSystem issues NetworkStackLatency events and resolves their callbacks
// when the client echoes the timestamp back. Implemented by anticheat/ack.
type AckSystem interface {
	Dispatch(cb AckCallback) packet.Packet
	OnResponse(timestamp int64, tick uint64)
	PendingCount() int
}

// MitigatePolicy is the enforcement action chosen per Detection.
//
//	PolicyNone              - log only, forward unchanged
//	PolicyClientRubberband  - send position teleport to snap client back
//	PolicyServerFilter      - drop or rewrite the offending packet
//	PolicyKick              - disconnect when MaxViolations is reached
type MitigatePolicy int

const (
	PolicyNone MitigatePolicy = iota
	PolicyClientRubberband
	PolicyServerFilter
	PolicyKick
)

// MitigateDispatcher turns a Detection flag into the configured enforcement
// action and returns the (possibly rewritten) packet to forward plus whether
// the session should be terminated. Implemented by anticheat/mitigate.
type MitigateDispatcher interface {
	Apply(playerUUID string, d Detection, meta *DetectionMetadata,
		original packet.Packet) (forwarded packet.Packet, kick bool)
}
