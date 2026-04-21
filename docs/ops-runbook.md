# Better-pm-AC Operator Runbook

Audience: server operators running `better-pm-ac` as a MiTM proxy in front
of a PMMP dedicated server. This document describes what the anti-cheat
reports, how to read it, and what to do when a signal fires.

If you are looking for the design rationale or check specifications, see
`docs/plans/` and `docs/check-specs/` respectively — this file is the
"what do I do at 3 AM" reference.

---

## 1. Binary layout and flags

The proxy is a single Go binary. Configuration is in `config.yml` in the
working directory; per-check thresholds live under `anticheat.checks.*`
and follow the naming pattern `<category>/<subcheck>` (e.g.
`movement/flya`, `combat/reacha`). Each check accepts:

- `enabled: bool` — soft-disables the check without rebuilding.
- `policy: none | client_rubberband | server_filter | kick` — what to
  do on violation. `none` means "log and count only".
- `max_violations: int` — for kick policy, how many violations before
  the session is torn down.

Dry-run mode is **not** a separate flag — it emerges automatically when a
mitigation hook is left unwired. On a fresh install the proxy wires
`kick` and `client_rubberband` but NOT `server_filter` (the filter path
requires packet-pipeline reordering and is scheduled for γ). Any check
configured with `policy: server_filter` in β will therefore be logged
and counted under `dry_run_suppressed` but no packet will be rewritten.
This is intentional: we want the signal before we commit to the filter
wiring.

---

## 2. Metrics

All counters are exposed via Go's standard `expvar` package at
`/debug/vars` (only reachable if `net/http/pprof` is imported in the
main binary — the default setup does so). Scrape JSON with
`curl -s http://127.0.0.1:<debug-port>/debug/vars | jq`.

| Counter | Type | When it increments |
|---|---|---|
| `better_pm_ac_violations_total` | uint64 | Every `Dispatcher.Apply` call whose policy is **not** `PolicyNone`. This is the top-line "how noisy is the AC" number. |
| `better_pm_ac_kicks_total` | uint64 | A `PolicyKick` call that (a) has a configured `KickFunc` and (b) whose `DetectionMetadata.Exceeded()` returned true. A kicked player = exactly one tick here. |
| `better_pm_ac_rubberbands_total` | uint64 | A `PolicyClientRubberband` call that actually invoked the `RubberbandFunc`. One per corrective teleport. |
| `better_pm_ac_filtered_packets_total` | uint64 | A `PolicyServerFilter` call that invoked the filter hook. Filter may still have returned the original packet — this counts **decisions**, not mutations. |
| `better_pm_ac_dry_run_suppressed_total` | uint64 | Any active policy (kick / rubberband / filter) whose hook was `nil`. This is how you tell "I have signal but mitigation is off" from "I have no signal". |

`violations_total` is always ≥ the sum of `kicks + rubberbands + filtered + dry_run_suppressed_for_active_policies`. The gap is explained by:

- `PolicyKick` calls where `max_violations` was NOT exceeded — they still
  count as a violation (they indicate a real detection) but no kick
  fires.
- Detections that are non-punishable (`Punishable() == false`).

Per-check breakdown is **not** published in β. The implementation would
either contend a `sync.Map` on the hot path or require registration at
startup; both tradeoffs are on the γ roadmap. For β, operators correlate
counters with the structured log stream (§3).

---

## 3. Structured log fields

Every `Dispatcher.Apply` emits exactly one `slog.Warn("violation", …)`
record with these fields:

| Field | Example | Meaning |
|---|---|---|
| `player` | `7e1b4a…` | UUID of the offending session. |
| `check` | `Combat/ReachA` | `Type()/SubType()` of the detection. |
| `violations` | `4` | Running violation count for the player on this check. |
| `max` | `5` | Configured `max_violations`. |
| `policy` | `kick` | Rendered policy name (`none` / `client_rubberband` / `server_filter` / `kick` / `unknown`). |

Additional records:

- `kick suppressed — no KickFunc configured` (warn) — dry-run kick.
- `rubberband suppressed — no hook configured` (warn) — dry-run rubber.
- `server filter suppressed — no hook configured` (warn) — dry-run filter.
- `unknown policy, defaulting to none` (error) — config drift; the
  policy string in `config.yml` didn't parse. Investigate.

