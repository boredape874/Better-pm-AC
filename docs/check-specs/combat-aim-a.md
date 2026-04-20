# combat-aim-a

## Summary

Aim/A flags mouse-clients whose per-tick yaw delta rounds to the
same value at 1-decimal and 5-decimal precision — i.e. a yaw
rotation that is suspiciously "round" (exactly 2.0000°, 5.0000°, …).
Aim-assist software frequently emits deltas that fall on perfectly
round fractions of a degree; human mouse movement nearly never does.

Applies only to `InputMode == Mouse`. Touch and gamepad clients
legitimately produce quantised deltas (snap-look / controller
deadzone) that would false-positive a round-value check.

## Inputs

- `data.Player.GetInputMode()` — must equal 1 (Mouse).
- `data.Player.RotationSnapshot()` — `(yawDelta, pitchDelta)` abs.

## Algorithm

```
if !Enabled: return pass
if InputMode != Mouse: return pass
yawDelta = |RotationDelta.X|
if yawDelta < 1e-3: return pass (camera idle)
r1   = round(yawDelta, 1)       # 1 decimal
r2   = round(yawDelta, 5)       # 5 decimals
diff = |r2 - r1|
if diff <= 3e-5:
    return flag("yaw_delta=... r1=... diff=...")
```

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 5     | Humans occasionally produce round deltas by chance |
| MaxBuffer     | 5     | Tight cap |
| MaxViolations | config.Violations (default 10) | Kick threshold |
| Round match   | ≤ 3e-5 | Floating-point tolerance |
| Idle gate     | yawDelta < 1e-3 → skip | Ignore stationary periods |

## Policy

Default `Kick`. Sustained detection across 5 ticks is a very
strong aim-assist signal.

## False-positive risk — **Medium**

- Mouse-only restriction eliminates touch/gamepad quantisation.
- 1e-3 idle gate prevents accumulating towards a false positive
  when the camera is stationary.
- A human player can produce occasional round deltas; FailBuffer=5
  requires five confirmations.

## Test vectors

See `anticheat/checks/combat/aim_test.go`:

1. `TestAimANonMouseSkips` — InputMode!=Mouse → pass.
2. `TestAimARoundYawDeltaFlags` — delta=2.0 (mouse) → flag with
   `yaw_delta=/r1=/diff=` in info.
3. `TestAimANaturalYawDeltaDoesNotFlag` — delta=2.34567 → pass.
4. `TestAimAIdleYawSkips` — delta<1e-3 → pass.
5. `TestAimAPolicyContract` — policy Kick.
