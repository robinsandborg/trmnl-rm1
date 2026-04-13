package trmnl

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

func loadConfig(paths Paths) (Config, error) {
	config := Config{
		BaseURL:                   defaultBaseURL,
		DisableWiFiBetweenUpdates: true,
		FBInkNoViewport:           true,
	}

	data, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return Config{}, err
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", paths.ConfigFile, err)
	}

	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	return config, nil
}

func saveState(paths Paths, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(paths.StateFile, data, 0o600)
}

func loadState(paths Paths) (State, error) {
	var state State
	data, err := os.ReadFile(paths.StateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return State{}, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("parse state %s: %w", paths.StateFile, err)
	}
	return state, nil
}

func validateConfig(paths Paths, cfg Config) error {
	var problems []string

	if _, err := url.ParseRequestURI(cfg.BaseURL); err != nil {
		problems = append(problems, fmt.Sprintf("base_url must be a valid absolute URL: %v", err))
	}

	if cfg.refreshFallback() <= 0 {
		problems = append(problems, "refresh_fallback_seconds must be greater than zero")
	}
	if cfg.refreshMin() <= 0 {
		problems = append(problems, "refresh_min_seconds must be greater than zero")
	}
	if cfg.refreshMax() <= 0 {
		problems = append(problems, "refresh_max_seconds must be greater than zero")
	}
	if cfg.refreshMin() > cfg.refreshMax() {
		problems = append(problems, "refresh_min_seconds must be less than or equal to refresh_max_seconds")
	}
	if cfg.refreshFallback() < cfg.refreshMin() || cfg.refreshFallback() > cfg.refreshMax() {
		problems = append(problems, "refresh_fallback_seconds must be within the configured min/max bounds")
	}
	if cfg.wifiTimeout() <= 0 {
		problems = append(problems, "wifi_timeout_seconds must be greater than zero")
	}
	if cfg.fullRefreshEvery() <= 0 {
		problems = append(problems, "full_refresh_every must be greater than zero")
	}
	if cfg.displayWidth() <= 0 || cfg.displayHeight() <= 0 {
		problems = append(problems, "display_width and display_height must be greater than zero")
	}
	if cfg.fbinkBitDepth() <= 0 {
		problems = append(problems, "fbink_bit_depth must be greater than zero")
	}

	deviceID, err := resolveDeviceID(cfg)
	if err != nil || strings.TrimSpace(deviceID) == "" {
		problems = append(problems, "no usable device_id could be determined from config or wireless MAC address")
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid config %s:\n- %s", paths.ConfigFile, strings.Join(problems, "\n- "))
	}
	return nil
}
