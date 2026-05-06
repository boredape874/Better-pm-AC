package world

import (
	"testing"
)

// --- T5.4: Trapdoor bbox tests ---

// TestTrapdoorClosedIsBottomSlab checks that a closed trapdoor (any facing)
// is a flat slab at the bottom of the block, 3/16 blocks thick.
func TestTrapdoorClosedIsBottomSlab(t *testing.T) {
	for _, facing := range []TrapdoorFacing{TrapdoorNorth, TrapdoorSouth, TrapdoorEast, TrapdoorWest} {
		bbs := TrapdoorBBox(facing, false /* closed */)
		if len(bbs) != 1 {
			t.Errorf("facing %d closed: want 1 bbox, got %d", facing, len(bbs))
			continue
		}
		bb := bbs[0]
		const t316 = trapdoorThickness
		if bb.Min()[1] != 0 || bb.Max()[1] != t316 {
			t.Errorf("facing %d closed: Y range should be [0, %.4f], got [%.4f, %.4f]",
				facing, t316, bb.Min()[1], bb.Max()[1])
		}
		if bb.Min()[0] != 0 || bb.Max()[0] != 1 || bb.Min()[2] != 0 || bb.Max()[2] != 1 {
			t.Errorf("facing %d closed: XZ should be [0,1]x[0,1], got X=[%.4f,%.4f] Z=[%.4f,%.4f]",
				facing, bb.Min()[0], bb.Max()[0], bb.Min()[2], bb.Max()[2])
		}
	}
}

// TestTrapdoorOpenNorthIsNorthWall verifies that an open trapdoor facing North
// is a vertical panel against the north wall (Z=0 to 3/16).
func TestTrapdoorOpenNorthIsNorthWall(t *testing.T) {
	bbs := TrapdoorBBox(TrapdoorNorth, true)
	if len(bbs) != 1 {
		t.Fatalf("open north: want 1 bbox, got %d", len(bbs))
	}
	bb := bbs[0]
	const thick = trapdoorThickness
	if bb.Min()[2] != 0 || bb.Max()[2] != thick {
		t.Errorf("open north: Z should be [0, %.4f], got [%.4f, %.4f]", thick, bb.Min()[2], bb.Max()[2])
	}
	if bb.Min()[1] != 0 || bb.Max()[1] != 1 {
		t.Errorf("open north: Y should be [0,1], got [%.4f, %.4f]", bb.Min()[1], bb.Max()[1])
	}
}

// TestTrapdoorOpenSouthIsSouthWall verifies south facing trapdoor.
func TestTrapdoorOpenSouthIsSouthWall(t *testing.T) {
	bbs := TrapdoorBBox(TrapdoorSouth, true)
	if len(bbs) != 1 {
		t.Fatalf("open south: want 1 bbox, got %d", len(bbs))
	}
	bb := bbs[0]
	const thick = trapdoorThickness
	if bb.Min()[2] != 1-thick || bb.Max()[2] != 1 {
		t.Errorf("open south: Z should be [%.4f, 1], got [%.4f, %.4f]", 1-thick, bb.Min()[2], bb.Max()[2])
	}
}

// TestTrapdoorOpenEastIsEastWall verifies east facing trapdoor.
func TestTrapdoorOpenEastIsEastWall(t *testing.T) {
	bbs := TrapdoorBBox(TrapdoorEast, true)
	if len(bbs) != 1 {
		t.Fatalf("open east: want 1 bbox, got %d", len(bbs))
	}
	bb := bbs[0]
	const thick = trapdoorThickness
	if bb.Min()[0] != 1-thick || bb.Max()[0] != 1 {
		t.Errorf("open east: X should be [%.4f, 1], got [%.4f, %.4f]", 1-thick, bb.Min()[0], bb.Max()[0])
	}
}

// TestTrapdoorOpenWestIsWestWall verifies west facing trapdoor.
func TestTrapdoorOpenWestIsWestWall(t *testing.T) {
	bbs := TrapdoorBBox(TrapdoorWest, true)
	if len(bbs) != 1 {
		t.Fatalf("open west: want 1 bbox, got %d", len(bbs))
	}
	bb := bbs[0]
	const thick = trapdoorThickness
	if bb.Min()[0] != 0 || bb.Max()[0] != thick {
		t.Errorf("open west: X should be [0, %.4f], got [%.4f, %.4f]", thick, bb.Min()[0], bb.Max()[0])
	}
}

// --- T5.5: Fence/wall connectivity bbox tests ---

// TestFencePostOnlyHasOneBox verifies that an isolated fence post (no
// connections) has exactly 1 bbox — the central post.
func TestFencePostOnlyHasOneBox(t *testing.T) {
	bbs := FenceBBox(false, false, false, false)
	if len(bbs) != 1 {
		t.Fatalf("isolated fence: want 1 bbox (post only), got %d", len(bbs))
	}
	// Verify the post is centred on the block.
	bb := bbs[0]
	const lo, hi = float64(5) / 16, float64(11) / 16
	if bb.Min()[0] != lo || bb.Max()[0] != hi {
		t.Errorf("post X: want [%.4f, %.4f], got [%.4f, %.4f]", lo, hi, bb.Min()[0], bb.Max()[0])
	}
}

