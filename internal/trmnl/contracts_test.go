package trmnl

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConfigDefaultsAndPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, data        string
		refresh, rotation int
		wifi, viewport    bool
	}{
		{"missing", "", 6, 3, true, true},
		{"empty", `{}`, 6, 3, true, true},
		{"explicit false", `{"disable_wifi_between_updates":false,"fbink_no_viewport":false}`, 6, 3, false, false},
		{"top level", `{"full_refresh_every":4}`, 4, 3, true, true},
		{"nested wins", `{"full_refresh_every":4,"display_power":{"full_refresh_every":2}}`, 2, 3, true, true},
		{"nested zero falls back", `{"full_refresh_every":4,"display_power":{"full_refresh_every":0}}`, 4, 3, true, true},
		{"nonpositive and unknown", `{"full_refresh_every":-1,"fbink_rotation":0,"refresh_fallback_seconds":-1,"wifi_timeout_seconds":-1,"future_field":true}`, 6, 3, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := isolatedPaths(t)
			if tc.data != "" {
				writeTestFile(t, paths.ConfigFile, []byte(tc.data))
			}
			cfg, err := loadConfig(paths)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.BaseURL != "http://larapaper.local" || cfg.fullRefreshEvery() != tc.refresh || cfg.fbinkRotation() != tc.rotation || cfg.DisableWiFiBetweenUpdates != tc.wifi || cfg.FBInkNoViewport != tc.viewport {
				t.Fatalf("config = %+v", cfg)
			}
			if cfg.refreshFallback() != 30*time.Minute || cfg.refreshMin() != 5*time.Minute || cfg.refreshMax() != 24*time.Hour || cfg.wifiTimeout() != 45*time.Second {
				t.Fatal("effective numeric defaults changed")
			}
			cfg.DeviceID = "explicit"
			if err := validateConfig(paths, cfg); err != nil {
				t.Fatalf("effective defaults no longer validate: %v", err)
			}
		})
	}
}

func TestConfigValidationBounds(t *testing.T) {
	for _, tc := range []struct {
		cfg     Config
		message string
	}{
		{Config{BaseURL: ":invalid"}, "base_url must be a valid absolute URL"},
		{Config{BaseURL: "http://host", RefreshMinSeconds: 4000, RefreshMaxSeconds: 3000}, "refresh_min_seconds must be less than or equal"},
		{Config{BaseURL: "http://host", RefreshFallbackSeconds: 60}, "refresh_fallback_seconds must be within"},
	} {
		cfg := tc.cfg
		cfg.DeviceID = "explicit"
		err := validateConfig(Paths{ConfigFile: "config.json"}, cfg)
		if err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("error = %v", err)
		}
	}
}

func TestStateAndLogFileContracts(t *testing.T) {
	paths := isolatedPaths(t)
	state, err := loadState(paths)
	if err != nil || state != (State{}) {
		t.Fatalf("missing state = %+v, %v", state, err)
	}
	before := readFixture(t, "before.json")
	writeTestFile(t, paths.StateFile, before)
	state, err = loadState(paths)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveState(paths, state); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(paths.StateFile)
	if err != nil || !bytes.Equal(got, before) {
		t.Fatalf("state round trip: %s, %v", got, err)
	}
	var entry CycleLog
	line := readFixture(t, "changed-log.jsonl")
	if err := json.Unmarshal(line, &entry); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := appendCycleLog(paths, entry); err != nil {
			t.Fatal(err)
		}
	}
	got, err = os.ReadFile(paths.LogFile)
	if err != nil || !bytes.Equal(got, bytes.Repeat(line, 2)) {
		t.Fatalf("JSONL append = %s, %v", got, err)
	}
	for _, path := range []string{paths.StateFile, paths.LogFile} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("permissions at %s: %v, %v", path, info, err)
		}
	}
}

func TestPathsUseXDGAndHomeFallbacks(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(key, "")
	}
	paths, err := defaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.ConfigFile != filepath.Join(root, ".config/trmnl-rm1/config.json") || paths.StateFile != filepath.Join(root, ".local/state/trmnl-rm1/state.json") || paths.DownloadedImage != filepath.Join(root, ".cache/trmnl-rm1/downloaded.png") {
		t.Fatalf("home paths = %+v", paths)
	}
	paths = isolatedPaths(t)
	if paths.ConfigFile != filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "trmnl-rm1/config.json") || paths.StateFile != filepath.Join(os.Getenv("XDG_STATE_HOME"), "trmnl-rm1/state.json") || paths.DownloadedImage != filepath.Join(os.Getenv("XDG_CACHE_HOME"), "trmnl-rm1/downloaded.png") {
		t.Fatalf("XDG paths = %+v", paths)
	}
}

func TestCLIContracts(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		args                      []string
		config, output, errorText string
	}{
		{"no arguments", nil, `{}`, "", "usage: trmnl-rm1 <validate|print-device-id|run-once|install-appliance|restore-stock>"},
		{"unknown", []string{"unknown"}, `{}`, "", "usage: trmnl-rm1 <validate|print-device-id|run-once|install-appliance|restore-stock>"},
		{"validate", []string{"validate"}, `{"device_id":"explicit"}`, "config is valid\n", ""},
		{"validate accepts extra arguments", []string{"validate", "extra"}, `{"device_id":"explicit"}`, "config is valid\n", ""},
		{"identity", []string{"print-device-id"}, `{"device_id":"explicit"}`, "explicit\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := isolatedPaths(t)
			writeTestFile(t, paths.ConfigFile, []byte(tc.config))
			var out, errOut bytes.Buffer
			err := NewApp(&out, &errOut).Run(tc.args)
			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if out.String() != tc.output || gotErr != tc.errorText || errOut.Len() != 0 {
				t.Fatalf("stdout=%q error=%q stderr=%q", out.String(), gotErr, errOut.String())
			}
		})
	}
	// Even an invalid invocation creates the runtime directories.
	t.Run("usage creates directories", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("HOME", root)
		for _, key := range []string{"XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
			t.Setenv(key, "")
		}
		paths, err := defaultPaths()
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := NewApp(&out, &out).Run(nil); err == nil {
			t.Fatal("expected usage error")
		}
		for _, path := range []string{paths.ConfigDir, paths.StateDir, paths.CacheDir} {
			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				t.Fatalf("missing runtime directory %s", path)
			}
		}
	})
}

func TestConfiguredDeviceIDPlatformContract(t *testing.T) {
	id, err := resolveDeviceID(Config{DeviceID: "  explicit  "})
	want := "  explicit  "
	if runtime.GOOS == "linux" {
		want = "explicit"
	}
	if err != nil || id != want {
		t.Fatalf("configured identity=%q, %v; want %q", id, err, want)
	}
}
