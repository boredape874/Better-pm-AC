package combat

import (
	"github.com/go-gl/mathgl/mgl32"
)

// HitResult is the outcome of a ray cast against an entity bounding box.
type HitResult struct {
	Hit      bool
	NearestT float32 // parametric distance along ray to nearest hit; 0 if no hit
}

// BBox is an axis-aligned bounding box centred at Pos.
type BBox struct {
	Pos       mgl32.Vec3
	HalfWidth float32
	Height    float32
}

// intersectRayBBox tests whether ray (origin + t*dir) intersects bbox.
// Returns (tMin, true) on hit where tMin >= 0, or (0, false) on miss.
func intersectRayBBox(origin, dir mgl32.Vec3, bbox BBox) (float32, bool) {
	// slab method
	min := mgl32.Vec3{bbox.Pos[0] - bbox.HalfWidth, bbox.Pos[1], bbox.Pos[2] - bbox.HalfWidth}
	max := mgl32.Vec3{bbox.Pos[0] + bbox.HalfWidth, bbox.Pos[1] + bbox.Height, bbox.Pos[2] + bbox.HalfWidth}
	tMin := float32(0)
	tMax := float32(1e9)
	for i := 0; i < 3; i++ {
		if abs32(dir[i]) < 1e-7 {
			if origin[i] < min[i] || origin[i] > max[i] {
				return 0, false
			}
			continue
		}
		t1 := (min[i] - origin[i]) / dir[i]
		t2 := (max[i] - origin[i]) / dir[i]
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tMin {
			tMin = t1
		}
		if t2 < tMax {
			tMax = t2
		}
		if tMin > tMax || tMax < 0 {
			return 0, false
		}
	}
	return tMin, tMin >= 0
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// CastN samples the target entity's bounding box across multiple tick snapshots
// (using the entity rewind system) and returns the nearest hit.
// entitySnapshots is a slice of BBox samples across the window.
// maxReach is the maximum parametric distance (in blocks) for a valid hit.
func CastN(origin, dir mgl32.Vec3, snapshots []BBox, maxReach float32) HitResult {
	best := HitResult{}
	for _, bbox := range snapshots {
		t, hit := intersectRayBBox(origin, dir, bbox)
		if hit && t <= maxReach && (best.NearestT == 0 || t < best.NearestT) {
			best.Hit = true
			best.NearestT = t
		}
	}
	return best
}
