// Package config defines all configuration types for Better-pm-AC and
// handles loading/saving a TOML file.
package config

import (
	"os"

	"github.com/pelletier/go-toml"
)

// Config is the root configuration structure.
type Config struct {
	Proxy     ProxyConfig     `toml:"proxy"`
	Anticheat AnticheatConfig `toml:"anticheat"`
}

// ProxyConfig holds the network addresses for the MiTM proxy.
type ProxyConfig struct {
	// ListenAddr is the address the proxy binds on for incoming Bedrock clients.
	ListenAddr string `toml:"listen_addr"`
	// RemoteAddr is the address of the downstream PMMP server.
	RemoteAddr string `toml:"remote_addr"`
}

// AnticheatConfig groups all check configurations.
type AnticheatConfig struct {
	Speed        SpeedConfig        `toml:"speed"`
	SpeedB       SpeedBConfig       `toml:"speed_b"`
	Fly          FlyConfig          `toml:"fly"`
	NoFall       NoFallConfig       `toml:"nofall"`
	NoFallB      NoFallBConfig      `toml:"nofall_b"`
	Phase        PhaseAConfig       `toml:"phase"`
	Reach        ReachConfig        `toml:"reach"`
	KillAura     KillAuraConfig     `toml:"killaura"`
	KillAuraB    KillAuraBConfig    `toml:"killaura_b"`
	KillAuraC    KillAuraCConfig    `toml:"killaura_c"`
	AutoClicker  AutoClickerConfig  `toml:"autoclicker"`
	AutoClickerB AutoClickerBConfig `toml:"autoclicker_b"`
	Aim          AimConfig          `toml:"aim"`
	AimB         AimBConfig         `toml:"aim_b"`
	BadPacket    BadPacketConfig    `toml:"badpacket"`
	BadPacketB   BadPacketBConfig   `toml:"badpacket_b"`
	BadPacketC   BadPacketCConfig   `toml:"badpacket_c"`
	BadPacketD   BadPacketDConfig   `toml:"badpacket_d"`
	Timer        TimerConfig        `toml:"timer"`
}

// SpeedConfig configures the Speed/A check.
type SpeedConfig struct {
	Enabled    bool    `toml:"enabled"`
	MaxSpeed   float64 `toml:"max_speed"`  // blocks/tick at 20 TPS
	Violations int     `toml:"violations"` // kicks at this VL
}

// SpeedBConfig configures the Speed/B check (aerial horizontal speed).
type SpeedBConfig struct {
	Enabled    bool    `toml:"enabled"`
	MaxSpeed   float64 `toml:"max_speed"`  // blocks/tick (same scale as Speed/A)
	Violations int     `toml:"violations"`
}

// FlyConfig configures the Fly/A check.
type FlyConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// NoFallConfig configures the NoFall/A check.
type NoFallConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// NoFallBConfig configures the NoFall/B check (persistent OnGround spoof).
type NoFallBConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// PhaseAConfig configures the Phase/A check (impossible position jump).
type PhaseAConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// ReachConfig configures the Reach/A check.
type ReachConfig struct {
	Enabled    bool    `toml:"enabled"`
	MaxReach   float64 `toml:"max_reach"` // blocks
	Violations int     `toml:"violations"`
}

// KillAuraConfig configures the KillAura/A check.
type KillAuraConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// KillAuraBConfig configures the KillAura/B check (angle-based FOV detection).
type KillAuraBConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// KillAuraCConfig configures the KillAura/C check (multi-target per-tick).
type KillAuraCConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// AutoClickerConfig configures the AutoClicker/A check.
type AutoClickerConfig struct {
	Enabled    bool `toml:"enabled"`
	MaxCPS     int  `toml:"max_cps"` // clicks per second limit
	Violations int  `toml:"violations"`
}

// AutoClickerBConfig configures the AutoClicker/B check (click interval consistency).
type AutoClickerBConfig struct {
	Enabled          bool    `toml:"enabled"`
	StdDevThreshold  float64 `toml:"std_dev_threshold_ms"` // ms; below this value is suspicious
	MinSamples       int     `toml:"min_samples"`          // minimum interval samples before flagging
	Violations       int     `toml:"violations"`
}

// AimConfig configures the Aim/A (rounded-yaw) check.
type AimConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// AimBConfig configures the Aim/B check (constant pitch during yaw rotation).
type AimBConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// BadPacketConfig configures the BadPacket/A check.
type BadPacketConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// BadPacketBConfig configures the BadPacket/B check (pitch range validation).
type BadPacketBConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// BadPacketCConfig configures the BadPacket/C check (sprint+sneak simultaneously).
type BadPacketCConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// BadPacketDConfig configures the BadPacket/D check (NaN/Infinity position).
type BadPacketDConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// TimerConfig configures the Timer/A check.
// MaxRatePS is the maximum number of PlayerAuthInput packets allowed per second.
// At 20 TPS the expected rate is exactly 20; 25 gives a 25% tolerance for
// server-side jitter while reliably catching Timer hacks (≥ 1.25×).
type TimerConfig struct {
	Enabled    bool `toml:"enabled"`
	MaxRatePS  int  `toml:"max_rate_ps"`
	Violations int  `toml:"violations"`
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	return Config{
		Proxy: ProxyConfig{
			ListenAddr: "0.0.0.0:19132",
			RemoteAddr: "127.0.0.1:19133",
		},
		Anticheat: AnticheatConfig{
			Speed:        SpeedConfig{Enabled: true, MaxSpeed: 0.4, Violations: 10},
			SpeedB:       SpeedBConfig{Enabled: true, MaxSpeed: 0.4, Violations: 10},
			Fly:          FlyConfig{Enabled: true, Violations: 5},
			NoFall:       NoFallConfig{Enabled: true, Violations: 5},
			NoFallB:      NoFallBConfig{Enabled: true, Violations: 5},
			Phase:        PhaseAConfig{Enabled: true, Violations: 3},
			Reach:        ReachConfig{Enabled: true, MaxReach: 3.1, Violations: 7},
			KillAura:     KillAuraConfig{Enabled: true, Violations: 1},
			KillAuraB:    KillAuraBConfig{Enabled: true, Violations: 5},
			KillAuraC:    KillAuraCConfig{Enabled: true, Violations: 3},
			AutoClicker:  AutoClickerConfig{Enabled: true, MaxCPS: 20, Violations: 20},
			AutoClickerB: AutoClickerBConfig{Enabled: true, StdDevThreshold: 5.0, MinSamples: 8, Violations: 15},
			Aim:          AimConfig{Enabled: true, Violations: 20},
			AimB:         AimBConfig{Enabled: true, Violations: 10},
			BadPacket:    BadPacketConfig{Enabled: true, Violations: 1},
			BadPacketB:   BadPacketBConfig{Enabled: true, Violations: 1},
			BadPacketC:   BadPacketCConfig{Enabled: true, Violations: 1},
			BadPacketD:   BadPacketDConfig{Enabled: true, Violations: 1},
			Timer:        TimerConfig{Enabled: true, MaxRatePS: 22, Violations: 5},
		},
	}
}

// Load reads config from path, creating the file with defaults if absent.
func Load(path string) (Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := Default()
		if err := save(path, cfg); err != nil {
			return cfg, err
		}
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func save(path string, cfg Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
