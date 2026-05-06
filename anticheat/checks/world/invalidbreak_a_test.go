package world

import (
	"testing"

	"github.com/boredape874/Better-pm-AC/config"
	"github.com/df-mc/dragonfly/server/block/cube"
	dfworld "github.com/df-mc/dragonfly/server/world"
	_ "github.com/df-mc/dragonfly/server/block"
	"github.com/go-gl/mathgl/mgl32"
)

// simpleWorld is a WorldReader backed by a set of solid-block positions.
// It returns real dragonfly blocks by name lookup so the check's EncodeBlock
// comparison works correctly.
type simpleWorld struct {
	solid map[cube.Pos]bool
}

func (s *simpleWorld) Block(pos cube.Pos) dfworld.Block {
	if s.solid[pos] {
		if b, ok := dfworld.BlockByName("minecraft:stone", nil); ok {
			return b
		}
	}
	if b, ok := dfworld.BlockByName("minecraft:air", nil); ok {
		return b
	}
	return nil
}

func TestInvalidBreakA_ClearLOS(t *testing.T) {
	cfg := config.InvalidBreakConfig{Enabled: true, Policy: "kick", Violations: 3}
	chk := NewInvalidBreakACheck(cfg)

	w := &simpleWorld{solid: map[cube.Pos]bool{}}
	eye := mgl32.Vec3{0, 2, 0}
	target := cube.Pos{3, 1, 0}

	flagged, _ := chk.Check(eye, target, w)
	if flagged {
		t.Error("should not flag when path is clear")
	}
}

func TestInvalidBreakA_BlockedLOS(t *testing.T) {
	cfg := config.InvalidBreakConfig{Enabled: true, Policy: "kick", Violations: 3}
	chk := NewInvalidBreakACheck(cfg)

	// Place a wall block between eye and target on the path.
	w := &simpleWorld{solid: map[cube.Pos]bool{
		{2, 1, 0}: true, // blocking block between eye (0,2,0) and target (5,1,0)
	}}
	eye := mgl32.Vec3{0, 2, 0}
	target := cube.Pos{5, 1, 0}

	flagged, info := chk.Check(eye, target, w)
	if !flagged {
		t.Errorf("should flag when path is blocked (info=%q)", info)
	}
}

func TestInvalidBreakA_Disabled(t *testing.T) {
	cfg := config.InvalidBreakConfig{Enabled: false, Policy: "kick", Violations: 3}
	chk := NewInvalidBreakACheck(cfg)

	w := &simpleWorld{solid: map[cube.Pos]bool{
		{2, 1, 0}: true,
	}}
	eye := mgl32.Vec3{0, 2, 0}
	target := cube.Pos{5, 1, 0}

	if f, _ := chk.Check(eye, target, w); f {
		t.Error("should not flag when disabled")
	}
}
