# packet-badpacket-e

## Summary

BadPacket/E flags PlayerAuthInput packets that carry contradictory
start+stop flag pairs in the same tick. A legitimate client produces
these as discrete, mutually-exclusive transitions — you either begin
sprinting or you stop, never both in one packet. The combinations
checked are:

- Start + Stop Sprinting
- Start + Stop Sneaking
- Start + Stop Swimming
- Start + Stop Gliding
- Start + Stop Crawling

Any contradiction indicates a packet injector, a modified client
deliberately emitting impossible state transitions, or a bot framework
that does not manage input flags correctly.

## Inputs

- `input interface { Load(bit int) bool }` — the PlayerAuthInput
  `InputData` bitset (protocol.Bitset in production).

## Algorithm

```
if !Enabled: return pass
violations = []
for pair in contradictoryPairs:
    if input.Load(pair.start) and input.Load(pair.stop):
        violations.append(pair.name)
if violations:
    return flag(join(violations, ","))
```

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 1     | A contradictory flag set is definitive |
| MaxBuffer     | 1     | Matches FailBuffer |
| MaxViolations | config.Violations (default 1) | Kick threshold |

## Policy

Default `Kick`. The packet shape is impossible in vanilla; there is
no legitimate client source.

## False-positive risk — **Very low**

- All five pairs are strict mutual transitions in the vanilla state
  machine.
- The info string lists every offending pair, aiding triage.

## Test vectors

See `anticheat/checks/packet/badpacket_test.go`:

- `TestBadPacketELegalFlagsDoesNotFlag` — no contradictions pass.
- `TestBadPacketESprintPairFlags` — start+stop sprint → flag with
  `start+stop_sprint` in info.
- `TestBadPacketEMultiplePairsInOneInfo` — sprint+sneak simultaneous
  contradictions join with comma.
- `TestBadPacketEDisabledSkips` — `Enabled=false` short-circuits.
- `TestBadPacketEPolicyContract` — policy Kick.
