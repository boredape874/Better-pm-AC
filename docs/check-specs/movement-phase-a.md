# movement-phase-a

## Summary

Phase/A flags any single-tick 3D displacement above 6 blocks that is not
attributed to a server-issued teleport. The limit sits roughly 1.5× above
the worst-case vanilla 3D velocity (sprint-jump horizontal ≈ 0.91 b/tick
combined with terminal-velocity fall ≈ 3.9 b/tick ≈ 4.0 b/tick 3D), so
only frank teleport/phase cheats cross it. The check is stateless: one
delta exceeding the cap → flag.

## Inputs

- `data.Player.PositionDelta()` — per-tick Vec3 displacement.
- `teleportGrace` argument (set by Manager when
  `InputFlagHandledTeleport` was observed this tick).
- `IsCreative()` / `IsGliding()` — exempt states.

## Algorithm

```
if !Enabled or IsCreative or IsGliding or teleportGrace: return pass
dist = |PositionDelta()|
if dist > 6.0: return flag("delta=... max=6.0")
```

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 1     | A single impossible jump is unambiguous |
| MaxBuffer     | 2     | Consecutive flagged frames still short-circuit to kick |
| MaxViolations | from config.Violations (default 3) | Aggressive threshold — 3 is enough |
| Distance cap  | 6.0 blocks | 1.5× vanilla worst case |

## Policy

Default `Kick`. Phase is a deterministic position exploit — a single
confirmed impossible delta is a kick-worthy violation. ServerFilter
could theoretically drop the packet but the player remains in a cheated
state client-side; Kick is the clean fix.

## False-positive risk — **Low**

- Creative / gliding exempt.
- Server teleport exempt via teleportGrace.
- 6-block cap is ~1.5× worst vanilla 3D velocity; no legitimate physics
  crosses it.

Residual risk: firework-boosted elytra with a transient network burst
could conceivably produce a 7-block frame. Exempt via IsGliding.

## Test vectors

See `anticheat/checks/movement/phase_test.go`:

1. `TestPhaseALegalSprintJumpDoesNotFlag` — realistic sprint-jump passes.
2. `TestPhaseATeleportCheatFlags` — 10-block delta flags with the
   `delta=` info string.
3. `TestPhaseABoundaryWithinLimit` — 5-block diagonal passes, 7-block
   flags.
4. `TestPhaseATeleportGraceSkipsCheck` — 100-block delta under grace is
   ignored.
5. `TestPhaseAPolicyContract` — policy Kick.
