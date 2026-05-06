package world

import (
	"testing"

	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
)

func TestNukerA_Flags(t *testing.T) {
	cfg := config.NukerConfig{Enabled: true, Policy: "kick", Violations: 3}
	chk := NewNukerACheck(cfg)

	// 6 distinct breaks should trigger (limit is 5).
	var flagged bool
	for i := 0; i < 6; i++ {
		pos := mgl32.Vec3{float32(i), 64, 0}
		f, _ := chk.RecordBreak(pos)
		if f {
			flagged = true
		}
	}
	if !flagged {
		t.Error("NukerA should have flagged at 6 breaks/sec")
	}
}

func TestNukerA_NoFlag(t *testing.T) {
	cfg := config.NukerConfig{Enabled: true, Policy: "kick", Violations: 3}
	chk := NewNukerACheck(cfg)

	// 4 breaks — below the limit.
	for i := 0; i < 4; i++ {
		pos := mgl32.Vec3{float32(i), 64, 0}
		if f, _ := chk.RecordBreak(pos); f {
			t.Errorf("NukerA flagged at break %d (expected no flag)", i+1)
		}
	}
}

func TestNukerA_Disabled(t *testing.T) {
	cfg := config.NukerConfig{Enabled: false, Policy: "kick", Violations: 3}
	chk := NewNukerACheck(cfg)

	for i := 0; i < 20; i++ {
		pos := mgl32.Vec3{float32(i), 64, 0}
		if f, _ := chk.RecordBreak(pos); f {
			t.Error("NukerA should not flag when disabled")
		}
	}
}
