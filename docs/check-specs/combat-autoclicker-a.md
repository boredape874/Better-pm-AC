# combat-autoclicker-a

## Summary

AutoClicker/A flags players whose clicks-per-second exceed the
configured cap (default 16 CPS). CPS is measured over a rolling
1-second window: every `RecordClick` appends `time.Now()` and prunes
entries older than `cpsWindow`.

Human click rates above ~14 CPS are very rare outside of specialised
hardware; 16 gives ~2 CPS of head-room and matches Oomph's
AutoclickerA default.

## Inputs

- `data.Player.CPS()` — count of clicks in the last 1 s.
- `config.AutoClickerConfig.MaxCPS` — cap (default 16).

## Algorithm

```
if !Enabled: return pass
if CPS() > MaxCPS:
    return flag("cps=... max=...")
```

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 4     | Avoid flagging on a single 1-second burst |
| MaxBuffer     | 4     | Tight cap |
| MaxViolations | config.Violations (default 10) | Kick threshold |
| TrustDuration | 30 s (600 ticks) | Decay transient burst patterns |
| MaxCPS        | 16    | Oomph default; ~2 CPS above human peak |

## Policy

Default `Kick`. AutoClicker is a direct combat advantage; sustained
high CPS across the buffer threshold is unambiguous.

## False-positive risk — **Low-Medium**

- Butterfly/drag-click techniques can reach 20–30 CPS briefly; the
  FailBuffer of 4 means the player must sustain this across four
  distinct tick evaluations before a violation is recorded.
- The cap itself (16) is conservative; legitimate players rarely
  sustain above it.

## Test vectors

See `anticheat/checks/combat/autoclicker_test.go`:

1. `TestAutoClickerALegalCPSDoesNotFlag` — 10 clicks in window → pass.
2. `TestAutoClickerACheatFlags` — 25 clicks in window → flag with
   `cps=/max=` in info.
3. `TestAutoClickerABoundary` — 16 clicks passes, 17 flags.
4. `TestAutoClickerADisabledSkips` — `Enabled=false` short-circuits.
5. `TestAutoClickerAPolicyContract` — policy Kick.
