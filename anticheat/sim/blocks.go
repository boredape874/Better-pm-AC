package sim

import (
	"strings"

	"github.com/df-mc/dragonfly/server/world"
)

// Block classifiers inspect world.Block by its Bedrock encoded name rather
// than type assertion. Dragonfly v0.10.12 exposes a subset of block types
// (Slime, SoulSand, PackedIce, BlueIce, Lava, Ladder, Vines, Water) but
// omits others (plain Ice, Cobweb, Scaffolding, PowderSnow, HoneyBlock).
// Name-based matching is stable across dragonfly versions and lets us
// support the missing types without building a custom block registry.

func blockName(b world.Block) string {
	if b == nil {
		return ""
	}
	name, _ := b.EncodeBlock()
	return name
}

func nameIs(b world.Block, suffixes ...string) bool {
	n := blockName(b)
	for _, s := range suffixes {
		if strings.HasSuffix(n, ":"+s) || n == s {
			return true
		}
	}
	return false
}

func isIce(b world.Block) bool {
	return nameIs(b, "ice", "packed_ice", "blue_ice", "frosted_ice")
}

func isSlime(b world.Block) bool    { return nameIs(b, "slime", "slime_block") }
func isHoney(b world.Block) bool    { return nameIs(b, "honey_block") }
func isSoulSand(b world.Block) bool { return nameIs(b, "soul_sand") }

func isScaffolding(b world.Block) bool { return nameIs(b, "scaffolding") }

func isLiquid(b world.Block) bool {
	return nameIs(b, "water", "flowing_water", "lava", "flowing_lava")
}

func isCobweb(b world.Block) bool { return nameIs(b, "web") }

func isPowderSnow(b world.Block) bool { return nameIs(b, "powder_snow") }

func isClimbable(b world.Block) bool {
	return nameIs(b, "ladder", "vine", "scaffolding", "twisting_vines",
		"weeping_vines", "cave_vines", "cave_vines_head_with_berries")
}
