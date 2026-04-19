// Package ack implements meta.AckSystem. It issues NetworkStackLatency events
// as tick markers and resolves registered callbacks when the client echoes
// the timestamp back, giving checks a reliable client-side confirmation of
// when a server event was applied.
//
// Owner: AI-A. See docs/plans/2026-04-19-anticheat-overhaul-design.md §5.4.
package ack
