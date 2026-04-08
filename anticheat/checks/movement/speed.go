package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// sprintSpeedMultiplier is the ratio of sprinting speed to walking speed in
// vanilla Bedrock Edition.  We apply it to the configured MaxSpeed so that
// legitimate sprinting is never flagged.
const sprintSpeedMultiplier = float32(1.30)

// sneakSpeedMultiplier is the ratio of sneaking speed to walking speed.
const sneakSpeedMultiplier = float32(0.30)

// SpeedCheck flags players whose horizontal movement exceeds the configured
// limit per tick. Velocity is now a raw positional delta (blocks/tick) rather
// than a wall-clock-derived blocks/second value, matching how Oomph computes
// displacement: it compares the positional delta from one PlayerAuthInput to
// the next against its simulated expectation.
//
// The check is limited to ticks where the player is on the ground.  Aerial
// speed is influenced by knockback, jump boost, ice momentum, and other
// factors that are better handled by dedicated checks (Fly/A).  Restricting
// to ground-movement eliminates nearly all Speed/A false positives without
// reducing detection of the most common speed hacks.
type SpeedCheck struct {
	cfg config.SpeedConfig
}

func NewSpeedCheck(cfg config.SpeedConfig) *SpeedCheck { return &SpeedCheck{cfg: cfg} }

func (*SpeedCheck) Type() string    { return "Speed" }
func (*SpeedCheck) SubType() string { return "A" }
func (*SpeedCheck) Description() string {
	return "Detects horizontal ground movement exceeding vanilla limits (blocks/tick)."
}
func (*SpeedCheck) Punishable() bool { return true }

func (c *SpeedCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    2,
		MaxBuffer:     4,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates the player's horizontal displacement in blocks/tick.
// The config MaxSpeed field is already expressed in blocks/tick (default 0.7).
// The effective limit is scaled by sprint/sneak state so that legitimate
// movement is never penalised.
func (c *SpeedCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	// Only check on-ground movement.  Aerial speed is complex (knockback,
	// terrain slopes, jump arcs) and is handled by Fly/A.
	if !p.IsOnGround() {
		return false, ""
	}

	speed := p.HorizontalSpeed() // blocks/tick
	maxSpeed := float32(c.cfg.MaxSpeed)

	// Adjust the limit based on the player's current input state.
	sprinting, sneaking, _ := p.InputSnapshot()
	switch {
	case sprinting:
		maxSpeed *= sprintSpeedMultiplier
	case sneaking:
		maxSpeed *= sneakSpeedMultiplier
	}

	if speed > maxSpeed {
		return true, fmt.Sprintf("speed=%.4f max=%.4f sprint=%v sneak=%v", speed, maxSpeed, sprinting, sneaking)
	}
	return false, ""
}
