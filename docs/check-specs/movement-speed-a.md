# movement-speed-a

## Summary

Speed/A flags horizontal ground-movement whose per-tick magnitude exceeds the
configured base `max_speed` (blocks/tick) after adjustment for the player's
current input state (sprint / sneak / crawl / item-use) and active Speed or
Slowness potion effects. It runs only while the player is on the ground and
outside knockback grace; aerial and recently-impulsed frames are deferred to
Fly/A and Velocity/A respectively. The check is positional-delta based — no
wall-clock math, matching Oomph's per-frame displacement comparison.

## Inputs

- `data.Player.HorizontalSpeed()` — XZ magnitude of the per-tick position delta
  in blocks/tick. Source: `Velocity = Position - LastPosition` from
  `UpdatePosition`.
- `data.Player.IsCreative()` / `HasKnockbackGrace()` / `IsOnGround()` /
  `IsJustLanded()` — early-out guards.
- `data.Player.InputSnapshotFull()` — per-frame sprint/sneak/crawl/usingItem
  flags set from PlayerAuthInput bitset.
- `data.Player.EffectAmplifier(packet.EffectSpeed | EffectSlowness)` —
  potion adjustments.
- Config `SpeedConfig.MaxSpeed` (default 0.20 b/tick walk).

## Algorithm

```
if !Enabled or IsCreative or HasKnockbackGrace or !IsOnGround or IsJustLanded:
    return pass
speed = HorizontalSpeed()
cap = MaxSpeed
switch first-match:
    usingItem     → cap *= 0.35
    crawling      → cap *= 0.25
    sprinting     → cap *= 1.30
    sneaking      → cap *= 0.30
if speedEffect.active:
    cap *= (1 + 0.20 * (amp + 1))
if slownessEffect.active:
    cap *= max(0, 1 - 0.15 * (amp + 1))
if speed > cap:
    flag(speed, cap)
```

## Thresholds

| Field          | Value | Rationale |
|----------------|-------|-----------|
| FailBuffer     | 2     | Two consecutive frames over the cap before flagging |
| MaxBuffer      | 4     | Caps buffer growth so a brief spike doesn't linger |
| MaxViolations  | from config.Violations (default 10) | Kick threshold |
| Pass() step    | 0.05  | Slow buffer decay so occasional bursts are tolerated |

Mitigate policy: **Kick** (default). Speed hacking is not a packet-replay
problem; rubberband alone is ineffective because the cheater rejoins and
resumes — disconnection is the correct response.

## Policy

`Kick`. Cheated horizontal speed is unambiguous once exemptions clear; it is
punishable and repeatable, so `Punishable() = true` and policy = Kick.

## False-positive risk — **Low**

Known triggers to watch:
- **Landing frames** — suppressed via `IsJustLanded()` grace.
- **Ice / blue ice** — not currently modeled; flagged β follow-up (Phase 4
  physics extensions may add an ice-momentum exemption).
- **Pushed by piston / mob** — covered by knockback grace since those paths
  generate SetActorMotion packets.
- **Lag spikes** — positional delta grows on recovery. Handled by the
  knockback grace window extending with RTT (see
  `player.RecordKnockback`).

## Test vectors

See `anticheat/checks/movement/speed_test.go`:

1. `TestSpeedALegalSprintDoesNotFlag` — sprint at 0.25 b/tick under 0.26
   sprint-adjusted cap passes.
2. `TestSpeedACheatFlags` — 0.50 b/tick walking under 0.20 cap flags.
3. `TestSpeedABoundarySneakExactlyAtLimit` — 0.059 under cap passes, 0.08
   over cap flags.
4. `TestSpeedAPolicyContract` — policy mapping round-trips for all four enum
   values plus invalid/empty → default Kick.
