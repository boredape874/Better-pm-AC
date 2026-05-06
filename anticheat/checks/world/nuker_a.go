package world

import (
	"fmt"
	"sync"
	"time"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
)

// NukerACheck flags when >= MaxBreaksPerSec distinct-position breaks occur
// within a rolling 1-second window. Implements anticheat.Detection.
type NukerACheck struct {
	cfg config.NukerConfig

	mu         sync.Mutex
	breakTimes []time.Time
	breakPos   []mgl32.Vec3
}

func NewNukerACheck(cfg config.NukerConfig) *NukerACheck {
	return &NukerACheck{cfg: cfg}
}

func (*NukerACheck) Type() string        { return "Nuker" }
func (*NukerACheck) SubType() string     { return "A" }
func (*NukerACheck) Description() string { return "Flags multi-break rate exceeding the vanilla limit." }
func (*NukerACheck) Punishable() bool    { return true }
func (c *NukerACheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *NukerACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    1,
		MaxBuffer:     3,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// maxBreaksPerSec is the maximum number of distinct-position block breaks per
// second before Nuker/A flags. Vanilla survival allows roughly 1 break/sec per
// block (tool-dependent), so 5 distinct positions in 1 second is generous.
const maxBreaksPerSec = 5

// RecordBreak records a block-break event at pos and returns (flagged, info).
func (c *NukerACheck) RecordBreak(pos mgl32.Vec3) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Second)

	// Prune entries outside the 1-second window.
	n := 0
	for i, t := range c.breakTimes {
		if !t.Before(cutoff) {
			c.breakTimes[n] = t
			c.breakPos[n] = c.breakPos[i]
			n++
		}
	}
	c.breakTimes = c.breakTimes[:n]
	c.breakPos = c.breakPos[:n]

	// Append this break.
	c.breakTimes = append(c.breakTimes, now)
	c.breakPos = append(c.breakPos, pos)

	count := len(c.breakTimes)
	if count >= maxBreaksPerSec {
		return true, fmt.Sprintf("breaks=%d per_sec_limit=%d", count, maxBreaksPerSec)
	}
	return false, ""
}
