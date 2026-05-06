package login

import (
	"fmt"
	"strings"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/anticheat/meta"
	"github.com/boredape874/Better-pm-AC/config"
)

// knownCheats is a list of known cheat-client signatures found in DeviceModel.
var knownCheats = []string{"Horion", "Zephyr", "Phantom", "XClient", "Nuker"}

// ClientSpoofACheck flags known cheat-client signatures in DeviceModel or
// abnormal ClientRandomID values. Implements anticheat.Detection.
type ClientSpoofACheck struct {
	cfg config.ClientSpoofConfig
}

func NewClientSpoofACheck(cfg config.ClientSpoofConfig) *ClientSpoofACheck {
	return &ClientSpoofACheck{cfg: cfg}
}

func (*ClientSpoofACheck) Type() string    { return "ClientSpoof" }
func (*ClientSpoofACheck) SubType() string { return "A" }
func (*ClientSpoofACheck) Description() string {
	return "Flags known cheat-client device models and zero ClientRandomID."
}
func (*ClientSpoofACheck) Punishable() bool { return true }
func (c *ClientSpoofACheck) Policy() meta.MitigatePolicy { return meta.ParsePolicy(c.cfg.Policy) }

func (c *ClientSpoofACheck) DefaultMetadata() *meta.DetectionMetadata {
	return &meta.DetectionMetadata{
		FailBuffer:    1,
		MaxBuffer:     1,
		MaxViolations: float64(c.cfg.Violations),
	}
}

// Check evaluates the login data for cheat-client signatures. Returns (flagged, info).
func (c *ClientSpoofACheck) Check(ld data.LoginData) (bool, string) {
	if !c.cfg.Enabled {
		return false, ""
	}

	// Check DeviceModel for known cheat signatures (case-insensitive).
	lower := strings.ToLower(ld.DeviceModel)
	for _, cheat := range knownCheats {
		if strings.Contains(lower, strings.ToLower(cheat)) {
			return true, fmt.Sprintf("device_model=%q matches_cheat=%q", ld.DeviceModel, cheat)
		}
	}

	// Flag zero ClientRandomID — legitimate Bedrock clients always generate a
	// non-zero random ID during installation.
	if ld.ClientRandomID == 0 {
		return true, "client_random_id=0 (spoofed or missing)"
	}

	return false, ""
}
