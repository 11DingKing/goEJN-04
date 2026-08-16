package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all tunable parameters for the microgrid dispatch service.
type Config struct {
	ServerPort             int           `json:"server_port"`
	ReadTimeout            time.Duration `json:"-"`
	WriteTimeout           time.Duration `json:"-"`
	BatteryTempThresholdC  float64       `json:"battery_temp_threshold_c"`
	MinSOCPercent          float64       `json:"min_soc_percent"`
	WorkOrderAcceptTimeout time.Duration `json:"-"`
	BlackStartDeadline     time.Duration `json:"-"`
	SMSResendInterval      time.Duration `json:"-"`
	SchedulerInterval      time.Duration `json:"-"`
}

// jsonConfig is the on-disk representation with human-friendly duration strings.
type jsonConfig struct {
	ServerPort             int     `json:"server_port"`
	ReadTimeoutSeconds     int     `json:"read_timeout_seconds"`
	WriteTimeoutSeconds    int     `json:"write_timeout_seconds"`
	BatteryTempThresholdC  float64 `json:"battery_temp_threshold_c"`
	MinSOCPercent          float64 `json:"min_soc_percent"`
	WorkOrderAcceptSeconds int     `json:"workorder_accept_seconds"`
	BlackStartDeadlineSecs int     `json:"blackstart_deadline_seconds"`
	SMSResendSeconds       int     `json:"sms_resend_seconds"`
	SchedulerIntervalSecs  int     `json:"scheduler_interval_seconds"`
}

// Default returns a production-ready configuration matching the business rules:
// 45 °C temperature threshold, 15 % minimum SOC, 15-minute work-order
// acceptance timeout, 20-minute black-start deadline, service on port 57579.
func Default() *Config {
	return &Config{
		ServerPort:             57579,
		ReadTimeout:            30 * time.Second,
		WriteTimeout:           30 * time.Second,
		BatteryTempThresholdC:  45.0,
		MinSOCPercent:          15.0,
		WorkOrderAcceptTimeout: 15 * time.Minute,
		BlackStartDeadline:     20 * time.Minute,
		SMSResendInterval:      30 * time.Second,
		SchedulerInterval:      10 * time.Second,
	}
}

// Load reads configuration from the JSON file at path, then applies environment
// variable overrides. Missing fields fall back to Default().
func Load(path string) (*Config, error) {
	cfg := Default()

	if data, err := os.ReadFile(path); err == nil {
		var jc jsonConfig
		if err := json.Unmarshal(data, &jc); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
		applyJSON(cfg, &jc)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	applyEnv(cfg)
	return cfg, nil
}

func applyJSON(cfg *Config, jc *jsonConfig) {
	if jc.ServerPort > 0 {
		cfg.ServerPort = jc.ServerPort
	}
	if jc.ReadTimeoutSeconds > 0 {
		cfg.ReadTimeout = time.Duration(jc.ReadTimeoutSeconds) * time.Second
	}
	if jc.WriteTimeoutSeconds > 0 {
		cfg.WriteTimeout = time.Duration(jc.WriteTimeoutSeconds) * time.Second
	}
	if jc.BatteryTempThresholdC > 0 {
		cfg.BatteryTempThresholdC = jc.BatteryTempThresholdC
	}
	if jc.MinSOCPercent > 0 {
		cfg.MinSOCPercent = jc.MinSOCPercent
	}
	if jc.WorkOrderAcceptSeconds > 0 {
		cfg.WorkOrderAcceptTimeout = time.Duration(jc.WorkOrderAcceptSeconds) * time.Second
	}
	if jc.BlackStartDeadlineSecs > 0 {
		cfg.BlackStartDeadline = time.Duration(jc.BlackStartDeadlineSecs) * time.Second
	}
	if jc.SMSResendSeconds > 0 {
		cfg.SMSResendInterval = time.Duration(jc.SMSResendSeconds) * time.Second
	}
	if jc.SchedulerIntervalSecs > 0 {
		cfg.SchedulerInterval = time.Duration(jc.SchedulerIntervalSecs) * time.Second
	}
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("GRID_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			cfg.ServerPort = p
		}
	}
	if v := os.Getenv("GRID_BATTERY_TEMP_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			cfg.BatteryTempThresholdC = f
		}
	}
	if v := os.Getenv("GRID_MIN_SOC"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			cfg.MinSOCPercent = f
		}
	}
	if v := os.Getenv("GRID_WO_ACCEPT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.WorkOrderAcceptTimeout = d
		}
	}
	if v := os.Getenv("GRID_BLACKSTART_DEADLINE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.BlackStartDeadline = d
		}
	}
}
