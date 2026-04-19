package entity

import (
	"sync"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
)

// Rewind implements meta.EntityRewind. Each tracked entity gets its own ring
// buffer; a single mutex protects the map. Per-entity writes are expected to
// be serialized by the proxy's packet loop, so Record/At contention is only
// between the packet goroutine and combat checks on different entities.
type Rewind struct {
	mu      sync.RWMutex
	buffers map[uint64]*ringBuffer
	// window is the ring size used for newly-tracked entities.
	window int
}

// NewRewind builds a Rewind with the default 40-tick window.
func NewRewind() *Rewind {
	return &Rewind{buffers: make(map[uint64]*ringBuffer), window: defaultWindow}
}

// NewRewindWithWindow lets callers override the ring size. Values outside
// [1, maxWindow] are clamped.
func NewRewindWithWindow(window int) *Rewind {
	if window < 1 {
		window = defaultWindow
	}
	if window > maxWindow {
		window = maxWindow
	}
	return &Rewind{buffers: make(map[uint64]*ringBuffer), window: window}
}

// Record stores one pose for rid at tick. If rid has no buffer yet, one is
// allocated lazily.
func (r *Rewind) Record(rid uint64, tick uint64, pos mgl32.Vec3, bbox cube.BBox, rot mgl32.Vec2) {
	r.mu.Lock()
	defer r.mu.Unlock()
	buf, ok := r.buffers[rid]
	if !ok {
		buf = newRingBuffer(r.window)
		r.buffers[rid] = buf
	}
	buf.push(meta.EntitySnapshot{Tick: tick, Position: pos, BBox: bbox, Rotation: rot})
}

// At returns the snapshot at or immediately before tick. ok is false when the
// entity is unknown or the requested tick predates the retained window.
func (r *Rewind) At(rid uint64, tick uint64) (meta.EntitySnapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	buf, ok := r.buffers[rid]
	if !ok {
		return meta.EntitySnapshot{}, false
	}
	return buf.at(tick)
}

// Purge frees the history for rid. Call when the server sends RemoveActor so
// the map does not grow unbounded.
func (r *Rewind) Purge(rid uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.buffers, rid)
}

// TrackedCount is a debug accessor exposing how many entities have buffers.
func (r *Rewind) TrackedCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.buffers)
}

// compile-time contract check
var _ meta.EntityRewind = (*Rewind)(nil)
