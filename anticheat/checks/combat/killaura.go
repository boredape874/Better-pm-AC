package combat

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// maxSwingTickDiff is the baseline maximum number of simulation ticks that may
// elapse between a swing and a hit before KillauraA flags. Oomph uses 10 ticks.
// The effective window is extended by the player's one-way ping in ticks so
// that high-latency clients whose swing and hit packets arrive out of order
// are not falsely flagged.
const maxSwingTickDiff = uint64(10)

// maxSwingPingCapTicks caps the extra swing-window granted by ping compensation.
// At 20 TPS, 10 extra ticks = 500 ms one-way, which covers even very high-latency
// connections without allowing unbounded tolerance.
const maxSwingPingCapTicks = uint64(10)

// KillAuraCheck (KillauraA) detects players that attack entities without
// swinging their arm within the expected tick window. This is Oomph's primary
// KillAura detection strategy: KillAura bots often send attack packets
// separately from swing animations (packet.Animate / InputFlagMissedSwing).
// Implements anticheat.Detection.
type KillAuraCheck struct {
	cfg config.KillAuraConfig
}

func NewKillAuraCheck(cfg config.KillAuraConfig) *KillAuraCheck {
	return &KillAuraCheck{cfg: cfg}
}

func (*KillAuraCheck) Type() string    { return "KillAura" }
func (*KillAuraCheck) SubType() string { return "A" }
func (*KillAuraCheck) Description() string {
	return "Detects attacking without swinging arm within the expected tick window."
}
func (*KillAuraCheck) Punishable() bool { return true }
func (c *KillAuraCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *KillAuraCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    1,
		MaxBuffer:     1,
		MaxViolations: float64(c.cfg.Violations),
	}
}

func (*KillAuraCheck) Name() string { return "KillAura/A" }

// Check evaluates whether the attack was accompanied by a recent arm swing.
//
// Logic (mirrors Oomph KillauraA):
//   - currentTick = player's SimulationFrame at time of attack
//   - lastSwing   = SimulationFrame of last packet.Animate / MissedSwing event
//   - tickDiff    = currentTick - lastSwing
//   - Flag if tickDiff > maxSwingTickDiff (i.e., no swing in the last 10 ticks)
func (c *KillAuraCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	currentTick := p.SimFrame()
	lastSwing := p.SwingTick()

	// On first attack before any swing has been recorded (lastSwing == 0),
	// give the player a grace pass to avoid false positives on join.
	if lastSwing == 0 {
		return false, ""
	}

	var tickDiff uint64
	if currentTick >= lastSwing {
		tickDiff = currentTick - lastSwing
	} else {
		// Tick wrapped or teleport/reconnect — reset gracefully.
		return false, ""
	}

	// Extend the allowed window by one-way ping in ticks so that high-latency
	// clients whose swing packet arrives after the attack packet are not falsely
	// flagged. Cap the extension to maxSwingPingCapTicks (mirrors GrimAC's
	// latency-compensated KillauraA window).
	latency := p.Latency()
	pingTicks := uint64(latency.Seconds() * 20.0 / 2.0)
	if pingTicks > maxSwingPingCapTicks {
		pingTicks = maxSwingPingCapTicks
	}
	effectiveMax := maxSwingTickDiff + pingTicks

	if tickDiff > effectiveMax {
		return true, fmt.Sprintf("tick_diff=%d last_swing=%d current=%d max=%d", tickDiff, lastSwing, currentTick, effectiveMax)
	}
	return false, ""
}
