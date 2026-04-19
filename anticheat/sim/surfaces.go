package sim

import (
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/df-mc/dragonfly/server/block/cube"
)

// refreshSurfaces inspects the blocks adjacent to the player's new position
// and sets the boolean flags on state that downstream checks read. The
// decisions here must be conservative: a check that trusts InLiquid=false
// when the player is actually in water will false-flag Jesus; the other way
// around false-clears Fly. Where AI-W's Block() returns unexpected block
// types we fail closed (leave flag false) and log via a dedicated world
// metric rather than panicking the sim.
func refreshSurfaces(state *meta.SimState, world meta.WorldTracker) {
	// Block under the player's feet (Y-1).
	feet := cube.Pos{
		int(state.Position[0]),
		int(state.Position[1]) - 1,
		int(state.Position[2]),
	}
	under := world.Block(feet)
	state.OnIce = isIce(under)
	state.OnSlime = isSlime(under)
	state.OnHoney = isHoney(under)
	state.OnSoulSand = isSoulSand(under)
	state.OnScaffolding = isScaffolding(under)

	// Blocks overlapping the player's AABB — used to detect liquid, cobweb,
	// climbable, powder snow.
	bodyMin := cube.Pos{
		int(state.Position[0]),
		int(state.Position[1]),
		int(state.Position[2]),
	}
	bodyMid := cube.Pos{bodyMin[0], bodyMin[1] + 1, bodyMin[2]}
	state.InLiquid = isLiquid(world.Block(bodyMin)) || isLiquid(world.Block(bodyMid))
	state.InCobweb = isCobweb(world.Block(bodyMin)) || isCobweb(world.Block(bodyMid))
	state.InPowderSnow = isPowderSnow(world.Block(bodyMin)) || isPowderSnow(world.Block(bodyMid))
	state.OnClimbable = isClimbable(world.Block(bodyMin)) || isClimbable(world.Block(bodyMid))
}
