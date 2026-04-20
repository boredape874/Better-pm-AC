# combat-reach-a

## Summary

Reach/A flags attacks whose eye-to-target distance exceeds the vanilla
reach cap plus a ping-compensation window. Bedrock vanilla caps reach
at 3.0 blocks; anything beyond that on a confirmed attack packet is a
Reach / long-sword cheat.

The attacker position stored in `data.Player` is feet-level (proxy
subtracts the 1.62-block eye offset). The check adds 1.62 back before
measuring, so the distance is eye→target — matching the vanilla hit
test rather than feet→target (which would silently grant ~1.6 blocks
of free reach).

## Inputs

- `data.Player.CurrentPosition()` — feet-level Vec3.
- `targetPos mgl32.Vec3` — feet position of the victim entity.
- `data.Player.Latency()` — RTT; one-way delay is RTT/2.
- `config.ReachConfig.MaxReach` — base cap (default 3.0 blocks).

## Algorithm

```
if !Enabled: return pass
eyePos  = feetPos + (0, 1.62, 0)
dist    = |eyePos − targetPos|
pingTicks = Latency.Seconds * 20 / 2      # one-way in ticks
pingComp  = min(pingTicks * 0.3, 1.0)      # b/tick × ticks, capped
maxReach  = cfg.MaxReach + pingComp
if dist > maxReach:
    return flag("dist=... max=... ping_comp=...")
```

## Thresholds

| Field         | Value | Rationale |
|---------------|-------|-----------|
| FailBuffer    | 1.01  | Matches Oomph ReachA: one confirmed over-reach is a violation |
| MaxBuffer     | 1.5   | Small head-room for decay |
| MaxViolations | config.Violations (default 10) | Kick threshold |
| Eye offset    | 1.62 blocks | Bedrock playerEyeHeight |
| Ping comp cap | 1.0 block  | Matches Oomph — prevents spoofed-ping abuse |
| Entity speed  | 0.3 b/tick | Conservative sprinting upper bound |

## Policy

Default `Kick`. Reach is a high-confidence, high-impact cheat; a
confirmed over-reach past a 1-block ping budget is unambiguous.

## False-positive risk — **Low**

- Eye-position math matches the vanilla hit test.
- Ping compensation up to 1 block covers even 300 ms RTT.
- Target position comes from the server-authoritative entity table,
  not a client claim.

Residual risk: targets on fast-moving rideable mounts could drift
more than 0.3 b/tick; treat mount-rider targets as special-cased
above the Reach layer if the ecosystem adds them.

## Test vectors

See `anticheat/checks/combat/reach_test.go`:

1. `TestReachALegalInRangeDoesNotFlag` — 2.5-block dist passes.
2. `TestReachACheatFlags` — 6-block dist flags with `dist=/max=`.
3. `TestReachABoundaryAtCap` — 3.0 passes, 3.2 flags (0 ping).
4. `TestReachAPingCompensationWidensWindow` — 200 ms RTT grants
   extra reach; 3.5 passes where it would otherwise flag.
5. `TestReachAPolicyContract` — policy Kick.
