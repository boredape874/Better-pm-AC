package config

import (
	"os"

	"github.com/pelletier/go-toml"
)

// Config holds the full configuration for Better-pm-AC.
type Config struct {
	Proxy    ProxyConfig    `toml:"proxy"`
	Anticheat AnticheatConfig `toml:"anticheat"`
}

// ProxyConfig defines listener and upstream addresses.
type ProxyConfig struct {
	// ListenAddr is the address the proxy listens on for incoming client connections.
	// Defaults to "0.0.0.0:19132"
	ListenAddr string `toml:"listen_addr"`
	// RemoteAddr is the address of the downstream PMMP server.
	// Defaults to "127.0.0.1:19133"
	RemoteAddr string `toml:"remote_addr"`
}

// AnticheatConfig holds toggles and thresholds for each check.
type AnticheatConfig struct {
	Speed   SpeedConfig   `toml:"speed"`
	Fly     FlyConfig     `toml:"fly"`
	NoFall  NoFallConfig  `toml:"nofall"`
	Reach   ReachConfig   `toml:"reach"`
	KillAura KillAuraConfig `toml:"killaura"`
}

type SpeedConfig struct {
	Enabled   bool    `toml:"enabled"`
	MaxSpeed  float64 `toml:"max_speed"`  // blocks per tick
	Violations int    `toml:"violations"` // violations before kick
}

type FlyConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

type NoFallConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

type ReachConfig struct {
	Enabled    bool    `toml:"enabled"`
	MaxReach   float64 `toml:"max_reach"` // blocks
	Violations int     `toml:"violations"`
}

type KillAuraConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// DefaultConfig returns a Config populated with safe defaults.
func DefaultConfig() Config {
	return Config{
		Proxy: ProxyConfig{
			ListenAddr: "0.0.0.0:19132",
			RemoteAddr: "127.0.0.1:19133",
		},
		Anticheat: AnticheatConfig{
			Speed: SpeedConfig{
				Enabled:    true,
				MaxSpeed:   0.7,
				Violations: 10,
			},
			Fly: FlyConfig{
				Enabled:    true,
				Violations: 5,
			},
			NoFall: NoFallConfig{
				Enabled:    true,
				Violations: 5,
			},
			Reach: ReachConfig{
				Enabled:    true,
				MaxReach:   3.1,
				Violations: 5,
			},
			KillAura: KillAuraConfig{
				Enabled:    true,
				Violations: 5,
			},
		},
	}
}

// Load reads a TOML config from path. If the file does not exist, the default
// config is returned and written to path.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if writeErr := Write(path, cfg); writeErr != nil {
				return cfg, writeErr
			}
			return cfg, nil
		}
		return cfg, err
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Write serialises cfg to TOML and writes it to path.
func Write(path string, cfg Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
