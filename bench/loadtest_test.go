// Package bench hosts load / perf benchmarks for Better-pm-AC.
//
// The benchmarks here are invoked via `go test -bench=. -run=^$ ./bench/...`
// and are intentionally NOT gated behind `-short`. They aren't part of the
// unit-test signal; they answer the operational questions "how much cost
// does one packet add" and "does the proxy stay under p99 < 20ms at 100
// CCU" (design.md §14.3).
//
// Benchmarks are deliberately lightweight — no real RakNet, no real world
// tracker. That keeps them reproducible on CI runners whose wall clock is
// noisy. The numbers here are lower-bound: real traffic is strictly more
// expensive. Track deltas, not absolute values.
package bench

import (
	"io"
	"log/slog"
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

// fakeBitset is a zero-allocation InputDataLoader. All bits read as
// false so no BadPacket/E path triggers during the bench.
type fakeBitset struct{}

func (fakeBitset) Load(int) bool { return false }

func silentLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// setupCCU constructs a Manager with n registered players and returns it
// plus the slice of UUIDs. The caller then pumps packets through them.
// All checks run with default config — the point is to measure the full
// registry cost, not a subset.
func setupCCU(tb testing.TB, n int) (*anticheat.Manager, []uuid.UUID) {
	tb.Helper()
	m := anticheat.NewManager(config.Default().Anticheat, silentLog())
	ids := make([]uuid.UUID, n)
	for i := 0; i < n; i++ {
		ids[i] = uuid.New()
		m.AddPlayer(ids[i], "bench")
	}
	return m, ids
}

// BenchmarkOnInputSinglePlayer measures the cost of a single OnInput
// call end-to-end: all movement + BadPacket + Timer checks run. This is
// the per-packet floor cost; multiply by packet rate (20 TPS) and CCU
// to get total CPU load.
func BenchmarkOnInputSinglePlayer(b *testing.B) {
	m, ids := setupCCU(b, 1)
	pid := ids[0]
	bs := fakeBitset{}

	// Seed one tick so subsequent ticks exercise the full "delta"
	// paths rather than the first-tick initialisation branches.
	m.OnInput(pid, 1, mgl32.Vec3{0, 64, 0}, true, 0, 0, 1, bs)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Keep coordinates bounded so float drift doesn't matter.
		x := float32(i&0xFF) * 0.05
		m.OnInput(pid, uint64(2+i), mgl32.Vec3{x, 64, 0}, true, 0, 0, 1, bs)
	}
}

// BenchmarkOnInput100CCU simulates 100 players sending one input each
// per iteration, the baseline for the "100 concurrent users" target in
// design.md §14.3. Divide ns/op by 100 to get per-packet cost under
// contention (if any).
func BenchmarkOnInput100CCU(b *testing.B) {
	const ccu = 100
	m, ids := setupCCU(b, ccu)
	bs := fakeBitset{}

	// Seed each player.
	for _, id := range ids {
		m.OnInput(id, 1, mgl32.Vec3{0, 64, 0}, true, 0, 0, 1, bs)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j, id := range ids {
			x := float32((i+j)&0xFF) * 0.05
			m.OnInput(id, uint64(2+i), mgl32.Vec3{x, 64, 0}, true, 0, 0, 1, bs)
		}
	}
}

// BenchmarkOnAttack measures the combat-check path: Reach + KillAura/A
// + KillAura/C + AutoClicker/A-B. This path is rarer (attacks are
// bursty) but more expensive per call than OnInput.
func BenchmarkOnAttack(b *testing.B) {
	m, ids := setupCCU(b, 1)
	pid := ids[0]
	p := m.GetPlayer(pid)
	p.UpdatePosition(mgl32.Vec3{0, 64, 0}, true)

	const rid = uint64(42)
	p.UpdateEntityPos(rid, mgl32.Vec3{0, 65.62, 2.5}) // in-range
	target := uuid.New()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.OnAttack(pid, target, rid)
	}
}

// BenchmarkPlayerAddRemove measures the cost of session churn — joining
// and leaving — which stresses the detection-registry construction
// path. Relevant for servers with a rotating player pool (hub servers).
func BenchmarkPlayerAddRemove(b *testing.B) {
	m := anticheat.NewManager(config.Default().Anticheat, silentLog())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := uuid.New()
		m.AddPlayer(id, "churn")
		m.RemovePlayer(id)
	}
}
