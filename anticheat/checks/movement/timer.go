package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// TimerCheck (Timer/A) detects players whose client sends PlayerAuthInput
// packets faster than the vanilla 20-packets-per-second rate. This is the
// primary signal for a Timer hack (also called a speed hack or tick manipulator),
// which accelerates game logic by increasing the simulation tick rate.
//
// Detection strategy (mirrors GrimAC Timer and Oomph's rate-based approach):
//   - Record the wall-clock arrival time of each PlayerAuthInput packet in a
//     rolling one-second window (data.Player.RecordInputTime / InputRate).
//   - If more than MaxRatePS packets are observed in that window, flag.
//
// The default MaxRatePS of 25 gives a 25 % tolerance above the nominal 20 TPS,
// which is enough to absorb brief bursts caused by network jitter while still
// catching Timer hacks operating at ≥ 1.25× speed.
type TimerCheck struct {
	cfg config.TimerConfig
}

func NewTimerCheck(cfg config.TimerConfig) *TimerCheck { return &TimerCheck{cfg: cfg} }

func (*TimerCheck) Type() string    { return "Timer" }
func (*TimerCheck) SubType() string { return "A" }
func (*TimerCheck) Description() string {
	return "Detects PlayerAuthInput arriving faster than the configured rate (packets/s)."
}
func (*TimerCheck) Punishable() bool { return true }

func (c *TimerCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    2,
		MaxBuffer:     4,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates the rolling PlayerAuthInput arrival rate for the current tick.
// It must be called after data.Player.RecordInputTime so that the timestamp of
// the current packet has already been added to the rolling window.
func (c *TimerCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	rate := p.InputRate()
	if rate > c.cfg.MaxRatePS {
		return true, fmt.Sprintf("rate=%d max=%d", rate, c.cfg.MaxRatePS)
	}
	return false, ""
}
