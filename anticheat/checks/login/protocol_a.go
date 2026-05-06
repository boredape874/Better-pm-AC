package login

import (
	"fmt"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// knownProtocols is the set of known/accepted MCBE protocol versions.
var knownProtocols = map[int]bool{
	748: true, // 1.21.80+
	729: true, // 1.21.60
	712: true, // 1.21.40
	685: true, // 1.21.20
	671: true, // 1.21.0 / 1.21.x
	662: true, // 1.20.80
	649: true, // 1.20.60
	630: true, // 1.20.40
	622: true, // 1.20.30
	618: true, // 1.20.10
	594: true, // 1.20.0
	575: true, // 1.19.80
	560: true, // 1.19.60
	554: true, // 1.19.50
	544: true, // 1.19.40
	534: true, // 1.19.30
	527: true, // 1.19.20
	503: true, // 1.19.10
}

// ProtocolACheck flags when the client connects with an unknown protocol version.
// Implements anticheat.Detection.
type ProtocolACheck struct {
	cfg config.ProtocolConfig
}

func NewProtocolACheck(cfg config.ProtocolConfig) *ProtocolACheck {
	return &ProtocolACheck{cfg: cfg}
}

func (*ProtocolACheck) Type() string    { return "Protocol" }
func (*ProtocolACheck) SubType() string { return "A" }
func (*ProtocolACheck) Description() string {
	return "Flags logins from unknown or unsupported MCBE protocol versions."
}
func (*ProtocolACheck) Punishable() bool { return true }
func (c *ProtocolACheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *ProtocolACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    1,
		MaxBuffer:     1,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates the protocol version in ld. Returns (flagged, info).
func (c *ProtocolACheck) Check(ld data.LoginData) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}
	// If operator has configured an explicit allowlist, use that.
	if len(c.cfg.AllowedVersions) > 0 {
		for _, v := range c.cfg.AllowedVersions {
			if int32(ld.Protocol) == v {
				return false, ""
			}
		}
		return true, fmt.Sprintf("protocol=%d not_in_allowlist", ld.Protocol)
	}
	// Otherwise use the built-in known-protocol table.
	if !knownProtocols[ld.Protocol] {
		return true, fmt.Sprintf("protocol=%d unknown", ld.Protocol)
	}
	return false, ""
}
