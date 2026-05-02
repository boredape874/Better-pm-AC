package anticheat

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/boredape874/Better-pm-AC/config"
)

func TestManagerServerTickAdvances(t *testing.T) {
	m := NewManager(config.Default().Anticheat, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer m.Stop()

	m.StartTicker()
	start := m.ServerTick()
	time.Sleep(120 * time.Millisecond) // ≥ 2 ticks at 20 TPS
	end := m.ServerTick()
	if end-start < 2 {
		t.Fatalf("ServerTick advanced %d ticks in 120ms; want ≥2", end-start)
	}
}
