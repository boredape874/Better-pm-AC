package combat

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

// TestRayCastDirectHit: ray fired straight at a 1×2 bbox (HalfWidth=0.5, Height=2)
// centred at {0,0,5} from origin {0,1,0} in direction +Z should hit.
func TestRayCastDirectHit(t *testing.T) {
	origin := mgl32.Vec3{0, 1, 0}
	dir := mgl32.Vec3{0, 0, 1}
	bbox := BBox{Pos: mgl32.Vec3{0, 0, 5}, HalfWidth: 0.5, Height: 2}

	snapshots := []BBox{bbox}
	result := CastN(origin, dir, snapshots, 10.0)
	if !result.Hit {
		t.Fatal("expected hit on direct ray through bbox")
	}
	if result.NearestT <= 0 {
		t.Fatalf("NearestT should be positive, got %f", result.NearestT)
	}
}

// TestRayCastMiss: ray fired 2m to the side of the bbox should miss.
func TestRayCastMiss(t *testing.T) {
	origin := mgl32.Vec3{2, 1, 0} // 2 blocks to the right of centre
	dir := mgl32.Vec3{0, 0, 1}
	bbox := BBox{Pos: mgl32.Vec3{0, 0, 5}, HalfWidth: 0.5, Height: 2}

	snapshots := []BBox{bbox}
	result := CastN(origin, dir, snapshots, 10.0)
	if result.Hit {
		t.Fatal("expected miss on ray 2m beside bbox")
	}
}

// TestRayCastMultipleTicksPicksNearest: two bboxes at different distances,
// ray hits both — CastN should return the nearer one.
func TestRayCastMultipleTicksPicksNearest(t *testing.T) {
	origin := mgl32.Vec3{0, 1, 0}
	dir := mgl32.Vec3{0, 0, 1}
	// Nearer bbox at Z=3, further at Z=7.
	near := BBox{Pos: mgl32.Vec3{0, 0, 3}, HalfWidth: 0.5, Height: 2}
	far := BBox{Pos: mgl32.Vec3{0, 0, 7}, HalfWidth: 0.5, Height: 2}

	snapshots := []BBox{far, near} // deliberately reversed to test ordering
	result := CastN(origin, dir, snapshots, 10.0)
	if !result.Hit {
		t.Fatal("expected hit")
	}
	// tMin for near bbox should be ~2.5 (near edge at Z=2.5 from origin at Z=0).
	if result.NearestT > 4.0 {
		t.Fatalf("NearestT too large (%f); expected nearer bbox picked", result.NearestT)
	}
}

// TestRayCastBeyondReach: ray hits bbox but bbox is beyond maxReach — no hit.
func TestRayCastBeyondReach(t *testing.T) {
	origin := mgl32.Vec3{0, 1, 0}
	dir := mgl32.Vec3{0, 0, 1}
	// Bbox at Z=10, so tMin ≈ 9.5, but maxReach is 3.1.
	bbox := BBox{Pos: mgl32.Vec3{0, 0, 10}, HalfWidth: 0.5, Height: 2}

	snapshots := []BBox{bbox}
	result := CastN(origin, dir, snapshots, 3.1)
	if result.Hit {
		t.Fatal("bbox beyond maxReach should not register as hit")
	}
}
