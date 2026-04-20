# phase4-physics-extensions (Phase 4 γ — AI-S)

Combined spec for the eight γ-scope physics extensions to the
`anticheat/sim` engine. These are not detection checks themselves;
they extend the sim engine so existing movement checks (Speed, Fly,
NoFall, Phase, NoSlow, Velocity) produce zero false-positives during
the listed gameplay states.

Each extension lands as a single file under `anticheat/sim/` plus a
unit test exercising the new state transitions. The Manager change
in each case is one line: register the new state on the per-player
`SimState` struct and ensure `Engine.Step` consults it.

## 4.1 Elytra glide

Add `Gliding bool` flag to `SimState`. When set:
- Replace gravity with the elytra glide model (Bedrock parity:
  `vy *= 0.99` per tick, additional `vy -= 0.05` capped by horizontal
  speed; with a Firework Rocket boost: `vx,vy,vz *= 1.5` for 5 ticks).
- Skip ground-friction; airborne friction = 0.99.
- Fly/A and Fly/B exempt while gliding.

File: `anticheat/sim/elytra.go`. Test: `TestElytraGlideMatchesVanilla`.

## 4.2 Trident riptide

Add `Riptiding int` (countdown ticks). When > 0:
- Velocity is the riptide impulse (0.4 × (sin(pitch), -sin(pitch),
  cos(pitch)) for Riptide I, scaled by level).
- Gravity disabled until Riptiding ticks down to zero.
- Velocity/A skipped (the impulse already overrides knockback).

File: `anticheat/sim/riptide.go`.

## 4.3 Wind charge knockback

Add a `WindChargePending mgl32.Vec3` field. When the proxy observes
a Wind Charge explosion within the player's blast radius, set this
field and consume it in `applyVertical`. Velocity/A treats this as
an applied knockback so the existing Anti-KB check still fires when
suppressed.

File: `anticheat/sim/windcharge.go`.

## 4.4 Powder snow sink/climb

Extend `surfaces.go` to recognise `minecraft:powder_snow` block IDs.
- Sinking: vertical velocity capped at -0.05 b/tick when not wearing
  Leather Boots (Bedrock parity).
- Climbing: when wearing Leather Boots, treat as a climbable surface
  (Sneak holds position; jump = +0.4 vy).

Files: `anticheat/sim/blocks.go` (block ID + flags),
`anticheat/sim/climbable.go` (boots branch).

## 4.5 Honey block slide

Extend `surfaces.go`: honey-block side contact while moving downward
caps vertical velocity at -0.05 b/tick. Horizontal friction = 0.4
(very sticky). NoFall exemption while sliding (FallDistance reset
each tick of contact).

File: `anticheat/sim/blocks.go` + `friction.go`.

## 4.6 Cobweb full-slow

Replace the partial cobweb handling with full-slow: any movement
through a cobweb block sets velocity *= 0.05 in all three axes.
Speed/A exemption while in cobweb.

Files: `anticheat/sim/blocks.go`, `anticheat/sim/walk.go`.

## 4.7 Scaffolding sneak/jump

Add scaffolding-specific behaviour to `walk.go`:
- Sneak on top of scaffolding clamps `vy` to 0 (no fall-through).
- Jump while standing in scaffolding sets `vy` to +0.42 (climb up).
- Down + scaffolding sets `vy` to -0.5 (descend through).

File: `anticheat/sim/walk.go`.

## 4.8 Slime bounce chain

Extend collision sweep: when landing on a slime block with `vy < -0.1`,
reflect `vy *= -0.8` (vanilla bounce coefficient). Sneak suppresses
the bounce. NoFall exempt during bounce (FallDistance reset).

Files: `anticheat/sim/collision.go`, `anticheat/sim/jump.go`.

## Common acceptance

Each extension requires:
1. Section above finalized.
2. Implementation file under `anticheat/sim/`.
3. Unit test in `anticheat/sim/<name>_test.go` exercising:
   - Vanilla parity case (matches reference Bedrock numbers).
   - Boundary case (just under / just over a threshold).
   - Off-state case (extension does not affect non-applicable ticks).
4. Reference numbers documented inline citing the Minecraft wiki page
   (Bedrock-only physics page where applicable).

## Open questions

- **Slow-fall × elytra interaction:** which dampens which? Bedrock
  applies slow-fall first then elytra glide cap.
- **Riptide in shallow water:** elytra-style trident throw stops
  exactly at 1-block depth. Need world-tracker for water depth.
