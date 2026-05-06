package login

import (
	"testing"

	"github.com/boredape874/Better-pm-AC/anticheat/data"
	"github.com/boredape874/Better-pm-AC/config"
)

func defaultEditionFakerCfg() config.EditionFakerConfig {
	return config.EditionFakerConfig{Enabled: true, Policy: "kick", Violations: 1}
}

func TestEditionFakerA_CorrectVersion(t *testing.T) {
	chk := NewEditionFakerACheck(defaultEditionFakerCfg())
	ld := data.LoginData{Protocol: 671, GameVersion: "1.21.0"}
	if f, info := chk.Check(ld); f {
		t.Errorf("should not flag matching version (info=%q)", info)
	}
}

func TestEditionFakerA_WrongMajor(t *testing.T) {
	chk := NewEditionFakerACheck(defaultEditionFakerCfg())
	ld := data.LoginData{Protocol: 671, GameVersion: "2.0.0"}
	if f, _ := chk.Check(ld); !f {
		t.Error("should flag when major version mismatches")
	}
}

func TestEditionFakerA_WrongMinor(t *testing.T) {
	chk := NewEditionFakerACheck(defaultEditionFakerCfg())
	// Protocol 671 expects 1.21.x; claiming 1.19.x is a mismatch.
	ld := data.LoginData{Protocol: 671, GameVersion: "1.19.50"}
	if f, _ := chk.Check(ld); !f {
		t.Error("should flag when minor version is below expected range")
	}
}

func TestEditionFakerA_UnparsableVersion(t *testing.T) {
	chk := NewEditionFakerACheck(defaultEditionFakerCfg())
	ld := data.LoginData{Protocol: 671, GameVersion: "not-a-version"}
	if f, _ := chk.Check(ld); !f {
		t.Error("should flag when game version cannot be parsed")
	}
}

func TestEditionFakerA_UnknownProtocol(t *testing.T) {
	chk := NewEditionFakerACheck(defaultEditionFakerCfg())
	// Unknown protocol: EditionFaker should skip (Protocol/A handles it).
	ld := data.LoginData{Protocol: 9999, GameVersion: "1.21.0"}
	if f, _ := chk.Check(ld); f {
		t.Error("should not flag unknown protocol (deferred to Protocol/A)")
	}
}

func TestEditionFakerA_Disabled(t *testing.T) {
	cfg := config.EditionFakerConfig{Enabled: false, Violations: 1}
	chk := NewEditionFakerACheck(cfg)
	ld := data.LoginData{Protocol: 671, GameVersion: "2.0.0"}
	if f, _ := chk.Check(ld); f {
		t.Error("should not flag when disabled")
	}
}
