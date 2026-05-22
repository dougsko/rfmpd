package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	content := `
node:
  callsign: "W1AW"
  ssid: 1
network:
  direwolf_host: "127.0.0.1"
  direwolf_port: 8001
  offline_mode: true
`
	f, _ := os.CreateTemp("", "config-*.yaml")
	f.WriteString(content)
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Node.Callsign != "W1AW" {
		t.Errorf("expected W1AW, got %s", cfg.Node.Callsign)
	}
	if cfg.Node.SSID != 1 {
		t.Errorf("expected SSID 1, got %d", cfg.Node.SSID)
	}
	if !cfg.Network.OfflineMode {
		t.Error("expected offline_mode true")
	}
	if cfg.Protocol.FragmentThreshold != 200 {
		t.Errorf("expected default threshold 200, got %d", cfg.Protocol.FragmentThreshold)
	}
	if cfg.LoadedFrom != f.Name() {
		t.Errorf("expected LoadedFrom=%s, got %s", f.Name(), cfg.LoadedFrom)
	}
}

func TestLoadConfig_NonexistentFile(t *testing.T) {
	_, err := Load("/tmp/nonexistent-rfmp-config-xyz.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	f, _ := os.CreateTemp("", "config-bad-*.yaml")
	f.WriteString("invalid: yaml: [[[")
	f.Close()
	defer os.Remove(f.Name())

	_, err := Load(f.Name())
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadConfig_EmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Node.Callsign != "N0CALL" {
		t.Errorf("expected default callsign N0CALL, got %s", cfg.Node.Callsign)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Node.Callsign != "N0CALL" {
		t.Errorf("expected N0CALL, got %s", cfg.Node.Callsign)
	}
	if cfg.Network.DirewolfPort != 8001 {
		t.Errorf("expected port 8001, got %d", cfg.Network.DirewolfPort)
	}
	if cfg.API.Port != 8080 {
		t.Errorf("expected api port 8080, got %d", cfg.API.Port)
	}
	if cfg.Timing.BaseDelay != 0.2 {
		t.Errorf("expected base_delay 0.2, got %f", cfg.Timing.BaseDelay)
	}
}

func TestExpandPath_Empty(t *testing.T) {
	result := ExpandPath("")
	if result != "" {
		t.Errorf("expected empty string, got %s", result)
	}
}

func TestExpandPath_Tilde(t *testing.T) {
	result := ExpandPath("~/test/path")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "test/path")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestExpandPath_TildeOnly(t *testing.T) {
	result := ExpandPath("~")
	home, _ := os.UserHomeDir()
	if result != home {
		t.Errorf("expected %s, got %s", home, result)
	}
}

func TestExpandPath_AbsolutePath(t *testing.T) {
	result := ExpandPath("/var/lib/rfmpd/data.db")
	if result != "/var/lib/rfmpd/data.db" {
		t.Errorf("expected unchanged path, got %s", result)
	}
}

func TestFullCallsign_NoSSID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Node.Callsign = "W1AW"
	cfg.Node.SSID = 0
	if cfg.FullCallsign() != "W1AW" {
		t.Errorf("expected W1AW, got %s", cfg.FullCallsign())
	}
}

func TestFullCallsign_WithSSID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Node.Callsign = "W1AW"
	cfg.Node.SSID = 5
	if cfg.FullCallsign() != "W1AW-5" {
		t.Errorf("expected W1AW-5, got %s", cfg.FullCallsign())
	}
}

func TestSaveToFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Node.Callsign = "TEST"

	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	err := cfg.SaveToFile(path)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Node.Callsign != "TEST" {
		t.Errorf("expected TEST, got %s", loaded.Node.Callsign)
	}
}

func TestLoadConfig_WithDatabasePath(t *testing.T) {
	content := `
storage:
  database_path: "~/data/rfmp.db"
`
	f, _ := os.CreateTemp("", "config-*.yaml")
	f.WriteString(content)
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "data/rfmp.db")
	if cfg.Storage.DatabasePath != expected {
		t.Errorf("expected %s, got %s", expected, cfg.Storage.DatabasePath)
	}
}

func TestLoadConfig_SearchPath(t *testing.T) {
	// Create a config.yaml in the working directory (first search path)
	content := `
node:
  callsign: "FOUND"
  ssid: 7
`
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.WriteFile("config.yaml", []byte(content), 0644)

	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Node.Callsign != "FOUND" {
		t.Errorf("expected FOUND, got %s", cfg.Node.Callsign)
	}
	if cfg.Node.SSID != 7 {
		t.Errorf("expected SSID 7, got %d", cfg.Node.SSID)
	}
	if cfg.LoadedFrom != "config.yaml" {
		t.Errorf("expected LoadedFrom=config.yaml, got %s", cfg.LoadedFrom)
	}
}

func TestLoadConfig_SearchPathInvalidYAML(t *testing.T) {
	origDir, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.WriteFile("config.yaml", []byte("bad: yaml: [[["), 0644)

	_, err := Load("")
	if err == nil {
		t.Fatal("expected error for invalid YAML in search path")
	}
}

func TestLoadConfig_AllFields(t *testing.T) {
	content := `
node:
  callsign: "KD2ABC"
  ssid: 3
network:
  direwolf_host: "192.168.1.1"
  direwolf_port: 9001
  reconnect_interval: 10
  offline_mode: false
protocol:
  fragment_threshold: 150
timing:
  base_delay: 0.5
  jitter: 1.0
sync:
  sync_interval: 120
storage:
  database_path: "/tmp/test.db"
api:
  host: "127.0.0.1"
  port: 9090
  cors_origins:
    - "http://localhost:3000"
logging:
  level: "DEBUG"
  file: "/tmp/rfmp.log"
  max_size: 5242880
  backup_count: 3
`
	f, _ := os.CreateTemp("", "config-full-*.yaml")
	f.WriteString(content)
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Node.Callsign != "KD2ABC" {
		t.Errorf("callsign: %s", cfg.Node.Callsign)
	}
	if cfg.Node.SSID != 3 {
		t.Errorf("ssid: %d", cfg.Node.SSID)
	}
	if cfg.Network.DirewolfHost != "192.168.1.1" {
		t.Errorf("host: %s", cfg.Network.DirewolfHost)
	}
	if cfg.Network.DirewolfPort != 9001 {
		t.Errorf("port: %d", cfg.Network.DirewolfPort)
	}
	if cfg.Protocol.FragmentThreshold != 150 {
		t.Errorf("threshold: %d", cfg.Protocol.FragmentThreshold)
	}
	if cfg.Timing.BaseDelay != 0.5 {
		t.Errorf("base_delay: %f", cfg.Timing.BaseDelay)
	}
	if cfg.Sync.SyncInterval != 120 {
		t.Errorf("sync_interval: %d", cfg.Sync.SyncInterval)
	}
	if cfg.API.Port != 9090 {
		t.Errorf("api port: %d", cfg.API.Port)
	}
	if cfg.Logging.Level != "DEBUG" {
		t.Errorf("level: %s", cfg.Logging.Level)
	}
	if cfg.Logging.MaxSize != 5242880 {
		t.Errorf("max_size: %d", cfg.Logging.MaxSize)
	}
}
