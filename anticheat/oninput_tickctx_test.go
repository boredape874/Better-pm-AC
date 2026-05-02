package anticheat

import (
	"io"
	"log/slog"
	"testing"

	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

func TestOnInputBuildsTickContext(t *testing.T) {
	m := NewManager(config.Default().Anticheat, slog.New(slog.NewTextHandler(io.Discard, nil)))
	id := uuid.New()
	m.AddPlayer(id, "u")

	// Force a known ServerTick.
	m.SetServerTickForTest(100)

	m.OnInput(id, 95, mgl32.Vec3{0, 64, 0}, true, 0, 0, 1, fakeBitset{})

	ctx := m.LastTickContextForTest(id)
	if ctx.ServerTick != 100 || ctx.ClientTick != 95 {
		t.Fatalf("ctx=%+v want {100,95}", ctx)
	}
	if ctx.Skew() != 5 {
		t.Fatalf("skew=%d want 5", ctx.Skew())
	}
}
