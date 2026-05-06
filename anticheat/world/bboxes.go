package world

import (
	"github.com/df-mc/dragonfly/server/block/cube"
)

// TrapdoorFacing is the direction a trapdoor is hinged to.
type TrapdoorFacing uint8

const (
	TrapdoorNorth TrapdoorFacing = iota
	TrapdoorSouth
	TrapdoorEast
	TrapdoorWest
)

// trapdoorThickness is the block thickness of a closed/open trapdoor in units
// of 1 block. 3/16 matches the Bedrock/Java specification.
const trapdoorThickness = float64(3) / 16

// TrapdoorBBox returns the collision bounding box(es) for a trapdoor.
//
// A trapdoor is 3/16 blocks thick.
//   - Closed: flat panel at the bottom (Y range [0, 3/16]).
//   - Open: vertical panel against the hinge wall.
//     North → flat against Z=0 wall (Z range [0, 3/16]).
//     South → flat against Z=1 wall (Z range [13/16, 1]).
//     East  → flat against X=1 wall (X range [13/16, 1]).
//     West  → flat against X=0 wall (X range [0, 3/16]).
//
// All coordinates are block-local (0..1 per axis).
func TrapdoorBBox(facing TrapdoorFacing, open bool) []cube.BBox {
	const t = trapdoorThickness
	if !open {
		// Closed: thin slab at the floor (bottom half, which is the default).
		return []cube.BBox{cube.Box(0, 0, 0, 1, t, 1)}
	}
	// Open: vertical panel against the hinge side.
	switch facing {
	case TrapdoorNorth:
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1, t)}
	case TrapdoorSouth:
		return []cube.BBox{cube.Box(0, 0, 1-t, 1, 1, 1)}
	case TrapdoorEast:
		return []cube.BBox{cube.Box(1-t, 0, 0, 1, 1, 1)}
	case TrapdoorWest:
		return []cube.BBox{cube.Box(0, 0, 0, t, 1, 1)}
	default:
		return []cube.BBox{cube.Box(0, 0, 0, t, 1, 1)}
	}
}

// connectivityBBox returns the collision bounding boxes for a fence or wall
// post with optional arm connections in each cardinal direction.
//
// A fence/wall has a central 2×2 (pixel) post plus 2×8 arms in each connected
// direction. All dimensions are in block units (0..1 per axis).
//
//   - Post: centre 6/16 × full height × 6/16 (pixels 5–11 on X/Z).
//   - Arm north: X [5/16, 11/16], Z [0, 8/16].
//   - Arm south: X [5/16, 11/16], Z [8/16, 1].
//   - Arm east:  X [8/16, 1],     Z [5/16, 11/16].
//   - Arm west:  X [0, 8/16],     Z [5/16, 11/16].
//
// The post is always included. Arms are added for each true connection flag.
func connectivityBBox(north, south, east, west bool) []cube.BBox {
	const (
		lo  = float64(5) / 16  // 0.3125
		hi  = float64(11) / 16 // 0.6875
		mid = float64(8) / 16  // 0.5
	)

	bboxes := []cube.BBox{
		// Central post.
		cube.Box(lo, 0, lo, hi, 1, hi),
	}

	if north {
		bboxes = append(bboxes, cube.Box(lo, 0, 0, hi, 1, mid))
	}
	if south {
		bboxes = append(bboxes, cube.Box(lo, 0, mid, hi, 1, 1))
	}
	if east {
		bboxes = append(bboxes, cube.Box(mid, 0, lo, 1, 1, hi))
	}
	if west {
		bboxes = append(bboxes, cube.Box(0, 0, lo, mid, 1, hi))
	}
	return bboxes
}

// FenceBBox returns the collision bboxes for a fence with the given neighbour
// connections. This is an exported wrapper around connectivityBBox.
func FenceBBox(north, south, east, west bool) []cube.BBox {
	return connectivityBBox(north, south, east, west)
}

// WallBBox returns the collision bboxes for a wall with the given neighbour
// connections. Walls use the same connectivity shape as fences but with a
// slightly wider post (the shape is identical at our resolution).
func WallBBox(north, south, east, west bool) []cube.BBox {
	return connectivityBBox(north, south, east, west)
}

// StairShape describes the shape variant of a stair block.
type StairShape uint8

const (
	StairStraight          StairShape = iota // plain two-step stair
	StairInnerCornerLeft                     // inner corner, left turn
	StairInnerCornerRight                    // inner corner, right turn
	StairOuterCornerLeft                     // outer corner (concave), left
	StairOuterCornerRight                    // outer corner (concave), right
)

// StairFacing is the direction the stair faces (the direction the player
// walks up the stair — toward the lower step).
type StairFacing uint8

const (
	StairFacingNorth StairFacing = iota
	StairFacingSouth
	StairFacingEast
	StairFacingWest
)

