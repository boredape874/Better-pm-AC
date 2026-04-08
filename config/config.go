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
Speed       SpeedConfig       `toml:"speed"`
Fly         FlyConfig         `toml:"fly"`
NoFall      NoFallConfig      `toml:"nofall"`
Reach       ReachConfig       `toml:"reach"`
KillAura    KillAuraConfig    `toml:"killaura"`
AutoClicker AutoClickerConfig `toml:"autoclicker"`
Aim         AimConfig         `toml:"aim"`
BadPacket   BadPacketConfig   `toml:"badpacket"`
}

// SpeedConfig configures the Speed/A check.
type SpeedConfig struct {
Enabled    bool    `toml:"enabled"`
MaxSpeed   float64 `toml:"max_speed"`   // blocks/tick at 20 TPS
Violations int     `toml:"violations"`  // kicks at this VL
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

// ReachConfig configures the Reach/A check.
type ReachConfig struct {
Enabled    bool    `toml:"enabled"`
MaxReach   float64 `toml:"max_reach"`  // blocks
Violations int     `toml:"violations"`
}

// KillAuraConfig configures the KillAura/A check.
type KillAuraConfig struct {
Enabled    bool `toml:"enabled"`
Violations int  `toml:"violations"`
}

// AutoClickerConfig configures the AutoClicker/A check.
type AutoClickerConfig struct {
Enabled    bool `toml:"enabled"`
MaxCPS     int  `toml:"max_cps"`    // clicks per second limit
Violations int  `toml:"violations"`
}

// AimConfig configures the Aim/A (rounded-yaw) check.
type AimConfig struct {
Enabled    bool `toml:"enabled"`
Violations int  `toml:"violations"`
}

// BadPacketConfig configures the BadPacket/A check.
type BadPacketConfig struct {
Enabled    bool `toml:"enabled"`
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
Speed:   SpeedConfig{Enabled: true, MaxSpeed: 0.7, Violations: 10},
Fly:     FlyConfig{Enabled: true, Violations: 5},
NoFall:  NoFallConfig{Enabled: true, Violations: 5},
Reach:   ReachConfig{Enabled: true, MaxReach: 3.1, Violations: 7},
KillAura: KillAuraConfig{Enabled: true, Violations: 1},
AutoClicker: AutoClickerConfig{Enabled: true, MaxCPS: 20, Violations: 20},
Aim:      AimConfig{Enabled: true, Violations: 20},
BadPacket: BadPacketConfig{Enabled: true, Violations: 1},
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
