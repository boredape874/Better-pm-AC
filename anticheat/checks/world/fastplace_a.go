package world

import (
	"fmt"
	"sync"
	"time"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// FastPlaceACheck flags when block placement rate exceeds cfg.MaxBPS
// (blocks per second) within a rolling 1-second window.
// Implements anticheat.Detection.
type FastPlaceACheck struct {
	cfg config.FastPlaceConfig

	mu         sync.Mutex
	placeTimes []time.Time
}

func NewFastPlaceACheck(cfg config.FastPlaceConfig) *FastPlaceACheck {
	return &FastPlaceACheck{cfg: cfg}
}

func (*FastPlaceACheck) Type() string    { return "FastPlace" }
func (*FastPlaceACheck) SubType() string { return "A" }
func (*FastPlaceACheck) Description() string {
	return "Flags block placement rate exceeding the vanilla limit."
}
func (*FastPlaceACheck) Punishable() bool { return true }
func (c *FastPlaceACheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *FastPlaceACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    2,
		MaxBuffer:     4,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// RecordPlace records a block-placement event and returns (flagged, info).
func (c *FastPlaceACheck) RecordPlace() (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Second)

	// Prune entries outside the 1-second window.
	n := 0
	for _, t := range c.placeTimes {
		if !t.Before(cutoff) {
			c.placeTimes[n] = t
			n++
		}
	}
	c.placeTimes = append(c.placeTimes[:n], now)

	count := len(c.placeTimes)
	maxBPS := c.cfg.MaxBPS
	if maxBPS <= 0 {
		maxBPS = 10
	}

	if count > maxBPS {
		return true, fmt.Sprintf("placements=%d max_bps=%d", count, maxBPS)
	}
	return false, ""
}
