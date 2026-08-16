package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.ServerPort != 57579 {
		t.Errorf("port = %d, want 57579", cfg.ServerPort)
	}
	if cfg.BatteryTempThresholdC != 45.0 {
		t.Errorf("temp threshold = %v, want 45", cfg.BatteryTempThresholdC)
	}
	if cfg.MinSOCPercent != 15.0 {
		t.Errorf("min SOC = %v, want 15", cfg.MinSOCPercent)
	}
	if cfg.WorkOrderAcceptTimeout != 15*time.Minute {
		t.Errorf("WO timeout = %v, want 15m", cfg.WorkOrderAcceptTimeout)
	}
	if cfg.BlackStartDeadline != 20*time.Minute {
		t.Errorf("deadline = %v, want 20m", cfg.BlackStartDeadline)
	}
}

func TestLoadFromJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{
		"server_port": 9999,
		"read_timeout_seconds": 60,
		"battery_temp_threshold_c": 50.0,
		"min_soc_percent": 20.0,
		"workorder_accept_seconds": 600,
		"blackstart_deadline_seconds": 900
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ServerPort != 9999 {
		t.Errorf("port = %d, want 9999", cfg.ServerPort)
	}
	if cfg.ReadTimeout != 60*time.Second {
		t.Errorf("read timeout = %v, want 60s", cfg.ReadTimeout)
	}
	if cfg.BatteryTempThresholdC != 50.0 {
		t.Errorf("temp threshold = %v, want 50", cfg.BatteryTempThresholdC)
	}
	if cfg.MinSOCPercent != 20.0 {
		t.Errorf("min SOC = %v, want 20", cfg.MinSOCPercent)
	}
	if cfg.WorkOrderAcceptTimeout != 600*time.Second {
		t.Errorf("WO timeout = %v, want 600s", cfg.WorkOrderAcceptTimeout)
	}
	if cfg.BlackStartDeadline != 900*time.Second {
		t.Errorf("deadline = %v, want 900s", cfg.BlackStartDeadline)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"server_port": 9999, "battery_temp_threshold_c": 50.0}`
	os.WriteFile(path, []byte(data), 0644)

	t.Setenv("GRID_PORT", "8080")
	t.Setenv("GRID_BATTERY_TEMP_THRESHOLD", "42.5")
	t.Setenv("GRID_WO_ACCEPT_TIMEOUT", "30m")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ServerPort != 8080 {
		t.Errorf("port = %d, want 8080", cfg.ServerPort)
	}
	if cfg.BatteryTempThresholdC != 42.5 {
		t.Errorf("temp threshold = %v, want 42.5", cfg.BatteryTempThresholdC)
	}
	if cfg.WorkOrderAcceptTimeout != 30*time.Minute {
		t.Errorf("WO timeout = %v, want 30m", cfg.WorkOrderAcceptTimeout)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("missing file should fall back to default, got error: %v", err)
	}
	if cfg.ServerPort != 57579 {
		t.Errorf("port = %d, want 57579 (default)", cfg.ServerPort)
	}
}