// stairBBox returns the collision bboxes for a stair block.
//
// All stairs include a bottom slab (Y [0, 0.5]) that spans the full block.
// The upper step placement depends on facing and shape:
//
//   - Straight: upper half-slab on the back side (the side against the wall).
//   - InnerCorner: upper half-slab covers two adjacent back quadrants (L-shape
//     rotated into the corner).
//   - OuterCorner: upper half-slab covers one back quadrant only.
//
// When upsideDown=true the whole stair is flipped vertically: the bottom slab
// occupies Y [0.5, 1] and the step occupies Y [0, 0.5].
//
// All coordinates are block-local (0..1 per axis).
func stairBBox(facing StairFacing, shape StairShape, upsideDown bool) []cube.BBox {
	// base slab and step slab Y ranges.
	var baseY0, baseY1, stepY0, stepY1 float64
	if upsideDown {
		baseY0, baseY1 = 0.5, 1.0
		stepY0, stepY1 = 0.0, 0.5
	} else {
		baseY0, baseY1 = 0.0, 0.5
		stepY0, stepY1 = 0.5, 1.0
	}

	// Full-width base slab (always present).
	bboxes := []cube.BBox{cube.Box(0, baseY0, 0, 1, baseY1, 1)}

	// Upper step: position depends on facing and shape.
	switch shape {
	case StairStraight:
		// Back half of the block (the wall side).
		bboxes = append(bboxes, straightStepBBox(facing, stepY0, stepY1))
	case StairInnerCornerLeft:
		bboxes = append(bboxes, innerCornerBBoxes(facing, true, stepY0, stepY1)...)
	case StairInnerCornerRight:
		bboxes = append(bboxes, innerCornerBBoxes(facing, false, stepY0, stepY1)...)
	case StairOuterCornerLeft:
		bboxes = append(bboxes, outerCornerBBox(facing, true, stepY0, stepY1))
	case StairOuterCornerRight:
		bboxes = append(bboxes, outerCornerBBox(facing, false, stepY0, stepY1))
	}
	return bboxes
}

// StairBBox is the exported version of stairBBox.
func StairBBox(facing StairFacing, shape StairShape, upsideDown bool) []cube.BBox {
	return stairBBox(facing, shape, upsideDown)
}

// straightStepBBox returns the upper step box for a straight stair: the back
// half of the block (the half against the wall the player walks toward).
func straightStepBBox(facing StairFacing, y0, y1 float64) cube.BBox {
	switch facing {
	case StairFacingNorth:
		// Player walks north → back is south (Z=0.5 to 1).
		return cube.Box(0, y0, 0.5, 1, y1, 1)
	case StairFacingSouth:
		// Player walks south → back is north (Z=0 to 0.5).
		return cube.Box(0, y0, 0, 1, y1, 0.5)
	case StairFacingEast:
		// Player walks east → back is west (X=0 to 0.5).
		return cube.Box(0, y0, 0, 0.5, y1, 1)
	case StairFacingWest:
		// Player walks west → back is east (X=0.5 to 1).
		return cube.Box(0.5, y0, 0, 1, y1, 1)
	default:
		return cube.Box(0, y0, 0.5, 1, y1, 1)
	}
}

// innerCornerBBoxes returns the two upper-step boxes for an inner-corner stair.
// An inner corner fills 3 of the 4 top quadrants. leftTurn determines which
// three quadrants are filled (rotating the L left or right).
func innerCornerBBoxes(facing StairFacing, leftTurn bool, y0, y1 float64) []cube.BBox {
	// Inner corners: the step covers the back half plus one side half.
	// We return 2 non-overlapping bboxes covering those 3 quadrants.
	back := straightStepBBox(facing, y0, y1)

	var side cube.BBox
	switch facing {
	case StairFacingNorth:
		if leftTurn {
			side = cube.Box(0, y0, 0, 0.5, y1, 0.5) // NW quadrant
		} else {
			side = cube.Box(0.5, y0, 0, 1, y1, 0.5) // NE quadrant
		}
	case StairFacingSouth:
		if leftTurn {
			side = cube.Box(0.5, y0, 0.5, 1, y1, 1) // SE quadrant
		} else {
			side = cube.Box(0, y0, 0.5, 0.5, y1, 1) // SW quadrant
		}
	case StairFacingEast:
		if leftTurn {
			side = cube.Box(0.5, y0, 0, 1, y1, 0.5) // NE quadrant
		} else {
			side = cube.Box(0.5, y0, 0.5, 1, y1, 1) // SE quadrant
		}
	case StairFacingWest:
		if leftTurn {
			side = cube.Box(0, y0, 0.5, 0.5, y1, 1) // SW quadrant
		} else {
			side = cube.Box(0, y0, 0, 0.5, y1, 0.5) // NW quadrant
		}
	}
	return []cube.BBox{back, side}
}

// outerCornerBBox returns the single upper-step box for an outer-corner stair.
// An outer corner fills only 1 of the 4 top quadrants (the back-corner one).
func outerCornerBBox(facing StairFacing, leftTurn bool, y0, y1 float64) cube.BBox {
	switch facing {
	case StairFacingNorth:
		if leftTurn {
			return cube.Box(0, y0, 0.5, 0.5, y1, 1) // SW back-corner
		}
		return cube.Box(0.5, y0, 0.5, 1, y1, 1) // SE back-corner
	case StairFacingSouth:
		if leftTurn {
			return cube.Box(0.5, y0, 0, 1, y1, 0.5) // NE back-corner
		}
		return cube.Box(0, y0, 0, 0.5, y1, 0.5) // NW back-corner
	case StairFacingEast:
		if leftTurn {
			return cube.Box(0, y0, 0, 0.5, y1, 0.5) // NW back-corner
		}
		return cube.Box(0, y0, 0.5, 0.5, y1, 1) // SW back-corner
	case StairFacingWest:
		if leftTurn {
			return cube.Box(0.5, y0, 0.5, 1, y1, 1) // SE back-corner
		}
		return cube.Box(0.5, y0, 0, 1, y1, 0.5) // NE back-corner
	default:
		return cube.Box(0.5, y0, 0.5, 1, y1, 1)
	}
}
