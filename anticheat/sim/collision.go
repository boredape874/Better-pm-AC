package sim

import (
	"math"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
)

// sweepCollision advances state.Position by state.Velocity, clamping each
// axis against block BBoxes along the way. Y is swept first (so landing
// clears vertical velocity before horizontal collision runs), then X, then
// Z. When horizontal motion is blocked on ground, an auto-step up to
// StepHeight is attempted.
//
// The sweep uses Dragonfly's cube.BBox.{X,Y,Z}Offset semantics: given an
// entity box and a candidate delta, the block box trims the delta to the
// maximum value that does not penetrate. This matches the Bedrock client.
func sweepCollision(state meta.SimState, world meta.WorldTracker) meta.SimState {
	out := state
	if out.Velocity[0] == 0 && out.Velocity[1] == 0 && out.Velocity[2] == 0 {
		return out
	}

	entityBox := playerBBox(out.Position, false)
	// Gather block boxes in a slightly-grown region around the entity's
	// swept AABB so we don't miss any along the path.
	growX := float32(math.Abs(float64(out.Velocity[0]))) + 1
	growY := float32(math.Abs(float64(out.Velocity[1]))) + 1
	growZ := float32(math.Abs(float64(out.Velocity[2]))) + 1
	nearby := blockBBoxesAround(world, entityBox, growX, growY, growZ)

	dY := out.Velocity[1]
	dX := out.Velocity[0]
	dZ := out.Velocity[2]

	// Y axis first.
	for _, b := range nearby {
		dY = offsetY(entityBox, b, dY)
	}
	entityBox = translateBox(entityBox, 0, dY, 0)

	// Attempt auto-step for X/Z motion when blocked on ground.
	attemptStep := state.OnGround && (dX != 0 || dZ != 0)

	// X axis.
	origDX := dX
	for _, b := range nearby {
		dX = offsetX(entityBox, b, dX)
	}
	if attemptStep && !floatEqual(dX, origDX) {
		// Try stepping up by up to StepHeight and re-running X.
		stepped := translateBox(entityBox, 0, StepHeight, 0)
		newDX := origDX
		for _, b := range nearby {
			newDX = offsetX(stepped, b, newDX)
		}
		if math.Abs(float64(newDX)) > math.Abs(float64(dX)) {
			dX = newDX
			dY += StepHeight
			entityBox = translateBox(entityBox, 0, StepHeight, 0)
		}
	}
	entityBox = translateBox(entityBox, dX, 0, 0)

	// Z axis.
	origDZ := dZ
	for _, b := range nearby {
		dZ = offsetZ(entityBox, b, dZ)
	}
	if attemptStep && !floatEqual(dZ, origDZ) {
		stepped := translateBox(entityBox, 0, StepHeight, 0)
		newDZ := origDZ
		for _, b := range nearby {
			newDZ = offsetZ(stepped, b, newDZ)
		}
		if math.Abs(float64(newDZ)) > math.Abs(float64(dZ)) {
			dZ = newDZ
		}
	}

	out.Position = mgl32.Vec3{
		out.Position[0] + dX,
		out.Position[1] + dY,
		out.Position[2] + dZ,
	}

	// Update OnGround + velocity component zeroing.
	if !floatEqual(dY, out.Velocity[1]) {
		if out.Velocity[1] < 0 {
			out.OnGround = true
		}
		out.Velocity[1] = 0
	} else {
		// Y unobstructed; if we were moving up or down at all, we left the
		// ground.
		if math.Abs(float64(out.Velocity[1])) > epsilonVel {
			out.OnGround = false
		}
	}
	if !floatEqual(dX, out.Velocity[0]) {
		out.Velocity[0] = 0
	}
	if !floatEqual(dZ, out.Velocity[2]) {
		out.Velocity[2] = 0
	}
	return out
}

// epsilonVel matches Dragonfly's FloatEqual tolerance — below this is
// treated as "not moving" for ground checks.
const epsilonVel = 1e-5

func floatEqual(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < epsilonVel
}

// playerBBox returns the entity AABB centered on pos. The sneaking flag
// swaps height for SneakBBoxHei; swim uses SwimBBoxHeight. For β we only
// need standing/sneak; swim pose arrives with fluid work.
func playerBBox(pos mgl32.Vec3, sneaking bool) cube.BBox {
	h := BBoxHeight
	if sneaking {
		h = SneakBBoxHei
	}
	half := BBoxWidth / 2
	return cube.Box(
		float64(pos[0]-half), float64(pos[1]), float64(pos[2]-half),
		float64(pos[0]+half), float64(pos[1]+h), float64(pos[2]+half),
	)
}

