package entity

import (
	"testing"

	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/go-gl/mathgl/mgl32"
)

func TestRewindRecordAndAtExactTick(t *testing.T) {
	r := NewRewind()
	r.Record(1, 10, mgl32.Vec3{1, 2, 3}, cube.Box(0, 0, 0, 1, 1, 1), mgl32.Vec2{})
	s, ok := r.At(1, 10)
	if !ok {
		t.Fatal("expected snapshot at tick 10")
	}
	if s.Tick != 10 || s.Position != (mgl32.Vec3{1, 2, 3}) {
		t.Fatalf("unexpected snapshot %+v", s)
	}
}

func TestRewindAtReturnsPriorTick(t *testing.T) {
	r := NewRewind()
	r.Record(2, 5, mgl32.Vec3{}, cube.Box(0, 0, 0, 1, 1, 1), mgl32.Vec2{})
	r.Record(2, 10, mgl32.Vec3{1, 0, 0}, cube.Box(0, 0, 0, 1, 1, 1), mgl32.Vec2{})
	// Query a tick between the two records — should return the earlier one.
	s, ok := r.At(2, 7)
	if !ok {
		t.Fatal("expected fallback snapshot")
	}
	if s.Tick != 5 {
		t.Fatalf("want tick 5, got %d", s.Tick)
	}
}

func TestRewindAtBeforeOldestReturnsFalse(t *testing.T) {
	r := NewRewindWithWindow(4)
	for tick := uint64(10); tick < 14; tick++ {
		r.Record(3, tick, mgl32.Vec3{}, cube.Box(0, 0, 0, 1, 1, 1), mgl32.Vec2{})
	}
	if _, ok := r.At(3, 5); ok {
		t.Fatal("expected no snapshot before oldest retained tick")
	}
}

func TestRewindRingOverflow(t *testing.T) {
	r := NewRewindWithWindow(4)
	// Push 6 entries into a size-4 ring: ticks 0..5 → ring retains 2..5.
	for tick := uint64(0); tick < 6; tick++ {
		r.Record(4, tick, mgl32.Vec3{float32(tick), 0, 0}, cube.Box(0, 0, 0, 1, 1, 1), mgl32.Vec2{})
	}
	// Tick 5 must still be retrievable.
	if s, ok := r.At(4, 5); !ok || s.Tick != 5 {
		t.Fatalf("want tick 5, got %v ok=%v", s.Tick, ok)
	}
	// Tick 1 fell out of the window — and is older than any retained snapshot
	// so the ring cannot return a suitable entry.
	if _, ok := r.At(4, 1); ok {
		t.Fatal("tick 1 should be outside the retained window")
	}
}

func TestRewindPurgeRemovesBuffer(t *testing.T) {
	r := NewRewind()
	r.Record(9, 0, mgl32.Vec3{}, cube.Box(0, 0, 0, 1, 1, 1), mgl32.Vec2{})
	if r.TrackedCount() != 1 {
		t.Fatalf("want 1 tracked, got %d", r.TrackedCount())
	}
	r.Purge(9)
	if r.TrackedCount() != 0 {
		t.Fatalf("want 0 after purge, got %d", r.TrackedCount())
	}
	if _, ok := r.At(9, 0); ok {
		t.Fatal("purged entity should have no snapshots")
	}
}

func TestRewindWindowClamped(t *testing.T) {
	r := NewRewindWithWindow(-10)
	if r.window != defaultWindow {
		t.Fatalf("negative window should clamp to default, got %d", r.window)
	}
	r = NewRewindWithWindow(999)
	if r.window != maxWindow {
		t.Fatalf("oversized window should clamp to max, got %d", r.window)
	}
}
