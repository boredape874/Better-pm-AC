package world

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// Block returns the block at pos, or an air stub if the chunk is not loaded
// or the position is outside the tracker's vertical range. The stub is a
// canonical air block so checks that dispatch on block name get a safe
// default rather than nil.
//
// World-interactive checks (Phase, Scaffold, Water-walk) must call
// ChunkLoaded first and bail out on miss — trusting Block()=air when the
// chunk is simply unloaded would false-flag legitimate movement.
func (t *Tracker) Block(pos cube.Pos) world.Block {
	if pos[1] < t.rng.Min() || pos[1] > t.rng.Max() {
		return airBlock()
	}

	cx, cz := chunkXZ(pos)
	t.mu.RLock()
	c, ok := t.chunks[chunkKey{cx, cz}]
	t.mu.RUnlock()
	if !ok {
		return airBlock()
	}

	t.mu.RLock()
	rid := c.Block(uint8(pos[0]&15), int16(pos[1]), uint8(pos[2]&15), 0)
	t.mu.RUnlock()

	b, found := world.BlockByRuntimeID(rid)
	if !found {
		return airBlock()
	}
	return b
}

// BlockBBoxes returns the collision boxes for the block at pos. An empty
// slice means "not solid" and callers can skip collision tests. The
// Tracker itself implements world.BlockSource so block models that query
// neighbours (e.g. fences checking connectivity) see the correct world
// view.
func (t *Tracker) BlockBBoxes(pos cube.Pos) []cube.BBox {
	b := t.Block(pos)
	if b == nil {
		return nil
	}
	model := b.Model()
	if model == nil {
		// Unknown block: treat as full cube. A conservative default avoids
		// Fly/Speed false-negatives on unregistered block IDs from modded
		// servers.
		return []cube.BBox{cube.Box(0, 0, 0, 1, 1, 1)}
	}
	return model.BBox(pos, t)
}

// HandleBlockUpdate applies a single-block delta at pos. Called from the
// proxy when UpdateBlock packets flow server→client. A miss on the chunk
// map is silently ignored — an update for an unloaded chunk means the
// server sent it before the LevelChunk arrived (rare, edge cases around
// chunk transitions) and will be corrected by the next LevelChunk.
func (t *Tracker) HandleBlockUpdate(pos cube.Pos, rid uint32) error {
	if pos[1] < t.rng.Min() || pos[1] > t.rng.Max() {
		return nil
	}
	cx, cz := chunkXZ(pos)
	t.mu.Lock()
	defer t.mu.Unlock()
	c, ok := t.chunks[chunkKey{cx, cz}]
	if !ok {
		return nil
	}
	c.SetBlock(uint8(pos[0]&15), int16(pos[1]), uint8(pos[2]&15), 0, rid)
	return nil
}

// airBlock is the canonical air used when a lookup falls back. A single
// instance would be ideal but world.BlockByRuntimeID returns a value type
// and Dragonfly's BlockByName lookup requires an initialized registry.
// The lookup is cheap and the fallback path is exceptional.
func airBlock() world.Block {
	if b, ok := world.BlockByName("minecraft:air", nil); ok {
		return b
	}
	return nil
}

// chunkXZ floors a block coordinate to its containing chunk coordinate.
// Go's integer division truncates toward zero, so for negative X/Z we
// shift instead to match Minecraft's floor semantics.
func chunkXZ(pos cube.Pos) (int32, int32) {
	return int32(pos[0] >> 4), int32(pos[2] >> 4)
}
