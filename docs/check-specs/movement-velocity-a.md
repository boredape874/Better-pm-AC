# movement-velocity-a

## Summary

Velocity/A is an **Anti-KB** detector: it flags clients whose
per-tick position delta, after a server-applied horizontal impulse
(SetActorMotion or MotionPredictionHints), does not carry at least 15%
of the applied magnitude in the knockback direction. A legitimate
client, even after a ground-friction tick, will show projection ≥ 0.15
× appliedMag; an Anti-KB cheat suppresses the motion and projects to
near zero.

## Inputs

- `data.Player.PositionDelta()` — current-tick velocity Vec3.
- `kb mgl32.Vec2` — the horizontal XZ velocity captured from the
  latest SetActorMotion / MotionPredictionHints, supplied by the
  Manager via `KnockbackSnapshot()`.
- `IsCreative() / IsGliding()`, `InputSnapshotFull()` (inWater,
  crawling) — exempt states.

## Algorithm

```
if !Enabled: return pass
appliedMag = |kb|
if appliedMag < 0.1:         # below detection floor
    return pass
if IsCreative or IsGliding or inWater or crawling: return pass
projection = velXZ · normalize(kb)
minExpected = appliedMag * 0.15
if projection < minExpected:
    return flag("kb=... player_spd=... projection=... min=...")
```

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 2     | Avoid flagging on single-tick edge cases (network reorder) |
| MaxBuffer     | 3     | Tight cap |
| MaxViolations | from config.Violations (default 5) | Kick threshold |
| minKB         | 0.1 b/tick  | Detection floor — weaker impulses are noisy |
| minRatio      | 0.15 (15%)  | Conservative ratio that survives ground friction |

## Policy

Default `Kick`. Anti-KB is high-value cheat: ignoring it makes the
player effectively unknockable in combat. Once two consecutive
absorbed impulses are confirmed, disconnection is the right response.

## False-positive risk — **Medium**

- Creative / gliding exempt.
- Water / crawling exempt.
- Sub-threshold impulses ignored.
- Ground friction accounted for with the conservative 15% floor.

Residual risk: the first impulse-tick timing is tricky. If the
SetActorMotion packet is observed but the client hasn't yet applied it
to position (e.g. the response is pipelined on the next tick), a
transient zero-projection tick is possible. FailBuffer=2 mitigates.

## Test vectors

See `anticheat/checks/movement/velocity_test.go`:

1. `TestVelocityALegalAbsorptionDoesNotFlag` — 80% absorption → pass.
2. `TestVelocityACheatFlags` — zero absorption → flag with
   `projection=/min=` in info.
3. `TestVelocityABelowMinKBSkips` — sub-0.1 impulse ignored.
4. `TestVelocityABoundaryAtMinRatio` — 0.075 projection passes
   (`< min` is strict), 0.07 flags.
5. `TestVelocityAPolicyContract` — policy Kick.
