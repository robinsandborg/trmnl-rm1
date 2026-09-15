package trmnl

import (
	"fmt"
	"github.com/robinsandborg/rm1-trmnl/internal/storage"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

func loadConfig(paths Paths) (Config, error) {
	config := Config{
		BaseURL:                   defaultBaseURL,
		DisableWiFiBetweenUpdates: true,
		FBInkNoViewport:           true,
	}

	if err := storage.LoadJSON(paths.ConfigFile, "config", &config); err != nil {
		return Config{}, err
	}

	if config.BaseURL == "" {
		config.BaseURL = defaultBaseURL
	}
	return config, nil
}

// installationSnapshot is independently durable: runtime recovery cannot erase
// the stock service choices needed by restore. Legacy fields remain in State
// for binaries which predate this file.
type installationSnapshot struct {
	Version           int             `json:"version"`
	StockSyncUnit     string          `json:"stock_sync_unit,omitempty"`
	XochitlWasEnabled bool            `json:"xochitl_was_enabled"`
	SyncWasEnabled    bool            `json:"sync_was_enabled"`
	MaskedNoise       map[string]bool `json:"masked_noise,omitempty"`
}

func installationPath(paths Paths) string {
	return filepath.Join(filepath.Dir(paths.StateFile), "install-state.json")
}
func readInstallationSnapshot(paths Paths) (installationSnapshot, bool, error) {
	return storage.LoadRecoverableJSON[installationSnapshot](installationPath(paths), "installation state", false, func(s installationSnapshot) error {
		if s.Version != 1 {
			return fmt.Errorf("unsupported installation snapshot version %d", s.Version)
		}
		return nil
	})
}
func hasInstallationSnapshot(paths Paths) (bool, error) {
	_, found, err := readInstallationSnapshot(paths)
	return found, err
}
func saveInstallationSnapshot(paths Paths, state State) error {
	snapshot, found, err := readInstallationSnapshot(paths)
	if err != nil {
		return err
	}
	if !found {
		snapshot = installationSnapshot{
			Version: 1, StockSyncUnit: state.StockSyncUnit,
			XochitlWasEnabled: state.XochitlWasEnabled, SyncWasEnabled: state.SyncWasEnabled,
		}
	}
	// A reinstall may discover newly supported stock services. Only their
	// previously unknown preferences may be added; existing choices stay intact.
	changed := !found
	for unit, enabled := range state.MaskedNoise {
		if _, known := snapshot.MaskedNoise[unit]; known {
			continue
		}
		if snapshot.MaskedNoise == nil {
			snapshot.MaskedNoise = make(map[string]bool)
		}
		snapshot.MaskedNoise[unit] = enabled
		changed = true
	}
	if !changed {
		// Also repairs a stale backup left by an interrupted earlier install.
		return storage.SaveJSON(installationPath(paths), snapshot)
	}
	if err := storage.SaveJSON(installationPath(paths), snapshot); err != nil {
		return err
	}
	// Unlike changing runtime counters, this snapshot must restore every service
	// about to be masked. Bring its recovery copy up to the same snapshot before
	// returning permission to perform those installation effects.
	return storage.SaveJSON(installationPath(paths), snapshot)
}
func reconcileInstallation(paths Paths, state *State) error {
	snapshot, found, err := readInstallationSnapshot(paths)
	if err != nil {
		return err
	}
	if !found {
		if state.StockSyncUnit != "" || state.XochitlWasEnabled || state.SyncWasEnabled || state.MaskedNoise != nil {
			return saveInstallationSnapshot(paths, *state)
		}
		return nil
	}
	state.StockSyncUnit = snapshot.StockSyncUnit
	state.XochitlWasEnabled = snapshot.XochitlWasEnabled
	state.SyncWasEnabled = snapshot.SyncWasEnabled
	state.MaskedNoise = snapshot.MaskedNoise
	return nil
}
func saveState(paths Paths, state State) error {
	if err := reconcileInstallation(paths, &state); err != nil {
		return err
	}
	return storage.SaveJSON(paths.StateFile, state)
}
func loadState(paths Paths) (State, error) {
	state, _, err := storage.LoadRecoverableJSON[State](paths.StateFile, "state", true)
	if err != nil {
		return State{}, err
	}
	if err := reconcileInstallation(paths, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func validateConfig(paths Paths, cfg Config) error {
	var problems []string
	policy := cfg.batteryPolicy()
	if policy.Critical < 1 || policy.Low <= policy.Critical || policy.Recovery <= policy.Low || policy.Recovery > 100 || policy.CheckInterval < 5*time.Minute || policy.CheckInterval > 24*time.Hour {
		problems = append(problems, "battery thresholds must satisfy 1 <= critical < low < recovery <= 100 and battery check interval must be 300..86400 seconds")
	}

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
