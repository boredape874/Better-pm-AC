# packet-badpacket-bcd

Combined spec for three tightly-related BadPacket subchecks that share
the same `FailBuffer=1, MaxBuffer=1` single-shot policy and the same
default Kick. Each detects a packet-shape anomaly that no vanilla client
produces, so one confirmed instance is enough to flag.

## BadPacket/B — pitch range

Flags PlayerAuthInput packets where `Pitch ∉ [-90, 90]`. Vanilla clamps
to this range; anything outside is a modified client. Bounds are
**inclusive**; `±90` passes, `±90.0001` flags.

## BadPacket/C — sprint + sneak

Flags packets where both Sprint and Sneak input bits are set in the
same tick. Vanilla clears sprint on sneak-enter and vice versa, so
both-set is physically impossible. The info string is literally
`"sprint+sneak"` — no numeric payload since the signal is binary.

## BadPacket/D — NaN / Inf position

Flags PlayerAuthInput positions with any NaN or Infinite component on
any axis. Runs before UpdatePosition so corrupt coordinates never
reach the state machine (would otherwise propagate NaN into every
downstream movement check).

## Inputs

- /B: `pitch float32` from `PlayerAuthInput`.
- /C: `data.Player.InputSnapshotFull()` — the per-tick input flags.
- /D: `pos mgl32.Vec3` from `PlayerAuthInput`.

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 1     | Single-shot: any confirmed anomaly is a kick offense |
| MaxBuffer     | 1     | Matches FailBuffer — no smoothing |
| MaxViolations | config.Violations (default 5) | Kick threshold |

## Policy

All three default to `Kick`. These are packet-shape anomalies (not
physics anomalies) — there is no legitimate vanilla source, so
ServerFilter / Rubberband make no sense. Disconnection is the clean
response; the player can rejoin with a working client.

## False-positive risk — **Very low**

- /B: all vanilla clients clamp pitch to [-90, 90]. No known legitimate
  source of out-of-range values.
- /C: sprint+sneak is impossible in vanilla input handling. No known
  legitimate source.
- /D: NaN/Inf never originates from a vanilla client; no known
  legitimate source.

## Test vectors

See `anticheat/checks/packet/badpacket_test.go`:

- `TestBadPacketBLegalPitchDoesNotFlag` — pitches in [-90, 90] pass.
- `TestBadPacketBOutOfRangeFlags` — pitches outside the range flag.
- `TestBadPacketBBoundary` — ±90 inclusive, ±90.0001 flags.
- `TestBadPacketCLegalFlags` — {no-flag, sprint-only, sneak-only} pass.
- `TestBadPacketCImpossibleFlagsFire` — sprint+sneak flags with
  "sprint+sneak" info.
- `TestBadPacketCDisabledSkips` — `Enabled=false` short-circuits.
- `TestBadPacketDLegalPositionDoesNotFlag` — finite coords pass.
- `TestBadPacketDNaNAndInfFlag` — per-axis NaN and ±Inf all flag with
  proper axis indices in info.
- Policy contract tests for each.
