# movement-nofall-a

## Summary

NoFall/A flags players who transition from airborne to on-ground with an
accumulated `fallDistance > 3.0` blocks but fail to receive fall damage.
The check fires exactly once on the landing transition — `NoFallSnapshot`
returns `(justLanded=true, fallDistance=N)` on the tick where
`LastOnGround=false, OnGround=true` and then resets. Water-absorption
(`inWater` / `HasRecentWaterExit`) and Slow Falling potion negate fall
damage in vanilla and are exempted.

## Inputs

- `data.Player.NoFallSnapshot()` → `(justLanded bool, fallDistance float32)`.
- `InputSnapshotFull()` → `inWater` flag.
- `HasRecentWaterExit()` — 10-tick grace after a water→land transition.
- `EffectAmplifier(EffectSlowFalling)`.

## Algorithm

```
if !Enabled: return pass
justLanded, fallDist = NoFallSnapshot()
if !justLanded or fallDist <= 3.0: return pass
if inWater: return pass
if HasRecentWaterExit: return pass
if SlowFallingActive: return pass
return flag("fall_dist=... threshold=3.0")
```

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 1     | Landing is a one-shot event |
| MaxBuffer     | 2     | Prevent aggressive buffer pinning if a server lags and produces two landing frames |
| MaxViolations | from config.Violations (default 5) | Kick threshold |
| Damage cutoff | 3.0 blocks | Vanilla fall-damage threshold |

## Policy

Default `Kick`. NoFall is a deterministic exploit (the client either
took damage or didn't); once the damage-absorbing exemptions are
excluded there is no legitimate reason for `fallDistance > 3.0` to land
without damage.

## False-positive risk — **Low**

Known triggers (all handled):
- Water landing — `inWater` exempt.
- Exited water, landed within 10 ticks — `HasRecentWaterExit` exempt.
- Slow Falling — effect amplifier exempt.

Residual risk: MLG water-bucket saves rely on the bucket being placed
BEFORE the landing tick (flips the block under the player to water).
If the proxy observes the UpdateBlock before the landing PlayerAuthInput,
the terrain will be water by landing time — this is handled via the
inWater path once World tracker integration is complete (Phase 3 Scaffold
migration uses the same world lookup pattern).

## Test vectors

See `anticheat/checks/movement/nofall_test.go`:

1. `TestNoFallALegalShortFallDoesNotFlag` — 2-block fall passes.
2. `TestNoFallACheatFlags` — 10-block fall without effect/water flags
   with a `fall_dist=` info string.
3. `TestNoFallABoundaryExactlyAtThreshold` — exactly 3 blocks passes,
   3.5 blocks flags.
4. `TestNoFallAPolicyContract` — policy enum round-trips.
