package reconcile

import (
	"expvar"
	"sync/atomic"
)

// reconcileMetrics holds atomic counters for reconcile outcomes.
// We mirror the mitigate.Metrics pattern: atomic uint64 fields published
// via expvar.Func so concurrent OnInput calls incur no lock contention.
type reconcileMetrics struct {
	accepts  atomic.Uint64
	pendings atomic.Uint64
	snaps    atomic.Uint64
}

// globalReconcile is the package-level counter set incremented by anticheat.OnInput.
var globalReconcile reconcileMetrics

func init() {
	expvar.Publish("anticheat_reconcile_accepts", expvar.Func(func() any {
		return globalReconcile.accepts.Load()
	}))
	expvar.Publish("anticheat_reconcile_pendings", expvar.Func(func() any {
		return globalReconcile.pendings.Load()
	}))
	expvar.Publish("anticheat_reconcile_snaps", expvar.Func(func() any {
		return globalReconcile.snaps.Load()
	}))
}

// IncAccept increments the OutcomeAccept counter.
func IncAccept() { globalReconcile.accepts.Add(1) }

// IncPending increments the OutcomePending counter.
func IncPending() { globalReconcile.pendings.Add(1) }

// IncSnap increments the OutcomeSnap counter.
func IncSnap() { globalReconcile.snaps.Add(1) }

// ReconcileSnapshot returns a point-in-time copy of all three counters.
func ReconcileSnapshot() (accepts, pendings, snaps uint64) {
	return globalReconcile.accepts.Load(),
		globalReconcile.pendings.Load(),
		globalReconcile.snaps.Load()
}
