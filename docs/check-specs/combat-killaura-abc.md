# combat-killaura-abc

Combined spec for the three KillAura subchecks. All three share the
default Kick policy and are triggered by RecordAttack; they differ in
which KillAura signature they detect.

## KillAura/A — swing-less attacks

Flags attacks that arrive more than 10 ticks (plus one-way ping, capped
at 10 ticks) after the last recorded arm swing. Vanilla clients send a
`packet.Animate` / `InputFlagMissedSwing` within the tick of the attack;
KillAura bots that synthesise attack packets without animation drift
past this window.

- Grace on first attack: `lastSwing == 0` short-circuits to pass so a
  player's first combat interaction isn't penalised.
- Tick-wrap safety: `currentTick < lastSwing` also short-circuits.

## KillAura/B — attack outside field of view

Flags attacks whose eye→target direction falls more than 90° off the
player's current look vector (plus a ping-derived angle tolerance, capped
at 15°). No legitimate click reaches a target behind the player.

- Target is server-authoritative feet position.
- Player eye = feet + 1.62 blocks.
- Look direction derived from (yaw, pitch) using the standard Bedrock
  convention.

## KillAura/C — multi-target in one tick

Flags when `RecordAttack` observes more than one distinct attack event
in the same `SimulationFrame`. Vanilla Bedrock combat is single-target
per tick; multi-target hits are only possible via injected attack packets.

## Inputs

- /A: `SimFrame()`, `SwingTick()`, `Latency()`.
- /B: `CurrentPosition()`, `RotationAbsolute()`, `Latency()`, `targetPos`.
- /C: `AttackTickCount()` after `RecordAttack`.

## Thresholds

| Field         | /A | /B | /C | Rationale |
|---------------|----|----|----|-----------|
| FailBuffer    | 1  | 3  | 1  | /A,/C are definitive; /B buffers lag-induced drift |
| MaxBuffer     | 1  | 5  | 1  | Matches FailBuffer for /A /C; small head-room for /B |
| MaxViolations | config.Violations | config.Violations | config.Violations | Kick threshold |
| Window /A     | 10 ticks + ping (capped 10) | — | — | 500 ms covers worst case |
| Angle /B      | 90° + ping (capped 15°) | — | — | Any entity past 90° is behind/off-axis |

## Policy

All three default to `Kick`. KillAura is among the highest-impact PvP
cheats; confirmed signatures warrant disconnection.

## False-positive risk — **Low** (A, C), **Medium** (B)

- /A: swing-less attack is a clean binary signal; ping-widened window
  covers high-latency legitimate clients.
- /B: fast camera movement during attack can transiently place the
  target off-axis until the next rotation update arrives. FailBuffer=3
  and ping-compensation mitigate.
- /C: single-tick multi-target has no legitimate vanilla source.

## Test vectors

See `anticheat/checks/combat/killaura_test.go`:

- /A: grace-on-first-attack, swing-within-window passes, silent-attack flags,
  tick-wrap safety, policy contract.
- /B: direct-line-of-sight passes, behind-target flags with `angle=/max=`,
  policy contract.
- /C: single-target pass, two-in-one-tick flag with `targets_per_tick=`,
  multi-tick single-target still passes, policy contract.
