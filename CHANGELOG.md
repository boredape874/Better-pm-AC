# Changelog

All notable changes to Better-pm-AC are documented here. The project
follows [Semantic Versioning](https://semver.org/) once v1.0.0 ships;
pre-1.0 releases use `0.MINOR.PATCH` with breaking changes allowed at
MINOR bumps. See `docs/plans/2026-04-19-work-board.md` for the live
roadmap.

## [v0.10.0-beta] — 2026-04-21

First β release of the "Oomph-parity" overhaul. Ships the Phase 1
foundation, the Phase 2 simulator + rewind + world tracker, the β
scope of Phase 3 check migrations, and the Phase 5 release
infrastructure. Phase 4 γ physics and the world / client / rewind
check families are specced in `docs/check-specs/` but not yet
implemented — γ features are disabled in this build.

### Added

- **Simulator (`anticheat/sim`)**
  - Deterministic player physics engine with gravity, drag, step-assist,
    sprint jump boost, and liquid drag.
  - Block-accurate AABB collision resolution against the world tracker
    snapshot.
  - Effect modifiers (speed / jump boost / slow falling).

- **World tracker (`anticheat/world`)**
  - Per-player chunk cache keyed on runtime ID with TTL-based unload.
  - Level event integration for block place / break / piston push.
  - Raycast + AABB intersection helpers (used by Reach and the γ
    BBox-based movement rewrite).

- **Entity rewind buffer (`anticheat/entity`)**
  - Bounded ring buffer of historic entity positions indexed by client
    tick, enabling ping-compensated reach and target correction.

- **Ack queue (`anticheat/ack`)**
  - Tick-keyed pending action queue so simulator decisions can wait
    until the client's confirmation arrives before committing state.

- **Mitigation dispatcher (`anticheat/mitigate`)**
  - Policy routing: `PolicyNone` / `PolicyClientRubberband` /
    `PolicyServerFilter` / `PolicyKick`.
  - Wired hooks: kick (disconnect) and rubberband (MovePlayer Reset
    snap). Server-filter hook is scaffolded but left unwired — see
    "Known limitations" below.
  - `expvar`-based metrics at `/debug/vars` under `better_pm_ac_*`:
    `violations_total`, `kicks_total`, `rubberbands_total`,
    `filtered_packets_total`, `dry_run_suppressed_total`.

- **Checks — β scope (spec + unit tests only; wired to dispatcher)**
  - Movement: `Speed/A`, `Fly/A`, `NoFall/A`, `Phase/A`, `NoSlow/A`,
    `Velocity/A`.
  - Combat: `Reach/A` (with 1.0-block ping compensation),
    `AutoClicker/A` (16 CPS cap), `Aim/A` (round-match + idle gate,
    mouse only), `KillAura/A-B-C` (swing-less / 90° FOV / multi-target).
  - Packet: `BadPacket/A-B-C-D-E` (malformed bit sets, inventory /
    container mismatches, bogus abilities).

- **Operations**
  - GitHub Actions CI: Linux + Windows matrix running `go build`,
    `go vet`, and `go test -race -count=1`; separate `golangci-lint`
    (v1.62) job; nightly bench smoke via cron.
  - `.golangci.yml` enabling errcheck, govet, ineffassign, staticcheck,
    unused, bodyclose, gosec, misspell, unconvert, unparam, prealloc,
    copyloopvar, and nilerr.
  - `docs/ops-runbook.md` with metric definitions, alert thresholds,
    a sample Grafana dashboard JSON, and an incident playbook.

### Changed

- Check registry is now dynamic (map-based) rather than a hardcoded
  struct of per-player detections.
- Every check exposes a `Policy()` method that routes through the
  dispatcher — legacy direct-kick call sites have been replaced.

### Fixed

- Fly/A terrain exemption (blocks with non-standard bounding boxes no
  longer produce false positives near step-assist boundaries).
- `GravViolTicks` first-tick initialization aligned with Oomph.
- Dynamic knockback grace window after velocity packets.

### Known limitations (resolved in γ)

- `PolicyServerFilter` hook is unwired; checks configured with this
  policy log and count as `dry_run_suppressed` but do not rewrite the
  offending packet. Server-filter wiring requires packet-pipeline
  reordering scheduled for v0.10.0.
- Per-check metric breakdown is deferred; operators correlate counters
  with the structured log stream instead.
- World / client / rewind check families (`3.C4.*`, `3.C5.*`,
  `Phase 4 γ physics extensions`) are specced but not yet implemented.
- AutoClicker/B, Aim/B, Scaffold/A rewind, and KillAura/B rewind
  variants are deferred to γ — they need timing-sensitive fixtures or
  the rewind-based remeasurement path that depends on Phase 4 work.

### Upgrade notes

- Go toolchain pinned to `1.24.x`; update `go.mod` and CI in the same
  PR if bumping.
- `config.yml` per-check schema adds a required `policy` string
  (`none` / `client_rubberband` / `server_filter` / `kick`). Existing
  installs that relied on implicit kick behavior should set
  `policy: kick` explicitly.

---

## [pre-v0.10.0] — historical

Prior commits (on `main` before the overhaul) implemented a narrower
set of checks directly in `anticheat/checks/` with a hardcoded
per-player detection struct. Those are superseded by the architecture
described above; see the git log for the full history.
