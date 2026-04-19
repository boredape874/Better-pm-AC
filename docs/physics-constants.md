# Bedrock Player Physics Constants

Reference sheet used by the sim engine. All values are per-tick at 20 TPS.
Values cross-referenced with the Oomph anti-cheat (`oomph-ac/oomph` master
branch `game/constants.go`, `player/simulation/*.go`) and with Dragonfly's
`server/entity/movement.go` behaviour.

Dragonfly delegates player physics to the client, so the authoritative source
for *player-specific* numbers is the Bedrock client itself. The values below
are the long-standing community-documented constants that Oomph matches.

## Vertical

| Constant | Value | Meaning |
|----------|-------|---------|
| `Gravity` | 0.08 b/tick² | Downward acceleration when airborne. |
| `AirDragY` | 0.98 | Multiplier applied to yVel every tick (after gravity). |
| `JumpVel` | 0.42 b/tick | Instantaneous y-velocity granted on jump. |
| `JumpBoostStep` | 0.10 b/tick | Added to `JumpVel` per JumpBoost level. |
| `SlowFallCap` | 0.01 b/tick | If `SlowFalling` active, yVel clamped so it does not go below `-SlowFallCap`. |
| `LevitationStep` | 0.05 b/tick | Upward velocity per Levitation level, replaces gravity. |

## Horizontal inputs

| Constant | Value | Meaning |
|----------|-------|---------|
| `GroundMovement` | 0.10 b/tick | Base forward input magnitude on ground. |
| `AirMovement` | 0.02 b/tick | Base forward input magnitude airborne. |
| `SprintMultiplier` | 1.30 | Multiplier when sprinting. |
| `SneakMultiplier` | 0.30 | Multiplier when sneaking (ground). |
| `UseItemMultiplier` | 0.20 | Multiplier when using a consumable/bow. |
| `SwimMultiplier` | 0.20 | Multiplier when swimming (not in water but Sprint-Swim mode). |
| `SpeedEffectStep` | 0.20 | Added per Speed level as multiplier above 1.0. |

## Friction

| Surface | Friction factor | Notes |
|---------|----------------|-------|
| `DefaultFriction` | 0.60 | Dirt, stone, grass, wood. Standard blocks. |
| `IceFriction` | 0.98 | Ice, packed ice, blue ice. |
| `SlimeFriction` | 0.80 | Slime block. |
| `SoulSandFriction` | 0.40 | Soul sand (40% slowdown). |
| `HoneyFriction` | 0.40 | Honey block (match soul sand). |
| `CobwebFriction` | 0.05 | Full slowdown in cobweb. |

Effective horizontal friction per tick on ground = `blockFriction × 0.91`.
Airborne uses `0.91` directly.

## Collision / sweep

| Constant | Value | Meaning |
|----------|-------|---------|
| `StepHeight` | 0.60 b | Maximum auto-step when a block blocks horizontal motion. |
| `BBoxWidth` | 0.60 b | Default player AABB width (`x` and `z` extent). |
| `BBoxHeight` | 1.80 b | Default standing player height. |
| `SneakBBoxHeight` | 1.50 b | Sneaking height. |
| `SwimBBoxHeight` | 0.60 b | Crawling / swimming pose height. |

## Fluid

| Constant | Value | Meaning |
|----------|-------|---------|
| `WaterDrag` | 0.80 | Velocity multiplier per tick in water (both axes). |
| `LavaDrag` | 0.50 | Velocity multiplier per tick in lava. |
| `WaterSwimBoost` | 0.04 b/tick | Extra forward impulse while Sprint-Swimming. |
| `BuoyancyY` | 0.04 b/tick | Upward impulse when head is in water. |

## Climbable

| Constant | Value | Meaning |
|----------|-------|---------|
| `ClimbUp` | 0.20 b/tick | Max ascent speed on ladder/vine when holding Jump or Forward-Up. |
| `ClimbDown` | 0.15 b/tick | Max descent speed (slower than gravity; clings). |
| `ScaffoldClimbUp` | 0.50 b/tick | Scaffolding (fast climb). |

## Divergences noted

- Dragonfly's `MovementComputer.applyHorizontalForces` multiplies by a fixed
  `0.6` block friction for unknown blocks — same as Bedrock default.
- Dragonfly's `MovementComputer.applyVerticalForces` supports
  `DragBeforeGravity` flag; for *players*, Bedrock applies drag after
  gravity (DragBeforeGravity = false). Our constants encode this.
- Oomph calls `JumpBoostStep` "jumpBoostMultiplier" and uses `0.1` identically.
- `basicengine92-tech/oomph:refactor/movement-sim` adjusts a subset of these
  values for 1.21+ parity; none diverge by more than 1% from the table above.

---

Edit this file only when Mojang changes player physics in a new Bedrock
release. All updates must cite the version and source.