Pipe into `jq -rc 'select(.msg=="violation") | [.player,.check,.policy] | @tsv'`
for a human-readable live stream.

---

## 4. Alert thresholds (recommended starting values)

These are starting points tuned for a 100-CCU server. Adjust based on
your own baseline after one week of observation.

- **Rate of violations_total > 10 / sec sustained over 60s**: likely a
  bad config (a check's threshold is too tight) OR a legitimate raid of
  cheaters. Check `check` field distribution in logs first.
- **kicks_total growing without rubberbands_total growing**: a check is
  going straight to kick without any soft enforcement. Fine for
  packet-layer cheats, suspicious for movement (should usually rubberband
  first in γ).
- **dry_run_suppressed_total growing > 0**: you have checks configured
  for policies whose hooks aren't wired. Intentional during β for
  `server_filter`; otherwise, a misconfiguration.
- **Zero activity across all counters for > 5 min on a populated server**:
  the AC is not running, not processing packets, or all checks are
  disabled. Check `enabled` flags and proxy → server packet flow.

---

## 5. Dashboard sample (Grafana / expvar-prometheus-exporter)

For operators using the standard expvar-to-Prometheus bridge
(`github.com/prometheus/expvar_exporter` or equivalent), the panel JSON
below is a minimal "AC health" row. Customize units to your liking.

```json
{
  "title": "Better-pm-AC β",
  "rows": [
    {
      "title": "Detection rate",
      "panels": [
        {"type": "graph", "title": "violations/s",
         "targets": [{"expr": "rate(better_pm_ac_violations_total[1m])"}]},
        {"type": "graph", "title": "kicks/min",
         "targets": [{"expr": "rate(better_pm_ac_kicks_total[1m]) * 60"}]},
        {"type": "graph", "title": "rubberbands/s",
         "targets": [{"expr": "rate(better_pm_ac_rubberbands_total[1m])"}]}
      ]
    },
    {
      "title": "Dry-run accounting",
      "panels": [
        {"type": "stat", "title": "suppressed (total)",
         "targets": [{"expr": "better_pm_ac_dry_run_suppressed_total"}]}
      ]
    }
  ]
}
```

---

## 6. Common incident playbook

**"My legit players are getting kicked."**
1. Pull last 1h of `violation` log records filtered by `player=<uuid>`.
2. Look at the `check` distribution — one check dominating usually means
   that check's threshold is too tight (or broken by a new client build).
3. Flip that check to `policy: none` in `config.yml`, reload the proxy.
   Signal is preserved in `violations_total` and logs; kicks stop.
4. File a bug in `docs/check-specs/<check>.md` with a note.

**"My players are rubberbanding in loops."**
1. Confirm via `rubberbands_total` delta — >1/sec for a single player is
   a loop.
2. Disable the offending check (`movement/flya`, `movement/speeda`,
   etc.) and investigate: client-side prediction drift, lag, or a
   genuine bug in the simulator.
3. γ introduces rewind-based remeasurement which eliminates most of
   these — if you can wait for γ, do.

**"The AC isn't catching obvious cheaters."**
1. Check `dry_run_suppressed_total` — if it's growing, your checks are
   wired to a policy whose hook is `nil`. Fix config (usually
   `policy: kick` instead of `policy: server_filter`).
2. Check `enabled: true` on the relevant checks.
3. Correlate with `violations_total` — if the counter is moving but no
   kick, your `max_violations` is too high. Lower it incrementally.
4. If `violations_total` is flat, the detection itself isn't firing —
   check the spec in `docs/check-specs/` for the expected trigger
   conditions and file a bug.

---

## 7. Log retention and forensics

Slog records are written to whatever handler the proxy main configures
(stdout by default). For forensics, operators typically pipe stdout to
`jq`-friendly files rotated daily. A single violation record is ~200
bytes so a 100-CCU server generating ~1 violation/s produces ~17 MB/day
— trivial to retain for 30 days on any modern box.

The proxy itself does NOT retain violation history across restarts. If
you need longer-term tracking, forward the log stream to Loki /
Elasticsearch / etc. and query there.
