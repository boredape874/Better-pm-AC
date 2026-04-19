package sim

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/go-gl/mathgl/mgl64"
)

// mgl32toVec64 converts a mgl32.Vec3 to mgl64.Vec3 for Dragonfly interop.
func mgl32toVec64(v mgl32.Vec3) mgl64.Vec3 {
	return mgl64.Vec3{float64(v[0]), float64(v[1]), float64(v[2])}
}

// posVec turns a cube.Pos into the mgl64 vector used to translate BBoxes.
func posVec(p cube.Pos) mgl64.Vec3 {
	return mgl64.Vec3{float64(p[0]), float64(p[1]), float64(p[2])}
}
