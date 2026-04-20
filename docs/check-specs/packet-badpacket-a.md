# packet-badpacket-a

## Summary

BadPacket/A flags invalid simulation-frame transitions on incoming
PlayerAuthInput packets. Vanilla clients always advance `Tick`
monotonically and never reset it back to zero mid-session; three
distinct misbehaviours are caught:

1. **Tick reset** — previously non-zero, incoming zero.
2. **Tick regression** — incoming tick < previous.
3. **Tick jump** — incoming tick > previous + 200 (≈ 10 s at 20 TPS).

Any of these indicates either a packet injector that rebuilds its own
`SimulationFrame`, a replay/rewind attack, or a modified client that
intentionally desyncs its frame counter to confuse physics checks.

## Inputs

- `data.Player.SimFrame()` — previous tick (UpdateTick has not yet
  been called with the incoming packet).
- `tick uint64` — `PlayerAuthInput.Tick` of the incoming packet.

## Algorithm

```
if !Enabled: return pass
prev = SimFrame()
if prev != 0 && tick == 0:             return flag("tick_reset")
if prev != 0 && tick < prev:           return flag("tick_regression prev=... new=...")
if prev != 0 && tick > prev + 200:     return flag("tick_jump prev=... new=... diff=...")
```

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 1     | Single-shot: any malformed transition is definitive |
| MaxBuffer     | 1     | Matches FailBuffer |
| MaxViolations | config.Violations (default 1) | Kick threshold |
| Jump window   | 200 ticks (10 s) | Covers the largest legitimate loading stutter |

## Policy

Default `Kick`. Framework-level packet corruption is unambiguous.

## False-positive risk — **Very low**

- Loading-screen stutters can produce skipped ticks but never past the
  200-tick window (which is 10 s).
- The `prev != 0` guard exempts the very first packet of a session.
- Reconnect paths create a new Player, not a rewound tick counter.

## Test vectors

See `anticheat/checks/packet/badpacket_test.go`:

- `TestBadPacketAFirstPacketGraceDoesNotFlag` — prev=0 branch exempt.
- `TestBadPacketAMonotonicDoesNotFlag` — tick=prev+1 passes.
- `TestBadPacketATickResetFlags` — prev=50, tick=0 → flag `tick_reset`.
- `TestBadPacketATickRegressionFlags` — prev=50, tick=20 → flag
  `tick_regression`.
- `TestBadPacketATickJumpFlags` — prev=100, tick=400 → flag `tick_jump`.
- `TestBadPacketAPolicyContract` — policy Kick.
