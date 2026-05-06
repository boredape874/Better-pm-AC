package world

import (
	"fmt"
	"math"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/df-mc/dragonfly/server/block/cube"
	dfworld "github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
)

// WorldReader is the minimal world interface needed for LOS checks.
type WorldReader interface {
	Block(pos cube.Pos) dfworld.Block
}

// InvalidBreakACheck flags when a broken block has no clear LOS from the
// player's eye position. Uses a DDA voxel raycast.
// Implements anticheat.Detection.
type InvalidBreakACheck struct {
	cfg config.InvalidBreakConfig
}

func NewInvalidBreakACheck(cfg config.InvalidBreakConfig) *InvalidBreakACheck {
	return &InvalidBreakACheck{cfg: cfg}
}

func (*InvalidBreakACheck) Type() string    { return "InvalidBreak" }
func (*InvalidBreakACheck) SubType() string { return "A" }
func (*InvalidBreakACheck) Description() string {
	return "Flags block breaks with no clear line-of-sight from the player's eye."
}
func (*InvalidBreakACheck) Punishable() bool { return true }
func (c *InvalidBreakACheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *InvalidBreakACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    2,
		MaxBuffer:     4,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// invalidBreakEyeHeight is the vertical offset from feet to eye level.
const invalidBreakEyeHeight = float32(1.62)

// Check evaluates whether the player has a clear LOS to the broken block.
// eye is the player's eye position (feet + eyeHeight). target is the broken
// block position. w is the world reader for block lookups. Returns (flagged, info).
func (c *InvalidBreakACheck) Check(eye mgl32.Vec3, target cube.Pos, w WorldReader) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if !c.hasLOS(eye, target, w) {
		return true, fmt.Sprintf("no_los eye=(%.1f,%.1f,%.1f) target=(%d,%d,%d)",
			eye[0], eye[1], eye[2], target[0], target[1], target[2])
	}
	return false, ""
}

// hasLOS raycasts from eye to target block center using DDA.
// Returns true if the path is clear (no blocking voxels before target).
func (c *InvalidBreakACheck) hasLOS(eye mgl32.Vec3, target cube.Pos, w WorldReader) bool {
	// Target center.
	tx := float64(target[0]) + 0.5
	ty := float64(target[1]) + 0.5
	tz := float64(target[2]) + 0.5

	dx := tx - float64(eye[0])
	dy := ty - float64(eye[1])
	dz := tz - float64(eye[2])

	length := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if length < 1e-6 {
		return true // eye is inside target block
	}

	// DDA: step through voxels along the ray.
	steps := int(length/0.5) + 2 // sample every 0.5 blocks
	stepX := dx / float64(steps)
	stepY := dy / float64(steps)
	stepZ := dz / float64(steps)

	cx := float64(eye[0])
	cy := float64(eye[1])
	cz := float64(eye[2])

	for i := 1; i < steps; i++ {
		cx += stepX
		cy += stepY
		cz += stepZ

		bx := int(math.Floor(cx))
		by := int(math.Floor(cy))
		bz := int(math.Floor(cz))

		// Skip the target block itself.
		if bx == target[0] && by == target[1] && bz == target[2] {
			continue
		}

		pos := cube.Pos{bx, by, bz}
		b := w.Block(pos)
		if b == nil {
			continue
		}
		name, _ := b.EncodeBlock()
		if name == "minecraft:air" {
			continue
		}
		// A non-air block blocks the ray.
		return false
	}
	return true
}
