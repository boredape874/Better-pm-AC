package login

import (
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
)

func TestProtocolA_KnownProtocol(t *testing.T) {
	cfg := config.ProtocolConfig{Enabled: true, Policy: "kick", Violations: 1}
	chk := NewProtocolACheck(cfg)

	ld := data.LoginData{Protocol: 671} // known: 1.21.0
	if f, info := chk.Check(ld); f {
		t.Errorf("should not flag known protocol 671 (info=%q)", info)
	}
}

func TestProtocolA_UnknownProtocol(t *testing.T) {
	cfg := config.ProtocolConfig{Enabled: true, Policy: "kick", Violations: 1}
	chk := NewProtocolACheck(cfg)

	ld := data.LoginData{Protocol: 9999} // obviously unknown
	if f, _ := chk.Check(ld); !f {
		t.Error("should flag unknown protocol 9999")
	}
}

func TestProtocolA_Allowlist(t *testing.T) {
	cfg := config.ProtocolConfig{
		Enabled:         true,
		Policy:          "kick",
		AllowedVersions: []int32{671, 662},
		Violations:      1,
	}
	chk := NewProtocolACheck(cfg)

	// 671 is in allowlist → no flag.
	if f, _ := chk.Check(data.LoginData{Protocol: 671}); f {
		t.Error("671 is in allowlist, should not flag")
	}
	// 649 is not in allowlist → flag.
	if f, _ := chk.Check(data.LoginData{Protocol: 649}); !f {
		t.Error("649 is not in allowlist, should flag")
	}
}

func TestProtocolA_Disabled(t *testing.T) {
	cfg := config.ProtocolConfig{Enabled: false, Policy: "kick", Violations: 1}
	chk := NewProtocolACheck(cfg)

	if f, _ := chk.Check(data.LoginData{Protocol: 1}); f {
		t.Error("should not flag when disabled")
	}
}
