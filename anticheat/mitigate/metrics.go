package mitigate

import (
	"expvar"
	"sync/atomic"
)

// Metrics records aggregate dispatcher activity for operator dashboards.
// We use stdlib expvar instead of pulling in a Prometheus client because
// the dependency footprint matters for a proxy that ships as a single
// binary; expvar publishes at /debug/vars when the standard net/http/pprof
// import is enabled, which is the typical operator setup anyway.
//
// Counters are atomic uint64 so concurrent dispatcher calls (one per
// player goroutine) do not need a lock. Per-check / per-policy
// breakdowns are NOT recorded here — they would require either a sync.Map
// keyed on check name (lock contention on hot paths) or a registration
// step at startup. For β we expose the totals; per-check breakdown is
// deferred to γ and folded into the Phase 5b.2 metrics task.
//
// Operators read these via expvar.Get("better_pm_ac_violations_total")
// etc., or via the auto-published /debug/vars endpoint.
type Metrics struct {
	violationsTotal       atomic.Uint64
	kicksTotal            atomic.Uint64
	rubberbandsTotal      atomic.Uint64
	filteredPacketsTotal  atomic.Uint64
	dryRunSuppressedTotal atomic.Uint64
}

// global is the package-level Metrics instance. The Dispatcher writes to
// it on every Apply call. Tests reset it via ResetForTesting.
var global Metrics

// init publishes the counters to expvar so /debug/vars surfaces them
// automatically. The "better_pm_ac_" prefix prevents clashes with other
// expvar-using libraries in the same process.
func init() {
	expvar.Publish("better_pm_ac_violations_total", expvar.Func(func() any {
		return global.violationsTotal.Load()
	}))
	expvar.Publish("better_pm_ac_kicks_total", expvar.Func(func() any {
		return global.kicksTotal.Load()
	}))
	expvar.Publish("better_pm_ac_rubberbands_total", expvar.Func(func() any {
		return global.rubberbandsTotal.Load()
	}))
	expvar.Publish("better_pm_ac_filtered_packets_total", expvar.Func(func() any {
		return global.filteredPacketsTotal.Load()
	}))
	expvar.Publish("better_pm_ac_dry_run_suppressed_total", expvar.Func(func() any {
		return global.dryRunSuppressedTotal.Load()
	}))
}

// Snapshot returns a point-in-time copy of the counters. Useful for tests
// and ad-hoc operator queries that want a consistent read across all
// counters (expvar.Func reads are NOT atomic across multiple variables).
func Snapshot() (violations, kicks, rubberbands, filtered, dryRun uint64) {
	return global.violationsTotal.Load(),
		global.kicksTotal.Load(),
		global.rubberbandsTotal.Load(),
		global.filteredPacketsTotal.Load(),
		global.dryRunSuppressedTotal.Load()
}

// ResetForTesting zeroes all counters. NOT safe for production use; the
// suffix marks it as test-only. Test files that exercise dispatcher
// paths call this in setup so per-test counts are independent.
func ResetForTesting() {
	global.violationsTotal.Store(0)
	global.kicksTotal.Store(0)
	global.rubberbandsTotal.Store(0)
	global.filteredPacketsTotal.Store(0)
	global.dryRunSuppressedTotal.Store(0)
}
