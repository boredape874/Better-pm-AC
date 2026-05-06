package login

import (
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
)

func defaultClientSpoofCfg() config.ClientSpoofConfig {
	return config.ClientSpoofConfig{Enabled: true, Policy: "kick", Violations: 1}
}

func TestClientSpoofA_KnownCheatModel(t *testing.T) {
	chk := NewClientSpoofACheck(defaultClientSpoofCfg())
	ld := data.LoginData{DeviceModel: "Horion_Client_v1.0", ClientRandomID: 12345}
	if f, _ := chk.Check(ld); !f {
		t.Error("should flag device model containing 'Horion'")
	}
}

func TestClientSpoofA_CaseInsensitive(t *testing.T) {
	chk := NewClientSpoofACheck(defaultClientSpoofCfg())
	ld := data.LoginData{DeviceModel: "phantom-mod", ClientRandomID: 99999}
	if f, _ := chk.Check(ld); !f {
		t.Error("should flag case-insensitive match for 'phantom'")
	}
}

func TestClientSpoofA_ZeroRandomID(t *testing.T) {
	chk := NewClientSpoofACheck(defaultClientSpoofCfg())
	ld := data.LoginData{DeviceModel: "Samsung SM-G991B", ClientRandomID: 0}
	if f, _ := chk.Check(ld); !f {
		t.Error("should flag ClientRandomID == 0")
	}
}

func TestClientSpoofA_LegitClient(t *testing.T) {
	chk := NewClientSpoofACheck(defaultClientSpoofCfg())
	ld := data.LoginData{DeviceModel: "Samsung SM-G991B", ClientRandomID: 1234567890}
	if f, info := chk.Check(ld); f {
		t.Errorf("should not flag legitimate client (info=%q)", info)
	}
}

func TestClientSpoofA_Disabled(t *testing.T) {
	cfg := config.ClientSpoofConfig{Enabled: false, Violations: 1}
	chk := NewClientSpoofACheck(cfg)
	ld := data.LoginData{DeviceModel: "Horion", ClientRandomID: 0}
	if f, _ := chk.Check(ld); f {
		t.Error("should not flag when disabled")
	}
}
