package world

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	dfchunk "github.com/df-mc/dragonfly/server/world/chunk"
	_ "github.com/df-mc/dragonfly/server/block"
)

func TestHistory_RingBuffer(t *testing.T) {
	tr := NewTracker()
	air, _ := dfchunk.StateToRuntimeID("minecraft:air", nil)
	c := dfchunk.New(air, tr.rng)
	injectChunk(tr, 0, 0, c)

	stoneRID, ok := dfchunk.StateToRuntimeID("minecraft:stone", nil)
	if !ok {
		t.Skip("block registry not initialized")
	}

	pos := cube.Pos{5, 64, 5}

	// Record 5 updates at the same position.
	for i := 0; i < 5; i++ {
		if err := tr.HandleBlockUpdate(pos, stoneRID); err != nil {
			t.Fatalf("HandleBlockUpdate: %v", err)
		}
	}

	hist := tr.History(pos)
	if len(hist) != 5 {
		t.Errorf("want 5 history entries, got %d", len(hist))
	}
}

func TestHistory_RingBuffer_Overflow(t *testing.T) {
	tr := NewTracker()
	air, _ := dfchunk.StateToRuntimeID("minecraft:air", nil)
	c := dfchunk.New(air, tr.rng)
	injectChunk(tr, 0, 0, c)

	stoneRID, ok := dfchunk.StateToRuntimeID("minecraft:stone", nil)
	if !ok {
		t.Skip("block registry not initialized")
	}

	pos := cube.Pos{1, 64, 1}

	// Write 250 events — more than historyRingSize (200).
	for i := 0; i < 250; i++ {
		if err := tr.HandleBlockUpdate(pos, stoneRID); err != nil {
			t.Fatalf("HandleBlockUpdate[%d]: %v", i, err)
		}
	}

	hist := tr.History(pos)
	if len(hist) != historyRingSize {
		t.Errorf("want %d history entries (ring cap), got %d", historyRingSize, len(hist))
	}
}

func TestHistory_EmptyPosition(t *testing.T) {
	tr := NewTracker()
	hist := tr.History(cube.Pos{0, 64, 0})
	if hist != nil {
		t.Errorf("want nil for position with no history, got %v", hist)
	}
}
