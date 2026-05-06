package combat

import (
	"fmt"
	"math"

	ac_combat "github.com/boredape874/Better-pm-AC/anticheat/combat"
	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
)

// killAuraBMaxAngleDeg is the baseline maximum angle (degrees) between the
// player's look direction and the direction from the player's eye to the target
// entity. If the target is further than this angle from the crosshair when the
// attack packet arrives, the player is flagged.
//
// 90° is chosen to match Oomph's KillauraB threshold: any entity more than
// 90° off-axis from the player's current look direction is behind or to the
// side in a way that no legitimate click could reach. The effective threshold
// is widened by a ping-derived tolerance so that high-latency clients whose
// rotation and attack packets arrive out of order are not falsely flagged.
const killAuraBMaxAngleDeg = float64(90)

// killAuraBPingAnglePer100ms is the additional angle tolerance (degrees)
// granted per 100 ms of one-way ping.
const killAuraBPingAnglePer100ms = float64(3.0)

// killAuraBPingAngleCap is the maximum additional angle (degrees) that can
// be granted by ping compensation, regardless of the measured RTT.
const killAuraBPingAngleCap = float64(15.0)

// killAuraBEyeHeight is the vertical offset from the stored feet position to
// the player's eye level, used to compute the correct eye→target direction.
// Must match attackerEyeHeight in reach.go and proxy.playerEyeHeight.
const killAuraBEyeHeight = float32(1.62)

// KillAuraBCheck detects players that attack entities outside their field of
// view — a clear signal of KillAura bots that auto-target regardless of where
// the player is looking.
//
// When entity rewind snapshots are provided (γ.4 raycast path):
//   - CastN finds the nearest hit snapshot within the rewind window.
//   - The angle between the attacker's look direction and eye→targetCenter is
//     checked at the tick of the nearest hit rather than the current tick.
//   - Flag when the angle exceeds killAuraBMaxAngleDeg.
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
func (c *KillAuraBCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *KillAuraBCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Require three consecutive off-FOV attacks before recording a
		// violation to avoid false-positives from lag-induced rotation drift.
		FailBuffer:    3,
		MaxBuffer:     5,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// angleBetween computes the angle in degrees between two vectors.
func angleBetween(a, b mgl32.Vec3) float32 {
	dot := float64(a.Normalize().Dot(b.Normalize()))
	if dot > 1 {
		dot = 1
	}
	if dot < -1 {
		dot = -1
	}
	return float32(math.Acos(dot)) * 180 / math.Pi
}

// pingAngleTolerance computes the ping-based angle tolerance in degrees.
func pingAngleTolerance(p *data.Player) float64 {
	latency := p.Latency()
	oneWayMs := float64(latency.Milliseconds()) / 2.0
	return math.Min(oneWayMs/100.0*killAuraBPingAnglePer100ms, killAuraBPingAngleCap)
}

// Check evaluates whether the attacked entity is within the player's field of view.
//
// When snapshots is non-empty (γ.4 raycast path):
//   - Find the bbox in snapshots with the nearest hit via CastN.
//   - Use the centre of that bbox as the target centre for the angle check.
//
// When snapshots is empty, targetPos is used as the target point (legacy path).
func (c *KillAuraBCheck) Check(p *data.Player, targetPos mgl32.Vec3, snapshots ...ac_combat.BBox) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	// Compute player eye position (stored position is feet-level).
	feetPos := p.CurrentPosition()
	eyePos := mgl32.Vec3{feetPos[0], feetPos[1] + killAuraBEyeHeight, feetPos[2]}

	// Build the player's look-direction unit vector from (yaw, pitch).
	yaw, pitch := p.RotationAbsolute()
	lookVec := buildLookDir(yaw, pitch)

	// Determine the target point to use for the angle check.
	var target mgl32.Vec3
	if len(snapshots) > 0 {
		// Raycast path: find the bbox snapshot nearest to the ray.
		const castReach = float32(10.0)
		result := ac_combat.CastN(eyePos, lookVec, snapshots, castReach)
		if !result.Hit {
			// Ray missed all snapshots — use the fallback targetPos for the angle check.
			target = targetPos
		} else {
			// Find the bbox that gave the nearest hit by selecting the snapshot
			// whose Pos is closest to the ray hit point (NearestT * dir + eyePos).
			hitPoint := eyePos.Add(lookVec.Mul(result.NearestT))
			bestDist := float32(1e9)
			bestBox := snapshots[0]
			for _, bbox := range snapshots {
				d := bbox.Pos.Sub(hitPoint).Len()
				if d < bestDist {
					bestDist = d
					bestBox = bbox
				}
			}
			// Centre of the bbox: Pos is the foot-centre, add half-height for the entity centre.
			target = mgl32.Vec3{bestBox.Pos[0], bestBox.Pos[1] + bestBox.Height/2, bestBox.Pos[2]}
		}
	} else {
		target = targetPos
	}

	toTarget := target.Sub(eyePos)
	if toTarget.Len() < 1e-4 {
		return false, ""
	}

	angleDeg := float64(angleBetween(lookVec, toTarget))
	effectiveMax := killAuraBMaxAngleDeg + pingAngleTolerance(p)

	if angleDeg > effectiveMax {
		return true, fmt.Sprintf("angle=%.1f max=%.1f", angleDeg, effectiveMax)
	}
	return false, ""
}
