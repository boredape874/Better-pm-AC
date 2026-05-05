package movement

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// sprintSpeedMultiplier is the ratio of sprinting speed to walking speed in
// vanilla Bedrock Edition.  We apply it to the configured MaxSpeed so that
// legitimate sprinting is never flagged.
const sprintSpeedMultiplier = float32(1.30)

// sneakSpeedMultiplier is the ratio of sneaking speed to walking speed.
const sneakSpeedMultiplier = float32(0.30)

// crawlSpeedMultiplier is the ratio of crawling speed to walking speed.
// In vanilla Bedrock Edition, crawling speed is approximately 0.15×–0.20× of
// walking speed, which is even slower than sneaking. We use 0.25 to allow a
// small tolerance above the empirical minimum and avoid false positives on
// latency-affected frames.
const crawlSpeedMultiplier = float32(0.25)

// useItemSpeedMultiplier is the ratio of item-use speed to walking speed.
// In vanilla Bedrock Edition, a player eating or drawing a bow moves at
// approximately 27% of their base walking speed.  We allow 0.35 as a tolerant
// upper bound; NoSlow/A uses 0.30 as a tighter threshold for its own check.
const useItemSpeedMultiplier = float32(0.35)

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
	cfg       config.SpeedConfig
	authority *config.AuthorityConfig
}

func NewSpeedCheck(cfg config.SpeedConfig) *SpeedCheck { return &SpeedCheck{cfg: cfg} }

// SetAuthority wires the shared AuthorityConfig so the check can read
// MovementAuth at call time without breaking the Detection interface.
func (c *SpeedCheck) SetAuthority(a *config.AuthorityConfig) { c.authority = a }

func (*SpeedCheck) Type() string    { return "Speed" }
func (*SpeedCheck) SubType() string { return "A" }
func (*SpeedCheck) Description() string {
	return "Detects horizontal ground movement exceeding vanilla limits (blocks/tick)."
}
func (*SpeedCheck) Punishable() bool { return true }
func (c *SpeedCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *SpeedCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    2,
		MaxBuffer:     4,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// CheckLegacy is the original position-delta implementation of Speed/A
// (γ.3.1 migration: retained as fallback when MovementAuth is disabled).
func (c *SpeedCheck) CheckLegacy(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if p.IsCreative() {
		return false, ""
	}
	if p.HasKnockbackGrace() {
		return false, ""
	}
	if !p.IsOnGround() {
		return false, ""
	}
	if p.IsJustLanded() {
		return false, ""
	}

	speed := p.HorizontalSpeed()
	maxSpeed := float32(c.cfg.MaxSpeed)

	sprinting, sneaking, _, crawling, usingItem := p.InputSnapshotFull()
	switch {
	case usingItem:
		maxSpeed *= useItemSpeedMultiplier
	case crawling:
		maxSpeed *= crawlSpeedMultiplier
	case sprinting:
		maxSpeed *= sprintSpeedMultiplier
	case sneaking:
		maxSpeed *= sneakSpeedMultiplier
	}

	if amp, active := p.EffectAmplifier(packet.EffectSpeed); active {
		maxSpeed *= 1.0 + speedEffectBonus*float32(amp+1)
	}
	if amp, active := p.EffectAmplifier(packet.EffectSlowness); active {
		maxSpeed *= 1.0 - slownessSpeedPenalty*float32(amp+1)
		if maxSpeed < 0 {
			maxSpeed = 0
		}
	}

	if speed > maxSpeed {
		return true, fmt.Sprintf("speed=%.4f max=%.4f sprint=%v sneak=%v crawl=%v usingItem=%v", speed, maxSpeed, sprinting, sneaking, crawling, usingItem)
	}
	return false, ""
}

// Check evaluates the player's horizontal displacement in blocks/tick.
// When MovementAuth is enabled the check uses CommittedPos delta (server-
// authoritative) instead of the client-reported Velocity field.
// The config MaxSpeed field is already expressed in blocks/tick (default 0.7).
// The effective limit is scaled by sprint/sneak state and active Speed potion
// effects so that legitimate movement is never penalised.
func (c *SpeedCheck) Check(p *data.Player) (bool, string) {
	if c.authority != nil && c.authority.MovementAuth {
		return c.checkCommitted(p)
	}
	return c.CheckLegacy(p)
}

// checkCommitted is the CommittedPos-delta path for MovementAuth mode.
// It computes the horizontal distance between this tick's and the previous
// tick's committed (server-accepted) position instead of using the client-
// reported Velocity field. This makes the check immune to position spoofing
// because CommittedPos is derived from reconcile.Decide, not the raw claim.
func (c *SpeedCheck) checkCommitted(p *data.Player) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	if p.IsCreative() {
		return false, ""
	}
	if p.HasKnockbackGrace() {
		return false, ""
	}
	if !p.IsOnGround() {
		return false, ""
	}
	if p.IsJustLanded() {
		return false, ""
	}

	// Horizontal delta from committed positions (server-authoritative).
	delta := p.CommittedPos().Sub(p.PrevCommittedPos())
	speed := mgl32.Vec2{delta[0], delta[2]}.Len()
	maxSpeed := float32(c.cfg.MaxSpeed)

	sprinting, sneaking, _, crawling, usingItem := p.InputSnapshotFull()
	switch {
	case usingItem:
		maxSpeed *= useItemSpeedMultiplier
	case crawling:
		maxSpeed *= crawlSpeedMultiplier
	case sprinting:
		maxSpeed *= sprintSpeedMultiplier
	case sneaking:
		maxSpeed *= sneakSpeedMultiplier
	}

	if amp, active := p.EffectAmplifier(packet.EffectSpeed); active {
		maxSpeed *= 1.0 + speedEffectBonus*float32(amp+1)
	}
	if amp, active := p.EffectAmplifier(packet.EffectSlowness); active {
		maxSpeed *= 1.0 - slownessSpeedPenalty*float32(amp+1)
		if maxSpeed < 0 {
			maxSpeed = 0
		}
	}

	if speed > maxSpeed {
		return true, fmt.Sprintf("committed_speed=%.4f max=%.4f sprint=%v sneak=%v crawl=%v usingItem=%v", speed, maxSpeed, sprinting, sneaking, crawling, usingItem)
	}
	return false, ""
}
