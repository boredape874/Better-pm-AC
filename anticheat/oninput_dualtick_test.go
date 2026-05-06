package anticheat

import (
	"io"
	"log/slog"
	"testing"

	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

func TestOnInputRecordsClaimedPosAndAdvancesTick(t *testing.T) {
	m := NewManager(config.Default().Anticheat, slog.New(slog.NewTextHandler(io.Discard, nil)))
	id := uuid.New()
	m.AddPlayer(id, "u")

	pos := mgl32.Vec3{1, 64, 0}
	m.OnInput(id, 5, pos, true, 0, 0, 1, fakeBitset{})

	p := m.GetPlayer(id)
	if p.ClaimedPos() != pos {
		t.Fatalf("ClaimedPos=%v want %v", p.ClaimedPos(), pos)
	}
	if p.LastClientTick() != 5 {
		t.Fatalf("LastClientTick=%d want 5", p.LastClientTick())
	}
}
