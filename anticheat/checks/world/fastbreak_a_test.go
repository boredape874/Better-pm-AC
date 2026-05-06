package world

import (
	"testing"

	"github.com/boredape874/Better-pm-AC/config"
)

// stoneBlockID is blockID=2 in our miningTime table → 7.5 seconds bare hand.
const stoneBlockID = uint32(2)

func TestFastBreakA_Flags(t *testing.T) {
	cfg := config.FastBreakConfig{Enabled: true, Policy: "kick", Violations: 3}
	chk := NewFastBreakACheck(cfg)

	chk.OnStartBreak(stoneBlockID)
	// Complete immediately (0 elapsed vs 7.5s expected * 0.8 = 6.0s minimum).
	flagged, info := chk.OnBreakComplete(stoneBlockID)
	if !flagged {
		t.Errorf("FastBreakA should have flagged (stone in 0s), info=%q", info)
	}
}

func TestFastBreakA_NoStartRecorded(t *testing.T) {
	cfg := config.FastBreakConfig{Enabled: true, Policy: "kick", Violations: 3}
	chk := NewFastBreakACheck(cfg)

	// No OnStartBreak call — should not flag.
	if f, _ := chk.OnBreakComplete(stoneBlockID); f {
		t.Error("FastBreakA should not flag when no start was recorded")
	}
}

func TestFastBreakA_Disabled(t *testing.T) {
	cfg := config.FastBreakConfig{Enabled: false, Policy: "kick", Violations: 3}
	chk := NewFastBreakACheck(cfg)

	chk.OnStartBreak(stoneBlockID)
	if f, _ := chk.OnBreakComplete(stoneBlockID); f {
		t.Error("FastBreakA should not flag when disabled")
	}
}
