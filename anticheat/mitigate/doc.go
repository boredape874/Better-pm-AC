// Package mitigate implements meta.MitigateDispatcher. It translates a
// Detection flag into the configured enforcement action: log-only, client
// rubberband teleport, server-side packet filter, or disconnect at
// MaxViolations.
//
// Owner: AI-M. See docs/plans/2026-04-19-anticheat-overhaul-design.md §5.5.
package mitigate
