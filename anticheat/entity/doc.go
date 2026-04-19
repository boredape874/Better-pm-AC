// Package entity implements meta.EntityRewind. A ring buffer stores each
// tracked entity's per-tick pose so combat checks can validate a swing
// against the target's position at the attacker's tick, compensating for
// client latency without granting reach advantage.
//
// Owner: AI-E. See docs/plans/2026-04-19-anticheat-overhaul-design.md §5.3.
package entity