// TestFenceNSConnectionAddsTwoArms checks a north+south connected fence has
// the post plus two arm bboxes.
func TestFenceNSConnectionAddsTwoArms(t *testing.T) {
	bbs := FenceBBox(true, true, false, false)
	if len(bbs) != 3 {
		t.Fatalf("NS fence: want 3 bboxes (post + 2 arms), got %d", len(bbs))
	}
}

// TestFenceAllConnectionsFiveBoxes checks that connecting all 4 sides gives
// the post plus 4 arm bboxes.
func TestFenceAllConnectionsFiveBoxes(t *testing.T) {
	bbs := FenceBBox(true, true, true, true)
	if len(bbs) != 5 {
		t.Fatalf("full fence: want 5 bboxes (post + 4 arms), got %d", len(bbs))
	}
}

// TestWallConnectivity mirrors TestFenceNSConnectionAddsTwoArms for walls.
func TestWallConnectivity(t *testing.T) {
	bbs := WallBBox(true, false, true, false)
	if len(bbs) != 3 {
		t.Fatalf("NE wall: want 3 bboxes (post + 2 arms), got %d", len(bbs))
	}
}

// --- T5.6: Stair bbox tests ---

// TestStairStraightHasTwoBBoxes verifies a straight stair has 2 bboxes:
// the base slab and the back step.
func TestStairStraightHasTwoBBoxes(t *testing.T) {
	for _, facing := range []StairFacing{StairFacingNorth, StairFacingSouth, StairFacingEast, StairFacingWest} {
		bbs := StairBBox(facing, StairStraight, false)
		if len(bbs) != 2 {
			t.Errorf("straight stair facing %d: want 2 bboxes, got %d", facing, len(bbs))
		}
		// Base slab should be Y [0, 0.5].
		base := bbs[0]
		if base.Min()[1] != 0 || base.Max()[1] != 0.5 {
			t.Errorf("facing %d: base slab Y should be [0, 0.5], got [%.3f, %.3f]",
				facing, base.Min()[1], base.Max()[1])
		}
	}
}

// TestStairStraightUpsideDownFlips verifies that upsideDown=true swaps the
// slab and step Y ranges.
func TestStairStraightUpsideDownFlips(t *testing.T) {
	bbs := StairBBox(StairFacingNorth, StairStraight, true)
	if len(bbs) != 2 {
		t.Fatalf("upside-down stair: want 2 bboxes, got %d", len(bbs))
	}
	base := bbs[0]
	// Upside-down base slab should be Y [0.5, 1].
	if base.Min()[1] != 0.5 || base.Max()[1] != 1 {
		t.Errorf("upside-down base slab Y should be [0.5, 1], got [%.3f, %.3f]",
			base.Min()[1], base.Max()[1])
	}
}

// TestStairInnerCornerHasThreeBBoxes verifies an inner corner stair has 3
// bboxes: base slab + 2 step pieces forming an L.
func TestStairInnerCornerHasThreeBBoxes(t *testing.T) {
	bbs := StairBBox(StairFacingNorth, StairInnerCornerLeft, false)
	if len(bbs) != 3 {
		t.Fatalf("inner corner stair: want 3 bboxes, got %d", len(bbs))
	}
}

// TestStairOuterCornerHasTwoBBoxes verifies an outer corner stair has 2
// bboxes: base slab + 1 step piece (one back quadrant only).
func TestStairOuterCornerHasTwoBBoxes(t *testing.T) {
	bbs := StairBBox(StairFacingSouth, StairOuterCornerRight, false)
	if len(bbs) != 2 {
		t.Fatalf("outer corner stair: want 2 bboxes, got %d", len(bbs))
	}
}

// TestStairNorthFacingStepIsAtBack verifies that for a north-facing straight
// stair, the step is against the south wall (Z [0.5, 1]).
func TestStairNorthFacingStepIsAtBack(t *testing.T) {
	bbs := StairBBox(StairFacingNorth, StairStraight, false)
	// bbs[1] is the upper step for a straight stair.
	step := bbs[1]
	if step.Min()[2] < 0.49 || step.Max()[2] > 1.01 {
		t.Errorf("north-facing stair step should be at Z=[0.5,1], got Z=[%.3f, %.3f]",
			step.Min()[2], step.Max()[2])
	}
}

// TestStairSouthFacingStepIsAtBack verifies that for a south-facing straight
// stair, the step is against the north wall (Z [0, 0.5]).
func TestStairSouthFacingStepIsAtBack(t *testing.T) {
	bbs := StairBBox(StairFacingSouth, StairStraight, false)
	step := bbs[1]
	if step.Min()[2] < -0.01 || step.Max()[2] > 0.51 {
		t.Errorf("south-facing stair step should be at Z=[0,0.5], got Z=[%.3f, %.3f]",
			step.Min()[2], step.Max()[2])
	}
}
