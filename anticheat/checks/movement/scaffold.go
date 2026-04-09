package movement

import (
	"fmt"
	"math"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/go-gl/mathgl/mgl32"
)

// scaffoldEyeHeight is the vertical offset from the stored feet position to the
// player's eye level, used to compute the correct eye→block-centre direction.
// Must match proxy.playerEyeHeight and attackerEyeHeight in the combat checks.
const scaffoldEyeHeight = float32(1.62)

// scaffoldMaxAngleDeg is the maximum angle (degrees) between the player's look
// direction and the direction from the player's eye to the centre of the face
// being placed on. An angle larger than this means the player cannot
// legitimately be looking at that face while placing the block.
//
// Vanilla Bedrock requires the target face to be within the player's crosshair
// (field of view ≈ 60° at default FOV). We use 90° to match Oomph's Scaffold/A
// threshold and provide tolerance for high-latency clients whose rotation packet
// has not yet arrived when the placement packet is processed.
const scaffoldMaxAngleDeg = float64(90)

// scaffoldPingAnglePer100ms is extra angle tolerance (degrees) granted per 100 ms
// of one-way ping, to compensate for lag-induced rotation drift. Same formula as
// KillAura/B (mirrors Oomph's latency-tolerant placement angle check).
const scaffoldPingAnglePer100ms = float64(3.0)

// scaffoldPingAngleCap caps the ping-derived angle bonus regardless of RTT.
const scaffoldPingAngleCap = float64(15.0)

// blockFaceNormal maps Bedrock block face indices to their outward normal
// vectors. Face 0 = Down (-Y), 1 = Up (+Y), 2 = North (-Z), 3 = South (+Z),
// 4 = West (-X), 5 = East (+X). These match Bedrock's BlockFace constants.
var blockFaceNormal = [6]mgl32.Vec3{
	{0, -1, 0}, // 0: Down
	{0, 1, 0},  // 1: Up
	{0, 0, -1}, // 2: North
	{0, 0, 1},  // 3: South
	{-1, 0, 0}, // 4: West
	{1, 0, 0},  // 5: East
}

// ScaffoldCheck (Scaffold/A / Placement/A) detects impossible block-placement
// angles that indicate a Scaffold or Breezily cheat. Such cheats place blocks
// behind or below a moving player without the player's look direction pointing
// near the clicked block face.
//
// Algorithm (mirrors Oomph's Scaffold/A and GrimAC's PlaceOrder check):
//  1. On a UseItemOnBlock transaction (InventoryTransaction with ActionType ==
//     UseItemActionClickBlock) the proxy calls Check with the block position
//     and face index reported by the client.
//  2. Compute the block-face centre from BlockPosition + 0.5×normal.
//  3. Compute the direction from the player's eye to that face centre.
//  4. Compare this direction against the player's stored look direction.
//  5. If the angle exceeds scaffoldMaxAngleDeg (+ ping tolerance), flag.
//
// This check does NOT require server-side world state: it only uses the
// attacker's reported position/rotation and the client-supplied placement data.
// Clients that send fraudulent BlockPosition or BlockFace values to bypass
// the check would still need their look direction to match, which a scaffold
// cheat does not do.
//
// Implements anticheat.Detection.
type ScaffoldCheck struct {
	cfg config.ScaffoldConfig
}

func NewScaffoldCheck(cfg config.ScaffoldConfig) *ScaffoldCheck {
	return &ScaffoldCheck{cfg: cfg}
}

func (*ScaffoldCheck) Type() string    { return "Scaffold" }
func (*ScaffoldCheck) SubType() string { return "A" }
func (*ScaffoldCheck) Description() string {
	return "Detects block placements at impossible angles (Scaffold/Breezily cheats)."
}
func (*ScaffoldCheck) Punishable() bool { return true }

func (c *ScaffoldCheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// Require two consecutive impossible placements before flagging, to
		// absorb a single mis-click or lag-induced rotation discrepancy.
		FailBuffer:    2,
		MaxBuffer:     3,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates whether the player could legitimately have clicked the given
// block face. blockPos is the base block being interacted with (as reported in
// UseItemTransactionData.BlockPosition, converted to a float32 Vec3). face is
// the UseItemTransactionData.BlockFace index (0–5).
func (c *ScaffoldCheck) Check(p *data.Player, blockPos mgl32.Vec3, face int32) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	// Creative players can place blocks freely at any angle; exempt entirely.
	if p.IsCreative() {
		return false, ""
	}
	// Validate face index to avoid out-of-bounds access.
	if face < 0 || face > 5 {
		return false, ""
	}

	// Compute the eye position (stored position is feet-level).
	feetPos := p.CurrentPosition()
	eyePos := mgl32.Vec3{feetPos[0], feetPos[1] + scaffoldEyeHeight, feetPos[2]}

	// Compute the centre of the clicked face:
	//   face centre = block origin + (0.5, 0.5, 0.5) + 0.5 × face normal
	normal := blockFaceNormal[face]
	blockCentre := mgl32.Vec3{
		blockPos[0] + 0.5 + normal[0]*0.5,
		blockPos[1] + 0.5 + normal[1]*0.5,
		blockPos[2] + 0.5 + normal[2]*0.5,
	}

	toBlock := blockCentre.Sub(eyePos)
	if toBlock.Len() < 1e-4 {
		// Block is at the same position as the player; skip.
		return false, ""
	}

	// Build the player's look-direction unit vector from stored yaw/pitch.
	// Same formula as KillAura/B (Bedrock coordinate convention).
	yaw, pitch := p.RotationAbsolute()
	yawRad := float64(yaw) * math.Pi / 180
	pitchRad := float64(pitch) * math.Pi / 180
	lookX := float32(-math.Sin(yawRad) * math.Cos(pitchRad))
	lookY := float32(-math.Sin(pitchRad))
	lookZ := float32(math.Cos(yawRad) * math.Cos(pitchRad))
	lookVec := mgl32.Vec3{lookX, lookY, lookZ}.Normalize()

	toBlockNorm := toBlock.Normalize()

	dot := float64(lookVec.Dot(toBlockNorm))
	if dot > 1.0 {
		dot = 1.0
	} else if dot < -1.0 {
		dot = -1.0
	}
	angleDeg := math.Acos(dot) * 180.0 / math.Pi

	// Ping-based angle tolerance (mirrors GrimAC / Oomph latency compensation).
	latency := p.Latency()
	oneWayMs := float64(latency.Milliseconds()) / 2.0
	pingAngle := math.Min(oneWayMs/100.0*scaffoldPingAnglePer100ms, scaffoldPingAngleCap)
	effectiveMax := scaffoldMaxAngleDeg + pingAngle

	if angleDeg > effectiveMax {
		return true, fmt.Sprintf("angle=%.1f max=%.1f face=%d ping_tol=%.1f",
			angleDeg, effectiveMax, face, pingAngle)
	}
	return false, ""
}
