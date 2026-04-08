package combat

import (
	"fmt"
	"math"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
)

// killAuraBMaxAngleDeg is the maximum angle (degrees) between the player's
// look direction and the direction from the player's eye to the target entity.
// If the target is further than this angle from the crosshair when the attack
// packet arrives, the player is flagged.
//
// 90° is chosen to match Oomph's KillauraB threshold: any entity more than
// 90° off-axis from the player's current look direction is behind or to the
// side in a way that no legitimate click could reach.
const killAuraBMaxAngleDeg = float64(90)

// killAuraBEyeHeight is the vertical offset from the stored feet position to
// the player's eye level, used to compute the correct eye→target direction.
// Must match attackerEyeHeight in reach.go and proxy.playerEyeHeight.
const killAuraBEyeHeight = float32(1.62)

// KillAuraBCheck detects players that attack entities outside their field of
// view — a clear signal of KillAura bots that auto-target regardless of where
// the player is looking.
//
// Algorithm (mirrors Oomph KillauraB):
//  1. Compute the player's look-direction unit vector from (yaw, pitch).
//  2. Compute the direction unit vector from the player's eye to the target.
//  3. Calculate the angle between the two vectors via acos(dot product).
//  4. Flag when the angle exceeds killAuraBMaxAngleDeg.
//
// Implements anticheat.Detection.
type KillAuraBCheck struct {
	cfg config.KillAuraBConfig
}

func NewKillAuraBCheck(cfg config.KillAuraBConfig) *KillAuraBCheck {
	return &KillAuraBCheck{cfg: cfg}
}

func (*KillAuraBCheck) Type() string    { return "KillAura" }
func (*KillAuraBCheck) SubType() string { return "B" }
func (*KillAuraBCheck) Description() string {
	return "Detects attacking entities outside the player's field of view."
}
func (*KillAuraBCheck) Punishable() bool { return true }

func (c *KillAuraBCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Require three consecutive off-FOV attacks before recording a
		// violation to avoid false-positives from lag-induced rotation drift.
		FailBuffer:    3,
		MaxBuffer:     5,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates whether the attacked entity is within the player's current
// field of view. targetPos is the server-authoritative feet position of the
// target entity, as tracked by the entity position table.
func (c *KillAuraBCheck) Check(p *data.Player, targetPos mgl32.Vec3) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	// Compute player eye position (stored position is feet-level).
	feetPos := p.CurrentPosition()
	eyePos := mgl32.Vec3{feetPos[0], feetPos[1] + killAuraBEyeHeight, feetPos[2]}

	// Direction from eye to target (not normalised yet).
	toTarget := targetPos.Sub(eyePos)
	if toTarget.Len() < 1e-4 {
		// Target is essentially at the same position as the player; skip.
		return false, ""
	}

	// Build the player's look-direction unit vector from (yaw, pitch).
	// Bedrock convention (same as Java):
	//   yaw=0   → South (+Z)
	//   yaw=90  → West  (-X)
	//   pitch=0 → horizontal; pitch=90 → looking straight down.
	yaw, pitch := p.RotationAbsolute()
	yawRad := float64(yaw) * math.Pi / 180
	pitchRad := float64(pitch) * math.Pi / 180
	lookX := float32(-math.Sin(yawRad) * math.Cos(pitchRad))
	lookY := float32(-math.Sin(pitchRad))
	lookZ := float32(math.Cos(yawRad) * math.Cos(pitchRad))
	lookVec := mgl32.Vec3{lookX, lookY, lookZ}.Normalize()

	toTargetNorm := toTarget.Normalize()

	// Clamp dot product to [-1, 1] to guard against floating-point rounding
	// before passing to acos.
	dot := float64(lookVec.Dot(toTargetNorm))
	if dot > 1.0 {
		dot = 1.0
	} else if dot < -1.0 {
		dot = -1.0
	}

	angleDeg := math.Acos(dot) * 180.0 / math.Pi
	if angleDeg > killAuraBMaxAngleDeg {
		return true, fmt.Sprintf("angle=%.1f max=%.0f", angleDeg, killAuraBMaxAngleDeg)
	}
	return false, ""
}
