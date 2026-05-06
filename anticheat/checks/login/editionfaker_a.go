package login

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// protocolVersionRange maps a MCBE protocol number to the expected
// [major, minMinor, maxMinor] of the GameVersion string.
// e.g. protocol 671 → "1.21.x" means major=1, minor must be 21.
type versionRange struct {
	Major    int
	MinMinor int
	MaxMinor int
}

// protocolToVersion maps known protocol numbers to their expected version range.
// MinMinor and MaxMinor are the minimum and maximum accepted minor version
// numbers for the GameVersion string "major.minor.patch". For protocols that
// map to a single minor version, MinMinor == MaxMinor.
var protocolToVersion = map[int]versionRange{
	748: {1, 21, 21}, // 1.21.80+  (still minor=21)
	729: {1, 21, 21}, // 1.21.60
	712: {1, 21, 21}, // 1.21.40
	685: {1, 21, 21}, // 1.21.20
	671: {1, 21, 21}, // 1.21.0
	662: {1, 20, 20}, // 1.20.80
	649: {1, 20, 20}, // 1.20.60
	630: {1, 20, 20}, // 1.20.40
	622: {1, 20, 20}, // 1.20.30
	618: {1, 20, 20}, // 1.20.10
	594: {1, 20, 20}, // 1.20.0
	575: {1, 19, 19}, // 1.19.80
	560: {1, 19, 19}, // 1.19.60
	554: {1, 19, 19}, // 1.19.50
	544: {1, 19, 19}, // 1.19.40
	534: {1, 19, 19}, // 1.19.30
	527: {1, 19, 19}, // 1.19.20
	503: {1, 19, 19}, // 1.19.10
}

// EditionFakerACheck flags when the advertised GameVersion doesn't match
// the protocol number's expected version range.
// Implements anticheat.Detection.
type EditionFakerACheck struct {
	cfg config.EditionFakerConfig
}

func NewEditionFakerACheck(cfg config.EditionFakerConfig) *EditionFakerACheck {
	return &EditionFakerACheck{cfg: cfg}
}

func (*EditionFakerACheck) Type() string    { return "EditionFaker" }
func (*EditionFakerACheck) SubType() string { return "A" }
func (*EditionFakerACheck) Description() string {
	return "Flags logins where GameVersion doesn't match the claimed protocol number."
}
func (*EditionFakerACheck) Punishable() bool { return true }
func (c *EditionFakerACheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *EditionFakerACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    1,
		MaxBuffer:     1,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// parseGameVersion parses "major.minor.patch" into (major, minor, ok).
func parseGameVersion(v string) (major, minor int, ok bool) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return maj, min, true
}

// Check evaluates the protocol / game version pair. Returns (flagged, info).
func (c *EditionFakerACheck) Check(ld data.LoginData) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	vr, known := protocolToVersion[ld.Protocol]
	if !known {
		// Unknown protocol: let Protocol/A handle it; skip here.
		return false, ""
	}

	major, minor, ok := parseGameVersion(ld.GameVersion)
	if !ok {
		return true, fmt.Sprintf("unparseable_game_version=%q", ld.GameVersion)
	}

	// Allow minor to be in the range [MinMinor, MaxMinor]. For a single-value
	// range (MinMinor==MaxMinor) we also allow up to +9 minor increments to
	// cover patch releases within the same minor version family.
	effectiveMax := vr.MaxMinor
	if effectiveMax == vr.MinMinor {
		effectiveMax = vr.MinMinor + 9
	}
	if major != vr.Major || minor < vr.MinMinor || minor > effectiveMax {
		return true, fmt.Sprintf("protocol=%d expected_major=%d minor_range=%d..%d got=%d.%d",
			ld.Protocol, vr.Major, vr.MinMinor, effectiveMax, major, minor)
	}
	return false, ""
}
