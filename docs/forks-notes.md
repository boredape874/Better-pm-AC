---
doc: forks-notes
owner: AI-O
phase: 1
task: 1.3
status: draft
date: 2026-04-19
---

# Oomph Forks — Adoptable Improvements

Survey of three public Oomph forks for patterns worth porting into Better-pm-AC. Each entry flags: **Adopt** (import the pattern), **Reference** (read but don't copy verbatim), or **Skip** (not relevant).

Upstream baseline: [`oomph-ac/oomph@master`](https://github.com/oomph-ac/oomph).

## 1. aerisnetwork/oomph

- Master branch is ~18 commits ahead of upstream.
- Bulk of the diff is version bumps (Bedrock 1.21.120 → 1.21.130, Go 1.26) and syntax modernization.
- No genuinely new anti-cheat detections.

### Notable patterns

| Pattern | Where | Verdict | Notes |
|---|---|---|---|
| State-gated check dispatch (`if p.State.Get() != player.StatePlaying { return }`) | `player/simulation/movement.go` | **Adopt** | Matches our "grace window" concept but cleaner. Apply to every check's entry in our future `anticheat/meta/Detection.Run`. |
| `monitoring_state.go` separate state machine | `player/monitoring_state.go` (new) | **Reference** | We already have grace flags in `data/player.go`; lift the enum pattern if it keeps state transitions explicit. |
| Per-player protocol chunk encoder | `world/chunk_source.go` | **Skip** | Multi-protocol-version support; our proxy pins one Bedrock version per config so it's not needed at β. |
| Deletion of `example/spectrum/*` | ‑ | **Skip** | Spectrum-PM integration does not apply; we target PMMP via direct MiTM. |

## 2. TrixNEW/oomph

Branches: `master`, `transfer-movement-sync`, `wall-collissions`.

### `wall-collissions` (PR #120) — **Adopt**

Direct quote from PR: *"dragonfly's wall model collapses short/tall connections into a single simplified box"*, causing incorrect collision heights. Fix uses fallback collision states to keep real wall heights.

Implication for us: **critical**. Our WorldTracker is Dragonfly-based; if we use Dragonfly's `block.Wall.Model()` naively we inherit the same bug and `NoClip/Phase` becomes unreliable near cobblestone walls / fences with mixed neighbors.

- AI-W must override wall BBox resolution with a per-neighbor fallback (short vs tall post) rather than trusting Dragonfly's simplified model.
- Add to design §5.1 (WorldTracker) as a known quirk.

### `transfer-movement-sync` — **Reference**

Branch name suggests sync of movement simulation across a server transfer boundary. Our proxy treats each backend session as fresh, so relevance is low; revisit if we add multi-backend transfer support.

## 3. basicengine92-tech/oomph

Richest fork — many feature branches. Covered here: the ones whose names map directly onto our §6 check catalog.

### `refactor/movement-sim` (PR #112, 5 commits, 19 files) — **Adopt**

- Extracts movement simulation into a reusable library module.
- Optimizes `BlockCollisions` and `GetNearbyBBoxes` (hot path for every tick).
- Adds new BBox methods for special cases; includes a `WoodFenceGate` fix.

Impact on us: validates our §4 design — a standalone `sim` package is the right shape. Port the collision-optimization ideas into AI-S's `sim.Step` implementation.

### `feat/lag-comp-cutoff` (1 commit, 10 files) — **Adopt**

Adds **configurable latency cutoffs** for entity rewind. Players above the cutoff don't benefit from rewind (combat checks fall back to a stricter no-rewind mode). Prevents high-ping players from abusing rewind windows.

Add to config schema (§7): `[entity_rewind] max_rewind_ticks`, `cutoff_policy = "strict" | "kick" | "nocheck"`. AI-E must expose this in the `EntityRewind` interface.

### `feat/scaffold-dtc` (36 commits, 5 files) — **Adopt**

"DTC" = **Don't-Trust-Client**. 36 commits implies sustained iteration — likely the most mature scaffold/tower check in the Oomph ecosystem. Our §6 catalog has `Scaffold/A` as a new check; this fork is the reference implementation to study line-by-line before AI-C writes ours.

### Other branches worth a lookback

| Branch | Maps to our check | Priority |
|---|---|---|
| `feat/subchunk-cache` | WorldTracker perf | P1 (100 CCU target) |
| `fix/reach-on-still-entities` | `Reach/A` (rewind-aware) | P0 |
| `fix/invisible-blocks` | `NoClip/Phase`, `Scaffold/A` | P0 |
| `fix/chained-block-placements` | `Scaffold/A` burst handling | P0 |
| `feat/proxy-flags-and-punishmments` | `mitigate` policy layer | P1 |
| `feat/liquid-movement` | `Velocity/A` water exemption (we already have) | P2 (cross-check) |
| `feat/collisions` | BBox/NoClip refinements | P1 |
| `refactor/performance-2` | 100 CCU perf budget | P1 |
| `chore/crafting-handling` | Out of scope | Skip |
| `feat/claude` | Unknown (likely AI-assisted) | Skip |

## Recommendations

1. **Before AI-S writes `sim.Step`**, read `basicengine92-tech:refactor/movement-sim` end-to-end. It is ahead of upstream and matches our architecture.
2. **Before AI-W writes the wall BBox resolver**, apply the `TrixNEW:wall-collissions` fallback pattern — treat this as a required fix, not optional.
3. **Entity rewind cutoff** (basicengine92's `feat/lag-comp-cutoff`) should land in Phase 2 (AI-E), not deferred. Config keys ship in Task 1.8.
4. **Scaffold/A** implementation (Phase 3, AI-C) MUST reference `basicengine92:feat/scaffold-dtc` — 36 iterations of refinement there.
5. Keep `aerisnetwork`'s **state-gated dispatch** pattern in mind when AI-O extends the `Detection` interface in Task 1.5 — easier to design in up front than retrofit.

## Limitations of this survey

- GitHub compare pages frequently timed out on the larger diffs; exact physics-constant changes were not extracted. AI-S should re-check against fork source when implementing simulation.
- `feat/scaffold-dtc` had 36 commits that were not individually reviewed; defer deep-dive until Phase 3 begins.
- `transfer-movement-sync` and several `feat/*` branches remain not deeply investigated. Re-open this file if we hit blockers in the matching subsystem.
