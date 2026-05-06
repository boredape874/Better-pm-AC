package world

import (
	"fmt"
	"sync"
	"time"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// miningTime maps blockID (runtime-ID agnostic, using canonical numeric IDs)
// to the vanilla mining time in seconds with a bare hand.
// Values are representative; real checks would also factor in tool efficiency.
var miningTime = map[uint32]float32{
	// blockID → seconds (bare hand)
	// dirt
	1: 0.75,
	// stone
	2: 7.5,
	// grass
	3: 0.9,
	// log (oak)
	5: 3.0,
	// sand
	12: 0.75,
	// gravel
	13: 0.9,
	// planks
	5000: 3.0,
	// default fallback: use 1.5s for unregistered blocks
}

// defaultMiningTime is the fallback when a blockID is not in the table.
const defaultMiningTime = float32(1.5)

// fastBreakThresholdFraction is the minimum fraction of expected mining time that
// must elapse before a break is considered legitimate. 80% of vanilla time.
const fastBreakThresholdFraction = float32(0.8)

// FastBreakACheck flags when actual break interval < 80% of vanilla mining
// time. Implements anticheat.Detection.
type FastBreakACheck struct {
	cfg config.FastBreakConfig

	mu        sync.Mutex
	breakStart time.Time
	hasStart   bool
}

func NewFastBreakACheck(cfg config.FastBreakConfig) *FastBreakACheck {
	return &FastBreakACheck{cfg: cfg}
}

func (*FastBreakACheck) Type() string    { return "FastBreak" }
func (*FastBreakACheck) SubType() string { return "A" }
func (*FastBreakACheck) Description() string {
	return "Flags block breaks faster than vanilla mining time allows."
}
func (*FastBreakACheck) Punishable() bool { return true }
func (c *FastBreakACheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *FastBreakACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    2,
		MaxBuffer:     3,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// OnStartBreak records the time when the player began mining a block.
// blockID is the block's numeric ID.
func (c *FastBreakACheck) OnStartBreak(blockID uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.breakStart = time.Now()
	c.hasStart = true
}

// OnBreakComplete evaluates whether the break completed in less than 80% of
// the expected vanilla mining time. Returns (flagged, info).
// blockID is the block's numeric ID.
func (c *FastBreakACheck) OnBreakComplete(blockID uint32) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.hasStart {
		// No start time recorded; skip this break.
		return false, ""
	}

	elapsed := time.Since(c.breakStart).Seconds()
	c.hasStart = false

	expected := miningTime[blockID]
	if expected == 0 {
		expected = defaultMiningTime
	}

	minAllowed := float64(expected * fastBreakThresholdFraction)
	if elapsed < minAllowed {
		return true, fmt.Sprintf("elapsed=%.3fs min=%.3fs block=%d", elapsed, minAllowed, blockID)
	}
	return false, ""
}
