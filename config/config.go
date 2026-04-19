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

// SimulationConfig tunes the server-authoritative physics replay.
//
// ToleranceSpeedXZ / ToleranceFlyY set how far the client-reported position may
// drift from the replayed position before a movement check flags. The physics
// constants mirror Bedrock defaults and should only be changed if Mojang alters
// them in an update.
type SimulationConfig struct {
	ToleranceSpeedXZ float64 `toml:"tolerance_speed_xz"`
	ToleranceFlyY    float64 `toml:"tolerance_fly_y"`
	StepHeight       float64 `toml:"step_height"`
	GravityAccel     float64 `toml:"gravity_accel"`
	AirDrag          float64 `toml:"air_drag"`
}

// WorldConfig tunes the chunk cache backing WorldTracker.
type WorldConfig struct {
	MaxChunksPerPlayer int `toml:"max_chunks_per_player"`
	ChunkCacheTTLTicks int `toml:"chunk_cache_ttl_ticks"`
}

// EntityConfig tunes entity rewind.
//
// RewindWindowTicks is the normal lag-compensation window. RewindMaxTicks is
// the hard cap; players whose latency exceeds it fall back to no-rewind combat
// checks (see basicengine92-tech:feat/lag-comp-cutoff).
type EntityConfig struct {
	RewindWindowTicks int `toml:"rewind_window_ticks"`
	RewindMaxTicks    int `toml:"rewind_max_ticks"`
}

