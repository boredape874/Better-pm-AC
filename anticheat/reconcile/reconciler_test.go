package reconcile

import (
	"testing"

	"github.com/go-gl/mathgl/mgl32"
)

// withinTol is the tolerance value used across all cases, matching the value
// wired into Manager.OnInput (0.5 metres).
const withinTol = float32(0.5)

func TestDecide_Accept(t *testing.T) {
	t.Parallel()
	claimed := mgl32.Vec3{10, 64, 10}
	expected := mgl32.Vec3{10.2, 64, 10}
	in := Input{
		Claimed:       claimed,
		Expected:      expected,
		HasPendingAck: false,
		Tolerance:     withinTol,
	}
	got := Decide(in)
	if got.Outcome != OutcomeAccept {
		t.Fatalf("expected OutcomeAccept, got %v", got.Outcome)
	}
	if got.Committed != claimed {
		t.Fatalf("expected Committed == Claimed %v, got %v", claimed, got.Committed)
	}
}

func TestDecide_Pending(t *testing.T) {
	t.Parallel()
	claimed := mgl32.Vec3{15, 64, 15}
	expected := mgl32.Vec3{10, 64, 10}
	in := Input{
		Claimed:       claimed,
		Expected:      expected,
		HasPendingAck: true,
		Tolerance:     withinTol,
	}
	got := Decide(in)
	if got.Outcome != OutcomePending {
		t.Fatalf("expected OutcomePending, got %v", got.Outcome)
	}
	// On Pending, Committed should be Expected so next-tick sim has a sane base.
	if got.Committed != expected {
		t.Fatalf("expected Committed == Expected %v on Pending, got %v", expected, got.Committed)
	}
}

func TestDecide_Snap(t *testing.T) {
	t.Parallel()
	claimed := mgl32.Vec3{20, 64, 20}
	expected := mgl32.Vec3{10, 64, 10}
	in := Input{
		Claimed:       claimed,
		Expected:      expected,
		HasPendingAck: false,
		Tolerance:     withinTol,
	}
	got := Decide(in)
	if got.Outcome != OutcomeSnap {
		t.Fatalf("expected OutcomeSnap, got %v", got.Outcome)
	}
	if got.Committed != expected {
		t.Fatalf("expected Committed == Expected %v on Snap, got %v", expected, got.Committed)
	}
}

func TestDecide_BoundaryExactlyAtTolerance(t *testing.T) {
	t.Parallel()
	// Place Claimed exactly withinTol away from Expected along a single axis.
	expected := mgl32.Vec3{0, 64, 0}
	claimed := mgl32.Vec3{withinTol, 64, 0}
	in := Input{
		Claimed:       claimed,
		Expected:      expected,
		HasPendingAck: false,
		Tolerance:     withinTol,
	}
	got := Decide(in)
	// diff.Len() == withinTol exactly; the condition is <=, so this must Accept.
	if got.Outcome != OutcomeAccept {
		t.Fatalf("boundary: expected OutcomeAccept at exact tolerance, got %v", got.Outcome)
	}
	if got.Committed != claimed {
		t.Fatalf("boundary: expected Committed == Claimed %v, got %v", claimed, got.Committed)
	}
}
