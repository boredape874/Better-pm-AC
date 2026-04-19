// Package world implements meta.WorldTracker. It consumes LevelChunk and
// SubChunk packets to build a server-authoritative view of the Bedrock world
// so anti-cheat checks can query block state and collision BBoxes without
// trusting the client.
//
// Owner: AI-W. See docs/plans/2026-04-19-anticheat-overhaul-design.md §5.1.
package world
