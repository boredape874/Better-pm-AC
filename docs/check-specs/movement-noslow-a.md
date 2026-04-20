# movement-noslow-a

## Summary

NoSlow/A flags horizontal motion that exceeds the item-use speed cap
(default 0.21 b/tick) while the player has the `usingItem` flag set.
Vanilla Bedrock slows the player to ~27% of base walking speed during
eating, bow-drawing, or shield-raising; cheats that bypass the
slowdown let the player sprint or walk normally mid-action. The check
sits idle for any tick without an active item use.

## Inputs

- `data.Player.InputSnapshotFull()` → `(_, _, inWater, _, usingItem)`.
- `data.Player.HorizontalSpeed()`.
- `data.Player.IsCreative()`, `HasKnockbackGrace()` — exempt states.
- `config.NoSlowConfig.MaxItemUseSpeed` (default 0.21).

## Algorithm

```
if !Enabled: return pass
if !usingItem: return pass
if IsCreative or HasKnockbackGrace: return pass
if inWater: return pass
if HorizontalSpeed() > MaxItemUseSpeed:
    return flag("speed=... max=...")
```

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 3     | Absorb the first accel frame before the slowdown takes effect client-side |
| MaxBuffer     | 4     | Brief overruns do not pin the buffer |
| MaxViolations | from config.Violations (default 8) | Kick threshold |
| Cap           | 0.21 b/tick | Observed vanilla item-use max |

## Policy

Default `Kick`. NoSlow is a clean binary — either the slowdown is
applied or it isn't. Repeatable and punishable.

## False-positive risk — **Low**

- Creative — exempt.
- Knockback grace — exempt (server-applied velocity overrides item-use
  slow on the impact tick).
- Swimming — exempt (underwater speed physics diverge from base).
- First-acceleration-frame jitter absorbed by FailBuffer=3.

## Test vectors

See `anticheat/checks/movement/noslow_test.go`:

1. `TestNoSlowALegalItemUseDoesNotFlag` — 0.18 b/tick under cap passes.
2. `TestNoSlowACheatFlags` — 0.50 b/tick → `speed=/max=` in info.
3. `TestNoSlowABoundaryJustUnder` — 0.205 passes, 0.215 flags.
4. `TestNoSlowANotUsingItemSkips` — 5 b/tick without usingItem → pass.
5. `TestNoSlowAPolicyContract` — policy Kick.
