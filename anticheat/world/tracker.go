package world

import (
	"sync"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/df-mc/dragonfly/server/block/cube"
	dfchunk "github.com/df-mc/dragonfly/server/world/chunk"
)

// Tracker is the server-authoritative block view. It is fed LevelChunk and
// SubChunk packets from the proxy and services Block() / BlockBBoxes() for
// checks. Reads use an RLock; writes (chunk load, block update) take a full
// lock — checks outnumber world changes by orders of magnitude so this skew
// favours the common path.
type Tracker struct {
	mu     sync.RWMutex
	rng    cube.Range
	air    uint32
	chunks map[chunkKey]*dfchunk.Chunk
}

type chunkKey struct{ X, Z int32 }

// NewTracker returns an empty Tracker for the Overworld. For Nether/End,
// pass a different cube.Range explicitly via NewTrackerWithRange.
func NewTracker() *Tracker {
	return NewTrackerWithRange(cube.Range{-64, 319})
}

// NewTrackerWithRange constructs a Tracker for a custom dimension range.
// Used for Nether ([0,127]) and End ([0,255]) when the proxy forwards a
// LevelChunk with Dimension != 0.
func NewTrackerWithRange(r cube.Range) *Tracker {
	// Dragonfly's chunk decoder needs the air runtime ID up front. This is
	// wired in the world package init, so importing anticheat/world for the
	// first time is a hard dependency on server/world being linked.
	air, _ := dfchunk.StateToRuntimeID("minecraft:air", nil)
	return &Tracker{
		rng:    r,
		air:    air,
		chunks: map[chunkKey]*dfchunk.Chunk{},
	}
}

// ChunkLoaded reports whether a chunk at (x,z) has been ingested. Checks
// that query a position in an unloaded chunk must fail open, not guess.
func (t *Tracker) ChunkLoaded(x, z int32) bool {
	t.mu.RLock()
	_, ok := t.chunks[chunkKey{x, z}]
	t.mu.RUnlock()
	return ok
}

// Close drops all cached chunks. Called on player disconnect.
func (t *Tracker) Close() error {
	t.mu.Lock()
	t.chunks = map[chunkKey]*dfchunk.Chunk{}
	t.mu.Unlock()
	return nil
}

// compile-time: Tracker satisfies meta.WorldTracker.
var _ meta.WorldTracker = (*Tracker)(nil)
