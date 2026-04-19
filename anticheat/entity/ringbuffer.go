package entity

import (
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
)

// defaultWindow is the number of ticks a ring buffer keeps by default.
// 40 ticks = 2s @ 20 TPS — matches design.md §5.3.2.
const defaultWindow = 40

// maxWindow is the hard upper bound when auto-extending for high-RTT players.
const maxWindow = 80

// ringBuffer stores per-tick entity snapshots in a fixed-size circular buffer.
// Writes are O(1); lookups bisect the populated range to find the snapshot at
// or just before the requested tick.
type ringBuffer struct {
	buf  []meta.EntitySnapshot
	next int
	// filled counts valid entries; caps at len(buf) once the ring wraps.
	filled int
}

func newRingBuffer(size int) *ringBuffer {
	if size <= 0 {
		size = defaultWindow
	}
	return &ringBuffer{buf: make([]meta.EntitySnapshot, size)}
}

// push appends a snapshot, overwriting the oldest slot when full.
func (r *ringBuffer) push(s meta.EntitySnapshot) {
	r.buf[r.next] = s
	r.next = (r.next + 1) % len(r.buf)
	if r.filled < len(r.buf) {
		r.filled++
	}
}

// at returns the snapshot whose tick is the largest value ≤ target. ok is false
// when the buffer has no entry at or before target (e.g. target predates the
// oldest retained tick, or buffer is empty).
func (r *ringBuffer) at(target uint64) (meta.EntitySnapshot, bool) {
	if r.filled == 0 {
		return meta.EntitySnapshot{}, false
	}
	// Walk newest to oldest — the hot path is "look up the current or last
	// tick", so linear scan from head is cheaper than sort/bisect for the
	// small (≤80) windows we use.
	best := meta.EntitySnapshot{}
	found := false
	for i := 0; i < r.filled; i++ {
		idx := (r.next - 1 - i + len(r.buf)) % len(r.buf)
		s := r.buf[idx]
		if s.Tick <= target {
			if !found || s.Tick > best.Tick {
				best = s
				found = true
			}
			// Entries are written in monotonically increasing tick order,
			// so the first one we find walking backward that is ≤ target
			// is the newest such entry.
			return best, true
		}
	}
	return best, found
}

// newest returns the most recently recorded snapshot, if any.
func (r *ringBuffer) newest() (meta.EntitySnapshot, bool) {
	if r.filled == 0 {
		return meta.EntitySnapshot{}, false
	}
	idx := (r.next - 1 + len(r.buf)) % len(r.buf)
	return r.buf[idx], true
}
