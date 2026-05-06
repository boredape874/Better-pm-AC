package world

import (
	"testing"

	"github.com/boredape874/Better-pm-AC/config"
)

func TestTowerA_Flags(t *testing.T) {
	cfg := config.TowerConfig{Enabled: true, Policy: "kick", Violations: 5}
	chk := NewTowerACheck(cfg)

	// Simulate 4 jump-place cycles rapidly.
	var flagged bool
	for i := uint64(0); i < 4; i++ {
		tick := i * 5 // 5 ticks apart
		chk.OnJump(tick)
		f, _ := chk.OnPlaceBelow(tick + 2) // place 2 ticks after jump
		if f {
			flagged = true
		}
	}
	if !flagged {
		t.Error("TowerA should have flagged at 4 jump-place cycles")
	}
}

func TestTowerA_NoFlagSlowCycles(t *testing.T) {
	cfg := config.TowerConfig{Enabled: true, Policy: "kick", Violations: 5}
	chk := NewTowerACheck(cfg)

	// Cycles spaced far apart — each outside the 2-second window relative to
	// the previous. Only 1 cycle should be in the window at a time.
	for i := uint64(0); i < 4; i++ {
		tick := i * 100 // 100 ticks apart (5 seconds between each)
		chk.OnJump(tick)
		if f, _ := chk.OnPlaceBelow(tick + 2); f {
			t.Errorf("TowerA should not flag when cycles are spread >2s apart (i=%d)", i)
		}
	}
}

func TestTowerA_Disabled(t *testing.T) {
	cfg := config.TowerConfig{Enabled: false, Policy: "kick", Violations: 5}
	chk := NewTowerACheck(cfg)

	for i := uint64(0); i < 10; i++ {
		chk.OnJump(i * 2)
		if f, _ := chk.OnPlaceBelow(i*2 + 1); f {
			t.Error("TowerA should not flag when disabled")
		}
	}
}
