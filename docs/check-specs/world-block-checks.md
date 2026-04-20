# world-block-checks (Phase 3 / AI-C-4 γ)

Combined spec for the **block-interaction** checks that depend on the
world tracker (`anticheat/world/Tracker`) and the per-player entity
rewind (`anticheat/entity/Rewind`). These all enter the codebase as
new implementations under `anticheat/checks/world/` once the tracker
is attached to `Manager` (Task 5a.1 sim/world wiring).

The world tracker stores authoritative block state from
`UpdateBlock`, `LevelChunk`, and `SubChunk` packets. Block-side checks
consult it instead of trusting the client-supplied `ClickedPosition`,
which is the central reason these are γ-scope: β builds use
client-reported positions and would false-positive on world drift.

Common policy: all default to `Kick`. Players that interact with
blocks the server has not authorized are unambiguously cheating.

## Scaffold/A — non-targetable face placement

Flags `UseItem (ClickBlock)` transactions where the clicked face on
the base block points away from the player's eye line, or the base
block does not exist in the world tracker at all (Scaffold software
fakes a base block).

- Inputs: `blockPos mgl32.Vec3`, `face int32`, world tracker lookup,
  player eye position, look direction.
- Detection floor: only flag when `dot(faceNormal, lookVec) > 0`
  (face is angled towards the player). Otherwise the click is
  geometrically impossible.
- FailBuffer 3 / MaxBuffer 5 — Oomph default; absorbs latency.

## Nuker/A — multi-block break burst

Flags more than N break events within W ticks (default N=3, W=5).
Vanilla mining is rate-limited by tool speed and BlockBreakProgress;
breaking multiple full blocks in one tick is impossible.

- Inputs: ring buffer of recent BreakBlock timestamps in `data.Player`.
- FailBuffer 1 — single confirmed multi-break is definitive.

## Nuker/B — wide-radius break

Flags break events beyond the configured reach radius (default 6 b)
from the player's eye position. The world tracker confirms the block
exists at the claimed location before measuring distance.

## FastBreak/A — sub-vanilla break time

Flags `LevelEvent.LevelEventBlockUpdate` (block-broken) events whose
elapsed time since the matching `PlayerActionStartBreak` is less than
the vanilla break time computed from block hardness × tool multiplier
× efficiency level (Sharpness/Efficiency tables in `world/blocks.go`).

- Tolerance: 25% under-time before flagging (covers ping jitter and
  Haste II buffs).
- FailBuffer 3.

## FastPlace/A — sub-vanilla place rate

Flags placement events at > 8 placements per second (default cap).
The vanilla cap is 5 placements/s under perfect conditions; 8 leaves
head-room for batched-network-tick edge cases.

## Tower/A — vertical-tower jump-place

Flags repeated `place-block-below + jump` patterns at faster than
vanilla allows. Specifically: place + immediate upward velocity +
new place at +1 Y within ≤ 4 ticks. Requires sim engine to confirm
the upward velocity could not be naturally produced.

- Depends on Phase 2 sim engine output (Task 5a.1 sim wiring).
- FailBuffer 4.

## InvalidBreak/A — break under invalid conditions

Flags `BreakBlock` actions while the player is in a state that should
prevent breaking: dead, in spectator, in a vehicle, or holding an
inventory-locked item. Cross-references `data.Player` exemption
flags + world tracker "block exists at position".

## Acceptance gate

Each check requires:
1. Spec section above (this doc) finalized.
2. Implementation under `anticheat/checks/world/<name>.go`.
3. Unit tests exercising legal/cheat/boundary + Policy contract.
4. Manager registration in `anticheat/anticheat.go` checks slice.
5. World-tracker dependency wired through `Manager.world`.

## Open questions

- **Trapdoor/door open vs. break:** vanilla treats opening as a break
  event for `Punch` packets; need to filter out client-side toggles.
- **Frost-walker / soul-speed block updates:** these emit BlockUpdate
  packets that look like fast-place; whitelist the source block IDs.
