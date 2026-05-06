package world

import (
	"sync"

	"github.com/df-mc/dragonfly/server/block/cube"
)

// historyRingSize is the maximum number of block events stored per position.
const historyRingSize = 200

// BlockEvent records one block state transition at a position.
type BlockEvent struct {
	Tick   uint64
	PrevID uint32
	NewID  uint32
}

// blockRing is a fixed-size ring buffer of BlockEvent values.
type blockRing struct {
	buf  [historyRingSize]BlockEvent
	head int // next write index
	size int // how many valid entries (0..historyRingSize)
}

func (r *blockRing) push(ev BlockEvent) {
	r.buf[r.head] = ev
	r.head = (r.head + 1) % historyRingSize
	if r.size < historyRingSize {
		r.size++
	}
}

// slice returns the stored events in chronological order (oldest first).
func (r *blockRing) slice() []BlockEvent {
	out := make([]BlockEvent, r.size)
	start := (r.head - r.size + historyRingSize) % historyRingSize
	for i := 0; i < r.size; i++ {
		out[i] = r.buf[(start+i)%historyRingSize]
	}
	return out
}

// historyKey is a comparable 3-tuple for cube.Pos so it can be used as a map key.
type historyKey struct{ X, Y, Z int }

// historyStore holds per-position ring buffers.
type historyStore struct {
	mu  sync.RWMutex
	pos map[historyKey]*blockRing
}

func newHistoryStore() *historyStore {
	return &historyStore{pos: make(map[historyKey]*blockRing)}
}

func (s *historyStore) record(pos cube.Pos, ev BlockEvent) {
	k := historyKey{pos[0], pos[1], pos[2]}
	s.mu.Lock()
	r, ok := s.pos[k]
	if !ok {
		r = &blockRing{}
		s.pos[k] = r
	}
	r.push(ev)
	s.mu.Unlock()
}

func (s *historyStore) get(pos cube.Pos) []BlockEvent {
	k := historyKey{pos[0], pos[1], pos[2]}
	s.mu.RLock()
	r, ok := s.pos[k]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return r.slice()
}

// History returns up to 200 recent block events at pos.
func (t *Tracker) History(pos cube.Pos) []BlockEvent {
	if t.history == nil {
		return nil
	}
	return t.history.get(pos)
}

// recordBlockEvent appends a BlockEvent for the given position. Called
// by HandleBlockUpdate after the chunk has been updated.
func (t *Tracker) recordBlockEvent(pos cube.Pos, tick uint64, prevID, newID uint32) {
	if t.history == nil {
		return
	}
	t.history.record(pos, BlockEvent{Tick: tick, PrevID: prevID, NewID: newID})
}
