package world

import (
	"testing"

	"github.com/boredape874/Better-pm-AC/config"
)

func TestFastPlaceA_Flags(t *testing.T) {
	cfg := config.FastPlaceConfig{Enabled: true, Policy: "kick", MaxBPS: 5, Violations: 5}
	chk := NewFastPlaceACheck(cfg)

	var flagged bool
	// 6 placements with MaxBPS=5 should flag.
	for i := 0; i < 6; i++ {
		f, _ := chk.RecordPlace()
		if f {
			flagged = true
		}
	}
	if !flagged {
		t.Error("FastPlaceA should have flagged at 6 placements (max=5)")
	}
}

func TestFastPlaceA_NoFlag(t *testing.T) {
	cfg := config.FastPlaceConfig{Enabled: true, Policy: "kick", MaxBPS: 10, Violations: 5}
	chk := NewFastPlaceACheck(cfg)

	for i := 0; i < 10; i++ {
		if f, _ := chk.RecordPlace(); f {
			t.Errorf("FastPlaceA flagged at placement %d (max=10)", i+1)
		}
	}
}

func TestFastPlaceA_Disabled(t *testing.T) {
	cfg := config.FastPlaceConfig{Enabled: false, Policy: "kick", MaxBPS: 1, Violations: 5}
	chk := NewFastPlaceACheck(cfg)

	for i := 0; i < 20; i++ {
		if f, _ := chk.RecordPlace(); f {
			t.Error("FastPlaceA should not flag when disabled")
		}
	}
}
