package world

import (
	"fmt"
	"sync"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// towerCycleThreshold is the number of consecutive jump-and-place-below cycles
// within towerWindowTicks that triggers a flag.
const towerCycleThreshold = 4

// towerWindowTicks is the rolling window (in server ticks at 20 TPS) for
// counting jump-place cycles. 2 seconds = 40 ticks.
const towerWindowTicks = 40

// towerFollowTicks is the maximum gap (ticks) between a jump and a
// place-below event to count as one cycle.
const towerFollowTicks = 10 // 0.5 s at 20 TPS

// TowerACheck flags when >= towerCycleThreshold jump-and-place-below
// cycles occur within towerWindowTicks. Implements anticheat.Detection.
type TowerACheck struct {
	cfg config.TowerConfig

	mu             sync.Mutex
	jumpTicks      []uint64
	placeBelowTicks []uint64
	cycleTicks     []uint64 // tick at which each cycle was detected
}

func NewTowerACheck(cfg config.TowerConfig) *TowerACheck {
	return &TowerACheck{cfg: cfg}
}

func (*TowerACheck) Type() string    { return "Tower" }
func (*TowerACheck) SubType() string { return "A" }
func (*TowerACheck) Description() string {
	return "Flags repeated jump-and-place-below cycles (self-tower cheats)."
}
func (*TowerACheck) Punishable() bool { return true }
func (c *TowerACheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *TowerACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    1,
		MaxBuffer:     3,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// OnJump records the tick at which the player jumped.
func (c *TowerACheck) OnJump(tick uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.jumpTicks = append(c.jumpTicks, tick)
}

// OnPlaceBelow records a block placement at or below the player's feet and
// checks if it follows a recent jump within towerFollowTicks. Returns
// (flagged, info) when towerCycleThreshold cycles are detected in the window.
func (c *TowerACheck) OnPlaceBelow(tick uint64) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.placeBelowTicks = append(c.placeBelowTicks, tick)

	// Check if any recent jump is within towerFollowTicks of this placement.
	isCycle := false
	for _, jt := range c.jumpTicks {
		if tick >= jt && tick-jt <= towerFollowTicks {
			isCycle = true
			break
		}
	}

	windowStart := uint64(0)
	if tick >= towerWindowTicks {
		windowStart = tick - towerWindowTicks
	}

	if isCycle {
		c.cycleTicks = append(c.cycleTicks, tick)
	}

	// Prune old cycles outside window.
	n := 0
	for _, ct := range c.cycleTicks {
		if ct >= windowStart {
			c.cycleTicks[n] = ct
			n++
		}
	}
	c.cycleTicks = c.cycleTicks[:n]

	// Prune old jumps outside window.
	nj := 0
	for _, jt := range c.jumpTicks {
		if jt >= windowStart {
			c.jumpTicks[nj] = jt
			nj++
		}
	}
	c.jumpTicks = c.jumpTicks[:nj]

	count := len(c.cycleTicks)
	if count >= towerCycleThreshold {
		return true, fmt.Sprintf("cycles=%d window=%d_ticks", count, towerWindowTicks)
	}
	return false, ""
}
