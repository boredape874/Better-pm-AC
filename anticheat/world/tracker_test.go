package world

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	dfworld "github.com/df-mc/dragonfly/server/world"
	dfchunk "github.com/df-mc/dragonfly/server/world/chunk"
	// Import dragonfly block package for its init() to register block states.
	// Without this, BlockByName/BlockByRuntimeID return unknownBlock.
	_ "github.com/df-mc/dragonfly/server/block"
)

// injectChunk mirrors what HandleLevelChunk does, minus the network decode.
// Tests use it to set up controlled chunk state without captured testdata.
func injectChunk(t *Tracker, cx, cz int32, c *dfchunk.Chunk) {
	t.mu.Lock()
	t.chunks[chunkKey{cx, cz}] = c
	t.mu.Unlock()
}

func TestEmptyTrackerHasNoChunks(t *testing.T) {
	tr := NewTracker()
	if tr.ChunkLoaded(0, 0) {
		t.Fatalf("new tracker should report chunk (0,0) unloaded")
	}
	b := tr.Block(cube.Pos{0, 64, 0})
	if name, _ := b.EncodeBlock(); name != "minecraft:air" {
		t.Fatalf("unloaded chunk should return air, got %s", name)
	}
	if bb := tr.BlockBBoxes(cube.Pos{0, 64, 0}); len(bb) != 0 {
		t.Fatalf("air should have no BBoxes, got %d", len(bb))
	}
}

func TestBlockUpdateRoundTrip(t *testing.T) {
	tr := NewTracker()
	air, _ := dfchunk.StateToRuntimeID("minecraft:air", nil)
	c := dfchunk.New(air, tr.rng)
	injectChunk(tr, 0, 0, c)

	stoneRID, ok := dfchunk.StateToRuntimeID("minecraft:stone", nil)
	if !ok {
		t.Skip("stone not registered; dragonfly block registry not initialized")
	}
	if err := tr.HandleBlockUpdate(cube.Pos{5, 64, 7}, stoneRID); err != nil {
		t.Fatalf("HandleBlockUpdate: %v", err)
	}

	b := tr.Block(cube.Pos{5, 64, 7})
	name, _ := b.EncodeBlock()
	if name != "minecraft:stone" {
		t.Fatalf("want stone after update, got %s", name)
	}

	// Full-cube block should have exactly one BBox spanning 0..1.
	bb := tr.BlockBBoxes(cube.Pos{5, 64, 7})
	if len(bb) != 1 {
		t.Fatalf("stone expected 1 bbox, got %d", len(bb))
	}
}

func TestBlockUpdateOutOfRange(t *testing.T) {
	tr := NewTracker()
	// Y above dimension max must silently drop; chunk isn't even checked.
	if err := tr.HandleBlockUpdate(cube.Pos{0, 9999, 0}, 1); err != nil {
		t.Fatalf("out-of-range update should not error, got %v", err)
	}
	if err := tr.HandleBlockUpdate(cube.Pos{0, -9999, 0}, 1); err != nil {
		t.Fatalf("out-of-range update should not error, got %v", err)
	}
}

func TestBlockUpdateForUnloadedChunk(t *testing.T) {
	tr := NewTracker()
	// No chunk present at (0,0); update should be a silent no-op.
	stoneRID, ok := dfchunk.StateToRuntimeID("minecraft:stone", nil)
	if !ok {
		t.Skip("block registry not initialized")
	}
	if err := tr.HandleBlockUpdate(cube.Pos{0, 64, 0}, stoneRID); err != nil {
		t.Fatalf("update for unloaded chunk should not error, got %v", err)
	}
	if tr.ChunkLoaded(0, 0) {
		t.Fatalf("update should not auto-create a chunk")
	}
}

func TestChunkXZNegativeCoords(t *testing.T) {
	// Verify floor-division semantics: (-1, -1) block is in chunk (-1, -1),
	// not (0, 0). A bug here would route queries to the wrong chunk.
	cases := []struct {
		pos        cube.Pos
		wantX, wZ int32
	}{
		{cube.Pos{0, 0, 0}, 0, 0},
		{cube.Pos{15, 0, 15}, 0, 0},
		{cube.Pos{16, 0, 16}, 1, 1},
		{cube.Pos{-1, 0, -1}, -1, -1},
		{cube.Pos{-16, 0, -16}, -1, -1},
		{cube.Pos{-17, 0, -17}, -2, -2},
	}
	for _, c := range cases {
		gotX, gotZ := chunkXZ(c.pos)
		if gotX != c.wantX || gotZ != c.wZ {
			t.Errorf("chunkXZ(%v) = (%d,%d), want (%d,%d)",
				c.pos, gotX, gotZ, c.wantX, c.wZ)
		}
	}
}

func TestCloseClearsChunks(t *testing.T) {
	tr := NewTracker()
	air, _ := dfchunk.StateToRuntimeID("minecraft:air", nil)
	injectChunk(tr, 0, 0, dfchunk.New(air, tr.rng))
	injectChunk(tr, 1, 0, dfchunk.New(air, tr.rng))

	if !tr.ChunkLoaded(0, 0) || !tr.ChunkLoaded(1, 0) {
		t.Fatalf("pre-close: chunks should be loaded")
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if tr.ChunkLoaded(0, 0) || tr.ChunkLoaded(1, 0) {
		t.Fatalf("post-close: chunks should be cleared")
	}
}

func TestLevelChunkCacheEnabledIsRejected(t *testing.T) {
	// Blob cache mode is β-unsupported. The packet should be silently
	// skipped without erroring — the proxy forwards many cached chunks
	// per session and logging each would be noisy.
	//
	// The test body is left thin: we don't own a real cached LevelChunk
	// payload. Once testdata capture (Task 2.W.0) lands, this test
	// upgrades to send a captured cache-enabled packet and verify
	// ChunkLoaded stays false.
	_ = dfworld.Overworld // keep import live
}
