package combat

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// KillAuraCCheck (KillAura/C) detects players that attack more than one entity
// within the same simulation tick. In vanilla Bedrock Edition a player can only
// hit one target per tick; hitting multiple targets simultaneously is only
// possible with KillAura software that inserts additional attack packets.
//
// Algorithm (mirrors GrimAC MultiTarget and Oomph's same-tick multi-attack check):
//  1. RecordAttack tracks how many distinct attack events the player sends
//     within the same SimulationFrame (LastAttackCount / LastAttackTick).
//  2. If LastAttackCount > 1 on a given tick, flag.
//
// Implements anticheat.Detection.
type KillAuraCCheck struct {
	cfg config.KillAuraCConfig
}

func NewKillAuraCCheck(cfg config.KillAuraCConfig) *KillAuraCCheck {
	return &KillAuraCCheck{cfg: cfg}
}

func (*KillAuraCCheck) Type() string    { return "KillAura" }
func (*KillAuraCCheck) SubType() string { return "C" }
func (*KillAuraCCheck) Description() string {
	return "Detects attacking multiple entities within the same simulation tick."
}
func (*KillAuraCCheck) Punishable() bool { return true }

func (c *KillAuraCCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// One confirmed multi-target hit is already definitive; no buffer needed.
		FailBuffer:    1,
		MaxBuffer:     1,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates whether the player attacked more than one entity in the current
// simulation tick. Must be called after RecordAttack so the count is up to date.
func (c *KillAuraCCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	_, count := p.AttackTickCount()
	if count > 1 {
		return true, fmt.Sprintf("targets_per_tick=%d", count)
	}
	return false, ""
}
