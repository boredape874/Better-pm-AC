// Package reconcile implements the 3-branch sim→reconcile→commit decision used
// by the γ.2 Reconciliation phase. All logic is pure (no I/O, no shared state)
// so it can be unit-tested and benchmarked in isolation.
package reconcile

import "github.com/go-gl/mathgl/mgl32"

// Outcome classifies a reconcile decision.
type Outcome int

const (
	OutcomeAccept  Outcome = iota // client pos within tolerance → commit as-is
	OutcomePending                // client pos outside but awaiting ack → hold
	OutcomeSnap                   // client pos invalid, no pending ack → snap to expected
)

// Input holds everything needed to make a reconcile decision.
type Input struct {
	Claimed       mgl32.Vec3 // unvalidated client-claimed position
	Expected      mgl32.Vec3 // sim-computed expected position
	HasPendingAck bool       // true if there is a pending ack action for this tick
	Tolerance     float32    // accept threshold (Euclidean, metres)
}

// Result is the reconciler's verdict.
type Result struct {
	Outcome   Outcome
	Committed mgl32.Vec3 // position to commit (= Claimed on Accept, = Expected on Snap/Pending)
}

// Decide runs the 3-branch reconcile logic.
// It is pure: no I/O, no shared state, deterministic.
func Decide(in Input) Result {
	diff := in.Claimed.Sub(in.Expected)
	if diff.Len() <= in.Tolerance {
		return Result{Outcome: OutcomeAccept, Committed: in.Claimed}
	}
	if in.HasPendingAck {
		return Result{Outcome: OutcomePending, Committed: in.Expected}
	}
	return Result{Outcome: OutcomeSnap, Committed: in.Expected}
}