// AckConfig tunes the NetworkStackLatency marker system. MarkerTimeoutTicks is
// how long a pending callback waits for a client echo before being dropped.
type AckConfig struct {
	MarkerTimeoutTicks int `toml:"marker_timeout_ticks"`
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
	LogViolations bool   `toml:"log_violations"`
	LogLevel      string `toml:"log_level"`

	Simulation SimulationConfig `toml:"simulation"`
	World      WorldConfig      `toml:"world"`
	Entity     EntityConfig     `toml:"entity"`
	Ack        AckConfig        `toml:"ack"`

	Speed        SpeedConfig        `toml:"speed"`
	SpeedB       SpeedBConfig       `toml:"speed_b"`
	Fly          FlyConfig          `toml:"fly"`
	FlyB         FlyBConfig         `toml:"fly_b"`
	FlyC         FlyCConfig         `toml:"fly_c"`
	NoFall       NoFallConfig       `toml:"nofall"`
	NoFallB      NoFallBConfig      `toml:"nofall_b"`
	NoSlow       NoSlowConfig       `toml:"noslow"`
	Phase        PhaseAConfig       `toml:"phase"`
	Step         StepConfig         `toml:"step"`
	HighJump     HighJumpConfig     `toml:"highjump"`
	Jesus        JesusConfig        `toml:"jesus"`
	Spider       SpiderConfig       `toml:"spider"`
	InvalidMove  InvalidMoveConfig  `toml:"invalidmove"`
	Reach        ReachConfig        `toml:"reach"`
	ReachB       ReachBConfig       `toml:"reach_b"`
	KillAura     KillAuraConfig     `toml:"killaura"`
	KillAuraB    KillAuraBConfig    `toml:"killaura_b"`
	KillAuraC    KillAuraCConfig    `toml:"killaura_c"`
	KillAuraD    KillAuraDConfig    `toml:"killaura_d"`
	AutoClicker  AutoClickerConfig  `toml:"autoclicker"`
	AutoClickerB AutoClickerBConfig `toml:"autoclicker_b"`
	AutoClickerC AutoClickerCConfig `toml:"autoclicker_c"`
	Aim          AimConfig          `toml:"aim"`
	AimB         AimBConfig         `toml:"aim_b"`
	BadPacket    BadPacketConfig    `toml:"badpacket"`
	BadPacketB   BadPacketBConfig   `toml:"badpacket_b"`
	BadPacketC   BadPacketCConfig   `toml:"badpacket_c"`
	BadPacketD   BadPacketDConfig   `toml:"badpacket_d"`
	BadPacketE   BadPacketEConfig   `toml:"badpacket_e"`
	BadPacketF   BadPacketFConfig   `toml:"badpacket_f"`
	BadPacketG   BadPacketGConfig   `toml:"badpacket_g"`
	Scaffold     ScaffoldConfig     `toml:"scaffold"`
	Nuker        NukerConfig        `toml:"nuker"`
	NukerB       NukerBConfig       `toml:"nuker_b"`
	FastBreak    FastBreakConfig    `toml:"fastbreak"`
	FastPlace    FastPlaceConfig    `toml:"fastplace"`
	Tower        TowerConfig        `toml:"tower"`
	InvalidBreak InvalidBreakConfig `toml:"invalidbreak"`
	EditionFaker EditionFakerConfig `toml:"editionfaker"`
	ClientSpoof  ClientSpoofConfig  `toml:"clientspoof"`
	Protocol     ProtocolConfig     `toml:"protocol"`
	Timer        TimerConfig        `toml:"timer"`
	Velocity     VelocityConfig     `toml:"velocity"`
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

// VelocityConfig configures the Velocity/A check (Anti-KB detection).
type VelocityConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// NoSlowConfig configures the NoSlow/A check.
// MaxItemUseSpeed is the maximum horizontal speed (blocks/tick) allowed while
// the player is actively using an item (eating, drawing a bow, blocking).
// Vanilla item-use speed is ~27% of base walking speed; 0.21 b/tick is a
// generous ceiling that prevents false positives on laggy clients.
type NoSlowConfig struct {
	Enabled         bool    `toml:"enabled"`
	MaxItemUseSpeed float64 `toml:"max_item_use_speed"` // blocks/tick, default 0.21
	Violations      int     `toml:"violations"`
}

// ScaffoldConfig configures the Scaffold/A check.
type ScaffoldConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// BadPacketEConfig configures the BadPacket/E check.
type BadPacketEConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// FlyBConfig configures the Fly/B gravity-bypass check.
type FlyBConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// FlyCConfig configures the Fly/C liquid-fly check.
type FlyCConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// StepConfig configures the Step/A check (client step > StepHeight).
type StepConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// HighJumpConfig configures the HighJump/A check.
type HighJumpConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// JesusConfig configures the Jesus/A water-walking check.
type JesusConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// SpiderConfig configures the Spider/A wall-climb check.
type SpiderConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// InvalidMoveConfig configures the InvalidMove/A yaw-snap check.
type InvalidMoveConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// ReachBConfig configures the Reach/B raycast-based reach check.
type ReachBConfig struct {
	Enabled    bool    `toml:"enabled"`
	MaxReach   float64 `toml:"max_reach"`
	Violations int     `toml:"violations"`
}

// KillAuraDConfig configures the KillAura/D yaw-snap-before-attack check.
type KillAuraDConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// AutoClickerCConfig configures the AutoClicker/C double-click pattern check.
type AutoClickerCConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// BadPacketFConfig configures the BadPacket/F missing-flag check.
type BadPacketFConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// BadPacketGConfig configures the BadPacket/G packet-order check.
type BadPacketGConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// NukerConfig configures the Nuker/A multi-break-per-tick check.
type NukerConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// NukerBConfig configures the Nuker/B angular-range break check.
type NukerBConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// FastBreakConfig configures the FastBreak/A check comparing break time to block hardness.
type FastBreakConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// FastPlaceConfig configures the FastPlace/A blocks-per-second check.
type FastPlaceConfig struct {
	Enabled    bool `toml:"enabled"`
	MaxBPS     int  `toml:"max_bps"`
	Violations int  `toml:"violations"`
}

// TowerConfig configures the Tower/A self-tower check.
type TowerConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// InvalidBreakConfig configures the InvalidBreak/A raycast-fail check.
type InvalidBreakConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// EditionFakerConfig configures the EditionFaker/A check.
// AllowedTitleIDs is the allow-list of legitimate Bedrock client title IDs;
// logins outside this list are treated as spoofed Java clients.
type EditionFakerConfig struct {
	Enabled         bool    `toml:"enabled"`
	AllowedTitleIDs []int64 `toml:"allowed_title_ids"`
	Violations      int     `toml:"violations"`
}

// ClientSpoofConfig configures the ClientSpoof/A check (DeviceOS vs TitleID mismatch).
type ClientSpoofConfig struct {
	Enabled    bool `toml:"enabled"`
	Violations int  `toml:"violations"`
}

// ProtocolConfig configures the Protocol/A check.
// AllowedVersions is the allow-list of accepted Bedrock protocol versions.
type ProtocolConfig struct {
	Enabled         bool    `toml:"enabled"`
	AllowedVersions []int32 `toml:"allowed_versions"`
	Violations      int     `toml:"violations"`
}
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
			LogViolations: true,
			LogLevel:      "info",
			Simulation: SimulationConfig{
				ToleranceSpeedXZ: 0.02,
				ToleranceFlyY:    0.01,
				StepHeight:       0.6,
				GravityAccel:     0.08,
				AirDrag:          0.98,
			},
			World: WorldConfig{
				MaxChunksPerPlayer: 400,
				ChunkCacheTTLTicks: 6000,
			},
			Entity: EntityConfig{
				RewindWindowTicks: 40,
				RewindMaxTicks:    80,
			},
			Ack: AckConfig{
				MarkerTimeoutTicks: 100,
			},

			Speed:        SpeedConfig{Enabled: true, MaxSpeed: 0.4, Violations: 10},
			SpeedB:       SpeedBConfig{Enabled: true, MaxSpeed: 0.4, Violations: 10},
			Fly:          FlyConfig{Enabled: true, Violations: 5},
			FlyB:         FlyBConfig{Enabled: true, Violations: 5},
			FlyC:         FlyCConfig{Enabled: true, Violations: 5},
			NoFall:       NoFallConfig{Enabled: true, Violations: 5},
			NoFallB:      NoFallBConfig{Enabled: true, Violations: 5},
			NoSlow:       NoSlowConfig{Enabled: true, MaxItemUseSpeed: 0.21, Violations: 8},
			Phase:        PhaseAConfig{Enabled: true, Violations: 3},
			Step:         StepConfig{Enabled: true, Violations: 3},
			HighJump:     HighJumpConfig{Enabled: true, Violations: 3},
			Jesus:        JesusConfig{Enabled: true, Violations: 5},
			Spider:       SpiderConfig{Enabled: true, Violations: 5},
			InvalidMove:  InvalidMoveConfig{Enabled: true, Violations: 1},
			Reach:        ReachConfig{Enabled: true, MaxReach: 3.1, Violations: 7},
			ReachB:       ReachBConfig{Enabled: true, MaxReach: 3.1, Violations: 5},
			KillAura:     KillAuraConfig{Enabled: true, Violations: 1},
			KillAuraB:    KillAuraBConfig{Enabled: true, Violations: 5},
			KillAuraC:    KillAuraCConfig{Enabled: true, Violations: 3},
			KillAuraD:    KillAuraDConfig{Enabled: true, Violations: 3},
			AutoClicker:  AutoClickerConfig{Enabled: true, MaxCPS: 20, Violations: 20},
			AutoClickerB: AutoClickerBConfig{Enabled: true, StdDevThreshold: 5.0, MinSamples: 8, Violations: 15},
			AutoClickerC: AutoClickerCConfig{Enabled: true, Violations: 10},
			Aim:          AimConfig{Enabled: true, Violations: 20},
			AimB:         AimBConfig{Enabled: true, Violations: 10},
			BadPacket:    BadPacketConfig{Enabled: true, Violations: 1},
			BadPacketB:   BadPacketBConfig{Enabled: true, Violations: 1},
			BadPacketC:   BadPacketCConfig{Enabled: true, Violations: 1},
			BadPacketD:   BadPacketDConfig{Enabled: true, Violations: 1},
			BadPacketE:   BadPacketEConfig{Enabled: true, Violations: 1},
			BadPacketF:   BadPacketFConfig{Enabled: true, Violations: 3},
			BadPacketG:   BadPacketGConfig{Enabled: true, Violations: 1},
			Scaffold:     ScaffoldConfig{Enabled: true, Violations: 3},
			Nuker:        NukerConfig{Enabled: true, Violations: 1},
			NukerB:       NukerBConfig{Enabled: true, Violations: 3},
			FastBreak:    FastBreakConfig{Enabled: true, Violations: 3},
			FastPlace:    FastPlaceConfig{Enabled: true, MaxBPS: 10, Violations: 5},
			Tower:        TowerConfig{Enabled: true, Violations: 5},
			InvalidBreak: InvalidBreakConfig{Enabled: true, Violations: 3},
			EditionFaker: EditionFakerConfig{
				Enabled:         true,
				AllowedTitleIDs: []int64{896928775, 2047319603, 1828326430},
				Violations:      1,
			},
			ClientSpoof: ClientSpoofConfig{Enabled: true, Violations: 1},
			Protocol:    ProtocolConfig{Enabled: true, AllowedVersions: []int32{}, Violations: 1},
			Timer:       TimerConfig{Enabled: true, MaxRatePS: 22, Violations: 5},
			Velocity:    VelocityConfig{Enabled: true, Violations: 5},
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
