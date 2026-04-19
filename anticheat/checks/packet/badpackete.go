package packet

import (
	"strings"

	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// BadPacketECheck (BadPacket/E) detects PlayerAuthInput packets that carry
// contradictory start+stop flags for the same state in a single tick. A
// legitimate Bedrock client never sends both InputFlagStartSprinting and
// InputFlagStopSprinting in the same packet because the events are mutually
// exclusive transitions — you either begin sprinting or you stop, never both.
//
// Flag pairs checked (mirrors GrimAC BadPackets and Oomph's flag-validity
// component):
//   - StartSprinting + StopSprinting
//   - StartSneaking + StopSneaking
//   - StartSwimming + StopSwimming
//   - StartGliding  + StopGliding
//   - StartCrawling + StopCrawling
//
// Each contradiction indicates either a packet injector, a modified client that
// deliberately sends impossible state transitions, or a bot framework that does
// not properly manage input flags.
//
// Implements anticheat.Detection.
type BadPacketECheck struct {
	cfg config.BadPacketEConfig
}

func NewBadPacketECheck(cfg config.BadPacketEConfig) *BadPacketECheck {
	return &BadPacketECheck{cfg: cfg}
}

func (*BadPacketECheck) Type() string    { return "BadPacket" }
func (*BadPacketECheck) SubType() string { return "E" }
func (*BadPacketECheck) Description() string {
	return "Detects contradictory start+stop input flags in the same PlayerAuthInput packet."
}
func (*BadPacketECheck) Punishable() bool { return true }
func (*BadPacketECheck) Policy() meta.MitigatePolicy { return meta.PolicyKick }

func (c *BadPacketECheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		// One occurrence is already impossible in legitimate code; kick immediately.
		FailBuffer:    1,
		MaxBuffer:     1,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// contradictoryPairs lists flag pairs that cannot legitimately both be set in
// the same PlayerAuthInput. Each entry is [startFlag, stopFlag]. The bit values
// are of type int matching protocol.Bitset.Load(int) bool.
var contradictoryPairs = [][2]int{
	{packet.InputFlagStartSprinting, packet.InputFlagStopSprinting},
	{packet.InputFlagStartSneaking, packet.InputFlagStopSneaking},
	{packet.InputFlagStartSwimming, packet.InputFlagStopSwimming},
	{packet.InputFlagStartGliding, packet.InputFlagStopGliding},
	{packet.InputFlagStartCrawling, packet.InputFlagStopCrawling},
}

// contradictoryPairNames is a parallel slice of human-readable names for the
// contradictory pairs, used in violation log messages.
var contradictoryPairNames = []string{
	"start+stop_sprint",
	"start+stop_sneak",
	"start+stop_swim",
	"start+stop_glide",
	"start+stop_crawl",
}

// Check evaluates the input flags for contradictory start+stop combinations.
// input is the InputData field from the PlayerAuthInput packet.
func (c *BadPacketECheck) Check(input interface{ Load(bit int) bool }) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	var violations []string
	for i, pair := range contradictoryPairs {
		if input.Load(pair[0]) && input.Load(pair[1]) {
			violations = append(violations, contradictoryPairNames[i])
		}
	}
	if len(violations) > 0 {
		return true, strings.Join(violations, ",")
	}
	return false, ""
}
