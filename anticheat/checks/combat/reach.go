package combat

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	ac_combat "github.com/boredape874/Better-pm-AC/anticheat/combat"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
	"math"
)

// attackerEyeHeight is the vertical offset from a player's feet position to
// their eye level in Bedrock Edition (matches proxy.playerEyeHeight).
// Vanilla reach is measured from the attacker's eye position, not their feet.
const attackerEyeHeight = float32(1.62)

// reachMaxEntitySpeed is a conservative upper bound on how fast an entity
// can move per tick (blocks/tick). A sprinting player does ~0.28 b/tick;
// 0.3 gives a small margin for potion effects. This is used to compute the
// ping-based reach compensation window.
const reachMaxEntitySpeed = float32(0.3)

// reachPingCompCap is the maximum extra reach (blocks) that can be granted due
// to ping compensation. Capping prevents spoofed or extreme latency values from
// opening a near-unlimited reach window. 1.0 block matches Oomph's limit.
const reachPingCompCap = float32(1.0)

// ReachCheck detects attacks on entities that are beyond the configured reach
// distance, indicating Reach or long-sword cheats.
// Implements anticheat.Detection.
type ReachCheck struct {
	cfg config.ReachConfig
}

func NewReachCheck(cfg config.ReachConfig) *ReachCheck { return &ReachCheck{cfg: cfg} }

func (*ReachCheck) Type() string        { return "Reach" }
func (*ReachCheck) SubType() string     { return "A" }
func (*ReachCheck) Description() string { return "Checks if combat reach exceeds the vanilla value." }
func (*ReachCheck) Punishable() bool    { return true }
func (c *ReachCheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *ReachCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Matching Oomph ReachA: buffer must reach 1.01 before a violation is
		// counted, and decays naturally at 0.0015 per passing tick.
		FailBuffer:    1.01,
		MaxBuffer:     1.5,
		MaxViolations: float64(c.cfg.Violations),
		TrustDuration: 60 * 20, // 60 s × 20 tps = 1200 ticks
	}
}

func (*ReachCheck) Name() string { return "Reach/A" }

// buildLookDir computes the player's look-direction unit vector from yaw/pitch.
// Bedrock convention: yaw=0 → South (+Z), yaw=90 → West (-X).
func buildLookDir(yaw, pitch float32) mgl32.Vec3 {
	yawRad := float64(yaw) * math.Pi / 180
	pitchRad := float64(pitch) * math.Pi / 180
	return mgl32.Vec3{
		float32(-math.Sin(yawRad) * math.Cos(pitchRad)),
		float32(-math.Sin(pitchRad)),
		float32(math.Cos(yawRad) * math.Cos(pitchRad)),
	}.Normalize()
}

// Check evaluates whether the attack was within reach.
//
// When snapshots is non-empty the multi-tick RayCaster path is used:
//   - The attacker's look direction is cast as a ray from their eye position.
//   - CastN checks all provided entity bbox samples (rewind window).
//   - A swing is flagged if the ray misses entirely OR the nearest hit is
//     beyond maxReach (including ping compensation).
//
// When snapshots is empty the check falls back to single-point distance
// (legacy: distance from eye to target feet position).
func (c *ReachCheck) Check(p *data.Player, targetPos mgl32.Vec3, snapshots ...ac_combat.BBox) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	// Ping compensation (mirrors Oomph / GrimAC lag-compensation).
	latency := p.Latency()
	pingTicks := float32(latency.Seconds()) * 20.0 / 2.0
	pingComp := pingTicks * reachMaxEntitySpeed
	if pingComp > reachPingCompCap {
		pingComp = reachPingCompCap
	}
	maxReach := float32(c.cfg.MaxReach) + pingComp

	feetPos := p.CurrentPosition()
	eyePos := mgl32.Vec3{feetPos[0], feetPos[1] + attackerEyeHeight, feetPos[2]}

	// Multi-tick RayCaster path (T4.3).
	if len(snapshots) > 0 {
		yaw, pitch := p.RotationAbsolute()
		dir := buildLookDir(yaw, pitch)
		result := ac_combat.CastN(eyePos, dir, snapshots, maxReach)
		if !result.Hit {
			// Ray missed all bboxes in the rewind window.
			dist := eyePos.Sub(targetPos).Len()
			return true, fmt.Sprintf("raycast_miss dist=%.3f max=%.3f ping_comp=%.3f", dist, maxReach, pingComp)
		}
		if result.NearestT > maxReach {
			return true, fmt.Sprintf("raycast_reach=%.3f max=%.3f ping_comp=%.3f", result.NearestT, maxReach, pingComp)
		}
		return false, ""
	}

	// Fallback: single-point distance check.
	dist := eyePos.Sub(targetPos).Len()
	if dist > maxReach {
		return true, fmt.Sprintf("dist=%.3f max=%.3f ping_comp=%.3f", dist, maxReach, pingComp)
	}
	return false, ""
}
