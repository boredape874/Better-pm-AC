// Package sim implements meta.SimEngine. It replays Bedrock player physics
// (gravity, drag, jump, sprint, liquid, climb, glide, etc.) server-side so
// movement checks can diff the client's reported position against the
// authoritative "expected" position.
//
// Owner: AI-S. See docs/plans/2026-04-19-anticheat-overhaul-design.md §5.2.
package sim
