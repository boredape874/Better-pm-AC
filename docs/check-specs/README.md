# Check Specifications

Per-check deep-dive specs live in this directory. Each check from the 42-check
catalog in [../plans/2026-04-19-anticheat-overhaul-design.md §6](../plans/2026-04-19-anticheat-overhaul-design.md)
gets its own file once an AI claims its implementation task.

## File naming

`<category>-<type>-<subtype>.md`, all lowercase. Examples:

- `movement-fly-a.md`
- `combat-reach-b.md`
- `world-nuker-a.md`

## File structure

Each spec must include:

1. **Summary** — one-paragraph description.
2. **Inputs** — which subsystem outputs the check reads (SimState, Rewind, World, etc.).
3. **Algorithm** — step-by-step pseudocode.
4. **Thresholds** — fail_buffer, max_buffer, violations, plus any check-specific tolerances.
5. **Policy** — Kick / ClientRubberband / ServerFilter / None and reasoning.
6. **False-positive risk** — Low/Medium/High with known triggers to watch.
7. **Test vectors** — inputs that must pass and must fail, for the test suite.

Specs are written by the AI implementing the check, before code.
