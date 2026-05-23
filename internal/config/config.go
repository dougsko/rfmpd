package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Node     NodeConfig     `yaml:"node" json:"node"`
	Network  NetworkConfig  `yaml:"network" json:"network"`
	Protocol ProtocolConfig `yaml:"protocol" json:"protocol"`
	Timing   TimingConfig   `yaml:"timing" json:"timing"`
	Sync     SyncConfig     `yaml:"sync" json:"sync"`
	Storage  StorageConfig  `yaml:"storage" json:"storage"`
	API      APIConfig      `yaml:"api" json:"api"`
	Logging  LoggingConfig  `yaml:"logging" json:"logging"`

	LoadedFrom string `yaml:"-" json:"-"`
}

type NodeConfig struct {
	Callsign string `yaml:"callsign" json:"callsign"`
	SSID     int    `yaml:"ssid" json:"ssid"`
}

type NetworkConfig struct {
	DirewolfHost      string `yaml:"direwolf_host" json:"direwolf_host"`
	DirewolfPort      int    `yaml:"direwolf_port" json:"direwolf_port"`
	ReconnectInterval int    `yaml:"reconnect_interval" json:"reconnect_interval"`
	OfflineMode       bool   `yaml:"offline_mode" json:"offline_mode"`
}

type ProtocolConfig struct {
	FragmentThreshold int `yaml:"fragment_threshold" json:"fragment_threshold"`
}

type TimingConfig struct {
	BaseDelay float64 `yaml:"base_delay" json:"base_delay"`
	Jitter    float64 `yaml:"jitter" json:"jitter"`
}

type SyncConfig struct {
	SyncInterval int `yaml:"sync_interval" json:"sync_interval"`
}

type StorageConfig struct {
	DatabasePath string `yaml:"database_path" json:"database_path"`
}

type APIConfig struct {
	Host        string   `yaml:"host" json:"host"`
	Port        int      `yaml:"port" json:"port"`
	CORSOrigins []string `yaml:"cors_origins" json:"cors_origins"`
}

type LoggingConfig struct {
	Level       string `yaml:"level" json:"level"`
	File        string `yaml:"file" json:"file,omitempty"`
	MaxSize     int    `yaml:"max_size" json:"max_size"`
	BackupCount int    `yaml:"backup_count" json:"backup_count"`
}

func DefaultConfig() *Config {
	return &Config{
		Node: NodeConfig{
			Callsign: "N0CALL",
			SSID:     0,
		},
		Network: NetworkConfig{
			DirewolfHost:      "127.0.0.1",
			DirewolfPort:      8001,
			ReconnectInterval: 5,
			OfflineMode:       false,
		},
		Protocol: ProtocolConfig{
			FragmentThreshold: 200,
		},
		Timing: TimingConfig{
			BaseDelay: 0.2,
			Jitter:    0.4,
		},
		Sync: SyncConfig{
			SyncInterval: 60,
		},
		Storage: StorageConfig{
			DatabasePath: "/var/lib/rfmpd/messages.db",
		},
		API: APIConfig{
			Host: "0.0.0.0",
			Port: 8080,
			CORSOrigins: []string{
				"*",
			},
		},
		Logging: LoggingConfig{
			Level:       "INFO",
			MaxSize:     10485760,
			BackupCount: 5,
		},
	}
}

// Load loads configuration from the given path. If path is empty, it searches
// default locations: ./config.yaml, ~/.config/rfmpd/config.yaml, /etc/rfmpd/config.yaml.
// If no config file is found, returns defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
		cfg.LoadedFrom = path
	} else {
		searchPaths := []string{
			"config.yaml",
			ExpandPath("~/.config/rfmpd/config.yaml"),
			"/etc/rfmpd/config.yaml",
		}
		for _, p := range searchPaths {
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
			cfg.LoadedFrom = p
			break
		}
	}

	cfg.Storage.DatabasePath = ExpandPath(cfg.Storage.DatabasePath)
	cfg.Logging.File = ExpandPath(cfg.Logging.File)

	return cfg, nil
}

// ExpandPath replaces a leading ~ with the user's home directory.
func ExpandPath(p string) string {
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return home
	}
	return p
}

func (c *Config) FullCallsign() string {
	if c.Node.SSID == 0 {
		return c.Node.Callsign
	}
	return fmt.Sprintf("%s-%d", c.Node.Callsign, c.Node.SSID)
}

// SaveToFile persists the current configuration to the given path.
func (c *Config) SaveToFile(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}
	return os.WriteFile(path, data, 0644)
}
