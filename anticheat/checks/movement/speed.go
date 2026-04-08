package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// sprintSpeedMultiplier is the ratio of sprinting speed to walking speed in
// vanilla Bedrock Edition.  We apply it to the configured MaxSpeed so that
// legitimate sprinting is never flagged.
const sprintSpeedMultiplier = float32(1.30)

// sneakSpeedMultiplier is the ratio of sneaking speed to walking speed.
const sneakSpeedMultiplier = float32(0.30)

// speedEffectBonus is the speed bonus per amplifier level for the Speed potion
// effect. Speed I (amplifier=0) adds +20%, Speed II (amplifier=1) adds +40%,
// etc. Formula: maxSpeed *= (1 + speedEffectBonus * (amplifier + 1)).
const speedEffectBonus = float32(0.20)

// slownessSpeedPenalty is the speed penalty per amplifier level for the
// Slowness potion effect. Slowness I (amplifier=0) reduces speed by 15%,
// Slowness II (amplifier=1) by 30%, etc.
// Formula: maxSpeed *= (1 - slownessSpeedPenalty * (amplifier + 1)).
const slownessSpeedPenalty = float32(0.15)

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
// The effective limit is scaled by sprint/sneak state and active Speed potion
// effects so that legitimate movement is never penalised.
func (c *SpeedCheck) Check(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	// Creative players can legitimately move at any speed; exempt them
	// entirely to match Oomph's creative-mode exemption in movement checks.
	if p.IsCreative() {
		return false, ""
	}
	// Exempt players that just received server-applied velocity (knockback,
	// explosions, wind charges, etc.). The resulting speed spike is legitimate
	// and would otherwise trigger Speed/A for several ticks.
	if p.HasKnockbackGrace() {
		return false, ""
	}
	// Only check on-ground movement.  Aerial speed is complex (knockback,
	// terrain slopes, jump arcs) and is handled by Fly/A.
	if !p.IsOnGround() {
		return false, ""
	}
	// Skip the landing tick (the first tick the player transitions from
	// airborne to on-ground). At that moment, Velocity still carries the
	// momentum from the last airborne position, which can exceed the ground
	// speed limit even for a legitimate sprint-jump. Mirrors Oomph's
	// landing-frame grace in its ground-speed validation.
	if p.IsJustLanded() {
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

	// Adjust for an active Speed potion effect (effect type 1 = Speed).
	// Speed I (amplifier 0) grants +20%, Speed II (amplifier 1) grants +40%, etc.
	// Mirrors Oomph's attribute-based limit adjustment.
	if amp, active := p.EffectAmplifier(packet.EffectSpeed); active {
		maxSpeed *= 1.0 + speedEffectBonus*float32(amp+1)
	}

	// Adjust for an active Slowness potion effect (effect type 2 = Slowness).
	// Slowness I (amplifier 0) reduces speed by 15%, Slowness II by 30%, etc.
	// This tightens the allowed range and closes a detection gap where a player
	// using a speed hack below the unmodified limit would pass the check.
	if amp, active := p.EffectAmplifier(packet.EffectSlowness); active {
		maxSpeed *= 1.0 - slownessSpeedPenalty*float32(amp+1)
		if maxSpeed < 0 {
			maxSpeed = 0
		}
	}

	if speed > maxSpeed {
		return true, fmt.Sprintf("speed=%.4f max=%.4f sprint=%v sneak=%v", speed, maxSpeed, sprinting, sneaking)
	}
	return false, ""
}