// translateBox shifts a BBox by (dx, dy, dz) in float32 inputs.
func translateBox(b cube.BBox, dx, dy, dz float32) cube.BBox {
	return b.Translate(mgl32toVec64(mgl32.Vec3{dx, dy, dz}))
}

// blockBBoxesAround gathers every block BBox intersecting an AABB grown by
// (gx, gy, gz) on every side — enough to include any block the entity could
// collide with during a single-tick sweep.
func blockBBoxesAround(world meta.WorldTracker, box cube.BBox, gx, gy, gz float32) []cube.BBox {
	min := box.Min()
	max := box.Max()
	minX := int(math.Floor(min[0] - float64(gx)))
	minY := int(math.Floor(min[1] - float64(gy)))
	minZ := int(math.Floor(min[2] - float64(gz)))
	maxX := int(math.Ceil(max[0] + float64(gx)))
	maxY := int(math.Ceil(max[1] + float64(gy)))
	maxZ := int(math.Ceil(max[2] + float64(gz)))

	out := make([]cube.BBox, 0, (maxX-minX)*(maxY-minY)*(maxZ-minZ)+2)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				pos := cube.Pos{x, y, z}
				for _, b := range world.BlockBBoxes(pos) {
					out = append(out, b.Translate(posVec(pos)))
				}
			}
		}
	}
	return out
}

// offsetX/Y/Z wrap cube.BBox.{X,Y,Z}Offset with float32 in/out. Dragonfly
// uses float64 internally; we convert at the boundary.
func offsetX(entity, block cube.BBox, dx float32) float32 {
	return float32(entity.XOffset(block, float64(dx)))
}
func offsetY(entity, block cube.BBox, dy float32) float32 {
	return float32(entity.YOffset(block, float64(dy)))
}
func offsetZ(entity, block cube.BBox, dz float32) float32 {
	return float32(entity.ZOffset(block, float64(dz)))
}

// epsilonStuck returns true when pos is within a small epsilon of wall,
// indicating the entity is numerically inside the wall surface. This handles
// the case where floating-point rounding leaves the position slightly
// penetrating a block boundary, which would otherwise cause the sweep to
// compute no collision and allow ghosting through thin surfaces.
//
// Two cases are detected:
//  1. Bit-exact equality: pos == wall (same IEEE754 representation).
//  2. Tiny positive gap: pos > wall and (pos - wall) has fewer than 10 ULPs
//     — measured as the raw uint32 difference between the floating-point
//     representations of pos and wall (not as difference of the float values).
//     This is equivalent to Math.ulp(wall)*10 and detects sub-1e-5 penetration
//     at magnitudes near 1.0 (1 ULP ≈ 1.2e-7, 10 ULPs ≈ 1.2e-6).
//
// For detecting penetration at the coarser 1e-4 scale (as required by T5.3),
// IsEpsilonInsideBlock uses a direct float comparison instead.
func epsilonStuck(pos, wall float32) bool {
	pb := math.Float32bits(pos)
	wb := math.Float32bits(wall)
	if pb == wb {
		return true
	}
	// pos > wall: check raw ULP distance between representations.
	if pb > wb {
		return pb-wb < 10
	}
	return false
}

// epsilonInsideThreshold is the maximum penetration distance (blocks) that
// IsEpsilonInsideBlock considers as "numerically inside" a wall.
// 1e-4 covers rounding errors that accumulate across many ticks.
const epsilonInsideThreshold = float32(1e-4)

// IsEpsilonInsideBlock returns true when entityEdge is within epsilon of
// blockWall, indicating the entity is numerically inside the block surface.
// Callers should snap the position to the wall when this returns true to
// prevent sub-ULP penetration accumulating across ticks.
//
// Detects both bit-exact equality and differences below epsilonInsideThreshold.
func IsEpsilonInsideBlock(entityEdge, blockWall float32) bool {
	if epsilonStuck(entityEdge, blockWall) {
		return true
	}
	// Also catch values within the coarser 1e-4 threshold.
	diff := entityEdge - blockWall
	return diff > 0 && diff < epsilonInsideThreshold
}
