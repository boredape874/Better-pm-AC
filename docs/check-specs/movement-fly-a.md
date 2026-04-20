# movement-fly-a

## Summary

Fly/A flags two distinct airborne anomalies: (1) **hover** — sustained
near-zero per-tick Y displacement after the jump-arc grace period; and
(2) **upward_fly** — a positive Y displacement above the hover threshold
after the same grace. Both branches rely on per-tick counters maintained
inside `data.Player.UpdatePosition`: `AirTicks` counts consecutive
airborne frames and `HoverTicks` counts consecutive near-zero Y deltas.
The grace window is extended by JumpBoost amplifier × 5 ticks so boosted
arcs are not penalised.

## Inputs

- `data.Player.FlySnapshot()` → `(airborne, yDelta, airTicks, hoverTicks, _)`.
- `data.Player.IsCreative()`, `IsGliding()`, `HasKnockbackGrace()`,
  `HasRecentWaterExit()`, `HasTerrainCollision()`, `InputSnapshotFull()`
  for exemptions.
- `EffectAmplifier(EffectSlowFalling | EffectLevitation | EffectJumpBoost)`
  for potion-aware adjustments.

## Algorithm

```
if !Enabled or IsCreative or IsGliding or SlowFallActive or LevitationActive
   or HasKnockbackGrace or HasRecentWaterExit:
    return pass
if !airborne: return pass
if inWater or crawling: return pass
if HasTerrainCollision: return pass
grace = 8 + (JumpBoostAmp + 1) * 5  (if JumpBoost active)
if airTicks <= grace: return pass
if yDelta > 0.005:
    return flag("upward_fly ...")
if hoverTicks >= 3:
    return flag("hover ...")
```

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 3     | Absorb one-tick jitter near grace expiry |
| MaxBuffer     | 5     | Short hover bursts do not pin the buffer |
| MaxViolations | from config.Violations (default 5) | Kick threshold |
| Grace         | 8 ticks (+5 per JumpBoost level) | Vanilla arc duration |
| Hover tolerance | 0.005 b/tick | Shared with `hoverDeltaThreshold` in data.Player |
| Min hover     | 3 ticks | Rule out jump apex transient |

## Policy

Default `ServerFilter` in config — the proxy can pin the player back to
the last valid Y and let the server continue unaware. Falls back to
`Kick` when no server-filter hook is wired. Every detection is
repeatable and robust enough that rubberband / kick are both defensible
responses; the config picks per deployment.

## False-positive risk — **Medium**

Known triggers (all handled):
- Slow Falling, Levitation, Jump Boost — potion exemptions above.
- Elytra gliding — `IsGliding()` exempt.
- Water exit grace — `HasRecentWaterExit()`.
- Knockback — `HasKnockbackGrace()`.
- Swimming / crawling — `InputSnapshotFull()` branch.
- Ladder / vine / wall contact — `HasTerrainCollision()`.

Residual risk: cobweb, scaffolding, honey-block slides not yet modeled
→ Phase 4 γ physics extensions.

## Test vectors

See `anticheat/checks/movement/fly_test.go`:

1. `TestFlyALegalJumpArcDoesNotFlag` — 6-tick vanilla jump arc passes.
2. `TestFlyAHoverFlags` — 20 ticks at yDelta ≈ 0 → "hover" branch fires.
3. `TestFlyAUpwardFlyFlags` — 12 ticks of positive yDelta → "upward_fly"
   branch fires.
4. `TestFlyACreativeExempt` — gamemode 1 never flags.
5. `TestFlyAPolicyContract` — policy enum round-trips.
