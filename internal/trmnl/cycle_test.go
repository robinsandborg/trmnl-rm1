package trmnl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var cycleTime = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

// The harness runs App.Run with real config/state/cache/log files and real
// BYOS parsing. Only external device effects and HTTP transport are fakes.
type cycleHarness struct {
	t           *testing.T
	app         *App
	paths       Paths
	before      State
	frame       []byte
	events      []string
	fail        map[string]error
	mode        RuntimeMode
	planned     RuntimeMode
	display     string
	status      int
	imageStatus int
	out, errOut bytes.Buffer
}

func newCycleHarness(t *testing.T) *cycleHarness {
	t.Helper()
	paths := isolatedPaths(t)
	h := &cycleHarness{
		t: t, paths: paths, frame: readFixture(t, "frame.png"), fail: map[string]error{},
		mode:    RuntimeMode{Name: "appliance", ShouldSuspend: true},
		display: `{"image_url":"/images/current.png","filename":"screen.png","refresh_rate":900}`,
		status:  200, imageStatus: 200,
	}
	if err := json.Unmarshal(readFixture(t, "before.json"), &h.before); err != nil {
		t.Fatal(err)
	}
	h.seed()
	writeTestFile(t, paths.ConfigFile, []byte(`{"base_url":"http://larapaper.test","device_id":"AA:BB:CC:DD:EE:FF","access_token":"test-token"}`))
	h.app = NewApp(&h.out, &h.errOut)
	h.app.now = func() time.Time { return cycleTime }
	h.app.cycle = cycleDeps{
		readBatterySample: func(Config) (*BatterySample, error) {
			if err := h.event("battery"); err != nil {
				return nil, err
			}
			return &BatterySample{Status: "Discharging", CapacityPct: "72"}, nil
		},
		determineRuntimeMode: func(_ Paths, cfg Config, state State, now time.Time) (RuntimeMode, error) {
			if now != cycleTime {
				t.Fatalf("mode clock = %v", now)
			}
			if err := h.event(fmt.Sprintf("mode:%d", state.ConsecutiveFailures)); err != nil {
				return RuntimeMode{}, err
			}
			if h.mode.Name == "appliance" && state.ConsecutiveFailures >= cfg.failureThreshold() {
				return RuntimeMode{Name: "recovery", MaintenanceReason: "failure-threshold"}, nil
			}
			return h.mode, nil
		},
		prepareNetwork: func(cfg Config) (*http.Client, func(), error) {
			client, cleanup, err := prepareNetworkWithDeps(cfg, networkDeps{
				bringUp:   func(Config) error { return h.event("wifi-up") },
				bringDown: func(Config) error { return h.event("wifi-down") },
				wait:      func(context.Context, Config) error { return h.event("connect") },
			})
			if client != nil {
				client.Transport = roundTripFunc(h.request)
			}
			return client, cleanup, err
		},
		writeFile: func(path string, data []byte, mode os.FileMode) error {
			if path != paths.DownloadedImage || mode != 0o600 || !bytes.Equal(data, h.frame) {
				t.Fatal("download cache contract changed")
			}
			if err := h.event("cache"); err != nil {
				return err
			}
			return os.WriteFile(path, data, mode)
		},
		renderImage: func(_ Config, data []byte, path string, mode RefreshMode) error {
			if !bytes.Equal(data, h.frame) || path != paths.LastRenderedImage {
				t.Fatal("render payload/path changed")
			}
			if err := h.event("render:" + string(mode)); err != nil {
				return err
			}
			return os.WriteFile(path, data, 0o600)
		},
		planNextCycle: func(_ Config, interval time.Duration, mode RuntimeMode) (RuntimeMode, error) {
			if err := h.event(fmt.Sprintf("schedule:%s:%s", interval, mode.Name)); err != nil {
				return mode, err
			}
			if h.planned.Name != "" {
				return h.planned, nil
			}
			return mode, nil
		},
		appendCycleLog: func(paths Paths, entry CycleLog) error {
			if err := h.event("log:" + entry.FailureCategory); err != nil {
				return err
			}
			return appendCycleLog(paths, entry)
		},
		saveState: func(paths Paths, state State) error {
			if err := h.event("save"); err != nil {
				return err
			}
			return saveState(paths, state)
		},
		suspendDevice: func(Config) error { return h.event("suspend") },
	}
	return h
}

func isolatedPaths(t *testing.T) Paths {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	paths, err := defaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureRuntimeDirs(paths); err != nil {
		t.Fatal(err)
	}
	return paths
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "cycle", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (h *cycleHarness) seed() {
	h.t.Helper()
	if err := saveState(h.paths, h.before); err != nil {
		h.t.Fatal(err)
	}
}

func (h *cycleHarness) event(name string) error {
	h.events = append(h.events, name)
	return h.fail[name]
}

func (h *cycleHarness) request(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet {
		h.t.Fatalf("method = %s", r.Method)
	}
	switch r.URL.String() {
	case "http://larapaper.test/api/display":
		if r.Header.Get("ID") != "AA:BB:CC:DD:EE:FF" || r.Header.Get("access-token") != "test-token" || r.Header.Get("User-Agent") != "trmnl-rm1/0.1.0" {
			h.t.Fatalf("display headers = %v", r.Header)
		}
		if err := h.event("display"); err != nil {
			return nil, err
		}
		resp := jsonResponse(h.display)
		resp.StatusCode, resp.Status = h.status, fmt.Sprintf("%d %s", h.status, http.StatusText(h.status))
		return resp, nil
	case "http://larapaper.test/images/current.png":
		if r.Header.Get("ID") != "" || r.Header.Get("access-token") != "" || r.Header.Get("User-Agent") != "" {
			h.t.Fatal("image request inherited display headers")
		}
		if err := h.event("image"); err != nil {
			return nil, err
		}
		resp := binaryResponse("image/png", h.frame)
		resp.StatusCode, resp.Status = h.imageStatus, fmt.Sprintf("%d %s", h.imageStatus, http.StatusText(h.imageStatus))
		if err := h.fail["image-read"]; err != nil {
			resp.Body = failingBody{err}
		}
		return resp, nil
	default:
		h.t.Fatalf("unexpected URL %s", r.URL)
		return nil, errors.New("unexpected URL")
	}
}

type failingBody struct{ err error }

func (b failingBody) Read([]byte) (int, error) { return 0, b.err }
func (b failingBody) Close() error             { return nil }

func (h *cycleHarness) state() State {
	h.t.Helper()
	state, err := loadState(h.paths)
	if err != nil {
		h.t.Fatal(err)
	}
	return state
}

func (h *cycleHarness) logs() []CycleLog {
	h.t.Helper()
	data, err := os.ReadFile(h.paths.LogFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		h.t.Fatal(err)
	}
	var entries []CycleLog
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var entry CycleLog
		if err := decoder.Decode(&entry); err == io.EOF {
			break
		} else if err != nil {
			h.t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func (h *cycleHarness) wantTrace(want string) {
	h.t.Helper()
	if got := strings.Join(h.events, " "); got != want {
		h.t.Fatalf("trace:\n got %s\nwant %s", got, want)
	}
}

func (h *cycleHarness) run() error {
	h.t.Helper()
	err := h.app.Run([]string{"run-once"})
	if h.out.Len() != 0 || h.errOut.Len() != 0 {
		h.t.Fatal("run-once unexpectedly wrote CLI output")
	}
	return err
}

func TestCycleChangedGolden(t *testing.T) {
	h := newCycleHarness(t)
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	h.wantTrace("battery mode:2 wifi-up connect display image cache render:partial schedule:15m0s:appliance log: save wifi-down suspend")
	for path, fixture := range map[string]string{h.paths.StateFile: "changed-state.json", h.paths.LogFile: "changed-log.jsonl"} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, readFixture(t, fixture)) {
			t.Fatalf("%s differs from %s:\n%s", path, fixture, got)
		}
	}
	for _, path := range []string{h.paths.DownloadedImage, h.paths.LastRenderedImage} {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, h.frame) {
			t.Fatalf("image payload at %s: %v", path, err)
		}
	}
}

func TestCycleUnchangedRetainsDisplayMetadata(t *testing.T) {
	h := newCycleHarness(t)
	h.before.LastImageHash = sha256Hex(h.frame)
	h.before.RenderedUpdates = 5
	h.seed()
	writeTestFile(t, h.paths.LastRenderedImage, []byte("previous prepared image"))
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	h.wantTrace("battery mode:2 wifi-up connect display image cache schedule:15m0s:appliance log: save wifi-down suspend")
	state := h.state()
	if state.LastFilename != h.before.LastFilename || state.LastImageURL != h.before.LastImageURL || state.RenderedUpdates != 5 || state.LastCycleChanged || state.ConsecutiveFailures != 0 {
		t.Fatalf("unchanged state = %+v", state)
	}
	if state.StockSyncUnit != h.before.StockSyncUnit || !state.SyncWasEnabled || !state.XochitlWasEnabled {
		t.Fatal("restore metadata lost")
	}
	logs := h.logs()
	if len(logs) != 1 || !logs[0].SkippedRender || logs[0].FullRefresh || logs[0].ChangedScreen || logs[0].ImageURL != "http://larapaper.test/images/current.png" {
		t.Fatalf("logs = %+v", logs)
	}
	got, _ := os.ReadFile(h.paths.LastRenderedImage)
	if string(got) != "previous prepared image" {
		t.Fatal("unchanged image was rendered")
	}
}

func TestCycleFullRefreshAndModeOutcomes(t *testing.T) {
	for _, mode := range []RuntimeMode{
		{Name: "maintenance", MaintenanceReason: "sentinel-file"},
		{Name: "maintenance", MaintenanceReason: "usb-network"},
		{Name: "boot-grace", MaintenanceReason: "boot-grace"},
		{Name: "recovery", MaintenanceReason: "failure-threshold"},
		{Name: "appliance", ShouldSuspend: true},
	} {
		t.Run(mode.Name+"/"+mode.MaintenanceReason, func(t *testing.T) {
			h := newCycleHarness(t)
			h.mode = mode
			h.before.RenderedUpdates = 5
			h.seed()
			if err := h.run(); err != nil {
				t.Fatal(err)
			}
			want := "battery mode:2 wifi-up connect display image cache render:full schedule:15m0s:" + mode.Name + " log: save wifi-down"
			if mode.ShouldSuspend {
				want += " suspend"
			}
			h.wantTrace(want)
			logs := h.logs()
			if h.state().RenderedUpdates != 6 || len(logs) != 1 || !logs[0].FullRefresh || logs[0].MaintenanceReason != mode.MaintenanceReason {
				t.Fatalf("logs = %+v", logs)
			}
		})
	}
}

func TestCycleRTCFallbackKeepsOriginalStateMode(t *testing.T) {
	h := newCycleHarness(t)
	h.planned = RuntimeMode{Name: "awake-fallback", MaintenanceReason: "rtc-fallback"}
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	h.wantTrace("battery mode:2 wifi-up connect display image cache render:partial schedule:15m0s:appliance log: save wifi-down")
	if h.state().LastMode != "appliance" || h.logs()[0].Mode != "awake-fallback" || h.logs()[0].MaintenanceReason != "rtc-fallback" {
		t.Fatal("RTC fallback state/log distinction changed")
	}
}

func TestCycleFailures(t *testing.T) {
	const prefix = "battery mode:2 wifi-up connect display image cache render:partial"
	tests := []struct {
		name, at, category, trace string
		logs, failures, updates   int
	}{
		{"wifi up", "wifi-up", "wifi", "battery mode:2 wifi-up log:wifi save mode:3 schedule:30m0s:recovery", 1, 3, 0},
		{"wifi timeout", "connect", "wifi", "battery mode:2 wifi-up connect wifi-down log:wifi save mode:3 schedule:30m0s:recovery", 1, 3, 0},
		{"display transport", "display", "http", "battery mode:2 wifi-up connect display log:http save mode:3 schedule:30m0s:recovery wifi-down", 1, 3, 0},
		{"image transport", "image", "http", "battery mode:2 wifi-up connect display image log:http save mode:3 schedule:30m0s:recovery wifi-down", 1, 3, 0},
		{"image body", "image-read", "http", "battery mode:2 wifi-up connect display image log:http save mode:3 schedule:30m0s:recovery wifi-down", 1, 3, 0},
		{"cache", "cache", "", "battery mode:2 wifi-up connect display image cache wifi-down", 0, 2, 0},
		{"render", "render:partial", "render", prefix + " log:render save mode:3 schedule:30m0s:recovery wifi-down", 1, 3, 0},
		{"schedule after success reset", "schedule:15m0s:appliance", "schedule", prefix + " schedule:15m0s:appliance log:schedule save mode:1 schedule:30m0s:appliance wifi-down", 1, 1, 1},
		{"log", "log:", "", prefix + " schedule:15m0s:appliance log: wifi-down", 0, 2, 0},
		{"state", "save", "", prefix + " schedule:15m0s:appliance log: save wifi-down", 1, 2, 0},
		{"suspend adds second log", "suspend", "suspend", prefix + " schedule:15m0s:appliance log: save wifi-down suspend log:suspend save mode:1 schedule:30m0s:appliance", 2, 1, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newCycleHarness(t)
			h.fail[tc.at] = errors.New("injected " + tc.name)
			err := h.run()
			if err == nil || !strings.Contains(err.Error(), h.fail[tc.at].Error()) {
				t.Fatalf("error = %v", err)
			}
			var ce *cycleError
			if tc.category != "" && (!errors.As(err, &ce) || ce.Category != tc.category) {
				t.Fatalf("category = %v", err)
			}
			h.wantTrace(tc.trace)
			state, logs := h.state(), h.logs()
			if len(logs) != tc.logs || state.ConsecutiveFailures != tc.failures || state.RenderedUpdates != tc.updates {
				t.Fatalf("state = %+v; logs = %+v", state, logs)
			}
			if state.StockSyncUnit != h.before.StockSyncUnit || !state.SyncWasEnabled || !state.XochitlWasEnabled {
				t.Fatal("restore metadata lost")
			}
			if tc.category == "" {
				if !reflect.DeepEqual(state, h.before) {
					t.Fatalf("direct error mutated state: %+v", state)
				}
			} else {
				last := logs[len(logs)-1]
				if state.LastFailureCategory != tc.category || state.LastFailureMessage != err.Error() || state.LastFailureAt != cycleTime || last.FailureCategory != tc.category || last.ConsecutiveFails != tc.failures {
					t.Fatalf("failure state/log = %+v / %+v", state, last)
				}
				if tc.failures == 3 && last.MaintenanceReason != "failure-threshold" {
					t.Fatal("threshold reason missing")
				}
				if tc.updates == 1 && state.LastSuccessAt != cycleTime {
					t.Fatal("post-render failure lost success timestamp")
				}
			}
		})
	}
}

func TestCycleHTTPFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body, message        string
		displayStatus, imageStatus int
		image                      bool
	}{
		{"display status", "{}", "display endpoint returned 503 Service Unavailable", 503, 200, false},
		{"invalid JSON", "{", "parse display response: unexpected EOF", 200, 200, false},
		{"missing image URL", "{}", "display response missing image_url", 200, 200, false},
		{"image status", `{"image_url":"/images/current.png"}`, "image download returned 404 Not Found", 200, 404, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCycleHarness(t)
			h.display, h.status, h.imageStatus = tc.body, tc.displayStatus, tc.imageStatus
			err := h.run()
			if err == nil || err.Error() != tc.message {
				t.Fatalf("error = %v", err)
			}
			want := "battery mode:2 wifi-up connect display"
			if tc.image {
				want += " image"
			}
			h.wantTrace(want + " log:http save mode:3 schedule:30m0s:recovery wifi-down")
			if h.state().LastImageHash != h.before.LastImageHash || h.state().ConsecutiveFailures != 3 || h.logs()[0].FailureCategory != "http" {
				t.Fatal("HTTP failure contract changed")
			}
		})
	}
}

func TestCycleEarlyFailuresBypassFinalization(t *testing.T) {
	for _, tc := range []struct {
		name, trace string
		setup       func(*cycleHarness)
	}{
		{"config parse", "", func(h *cycleHarness) { writeTestFile(h.t, h.paths.ConfigFile, []byte("{")) }},
		{"config validation", "", func(h *cycleHarness) {
			writeTestFile(h.t, h.paths.ConfigFile, []byte(`{"base_url":":bad","device_id":"explicit"}`))
		}},
		{"state parse", "", func(h *cycleHarness) { writeTestFile(h.t, h.paths.StateFile, []byte("{")) }},
		{"runtime observation", "battery mode:2", func(h *cycleHarness) { h.fail["mode:2"] = errors.New("mode failed") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCycleHarness(t)
			tc.setup(h)
			before, _ := os.ReadFile(h.paths.StateFile)
			if err := h.run(); err == nil {
				t.Fatal("expected error")
			}
			h.wantTrace(tc.trace)
			after, _ := os.ReadFile(h.paths.StateFile)
			if !bytes.Equal(before, after) || len(h.logs()) != 0 {
				t.Fatal("early failure was finalized")
			}
		})
	}
}

func TestCycleFailureFinalizationErrors(t *testing.T) {
	for _, at := range []string{"log:http", "save", "mode:3", "schedule:30m0s:recovery"} {
		t.Run(at, func(t *testing.T) {
			h := newCycleHarness(t)
			h.status = 503
			secondary := errors.New("secondary failure")
			h.fail[at] = secondary
			err := h.run()
			if err == nil || !strings.Contains(err.Error(), "display endpoint returned 503") {
				t.Fatalf("error = %v", err)
			}
			wantJoined := at == "log:http" || at == "save"
			if errors.Is(err, secondary) != wantJoined {
				t.Fatalf("secondary error visibility = %v", err)
			}
			trace := "battery mode:2 wifi-up connect display log:http"
			if at != "log:http" {
				trace += " save"
			}
			if !wantJoined {
				trace += " mode:3"
			}
			if at == "schedule:30m0s:recovery" {
				trace += " schedule:30m0s:recovery"
			}
			h.wantTrace(trace + " wifi-down")
			if wantJoined && !reflect.DeepEqual(h.state(), h.before) {
				t.Fatal("failed persistence changed state")
			}
			if !wantJoined && h.state().ConsecutiveFailures != 3 {
				t.Fatal("failure state not persisted")
			}
		})
	}
}

func TestCycleBestEffortEffects(t *testing.T) {
	for _, at := range []string{"battery", "wifi-down"} {
		t.Run(at, func(t *testing.T) {
			h := newCycleHarness(t)
			h.fail[at] = errors.New("ignored effect failure")
			if err := h.run(); err != nil {
				t.Fatal(err)
			}
			h.wantTrace("battery mode:2 wifi-up connect display image cache render:partial schedule:15m0s:appliance log: save wifi-down suspend")
			if h.state().ConsecutiveFailures != 0 {
				t.Fatal("best-effort failure counted")
			}
			if at == "battery" && h.logs()[0].Battery != nil {
				t.Fatal("unexpected battery sample")
			}
		})
	}
}

func TestPrepareNetworkLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name    string
		disable bool
		fail    string
		want    []string
	}{
		{"managed", true, "", []string{"up", "wait", "down"}},
		{"unmanaged", false, "", []string{"wait"}},
		{"up failure", true, "up", []string{"up"}},
		{"timeout", true, "wait", []string{"up", "wait", "down"}},
		{"unmanaged timeout", false, "wait", []string{"wait"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			var captured context.Context
			failure := errors.New("network failure")
			record := func(name string) error {
				events = append(events, name)
				if tc.fail == name {
					return failure
				}
				return nil
			}
			cfg := Config{DisableWiFiBetweenUpdates: tc.disable, WiFiTimeoutSeconds: 7}
			client, cleanup, err := prepareNetworkWithDeps(cfg, networkDeps{
				bringUp:   func(Config) error { return record("up") },
				bringDown: func(Config) error { return record("down") },
				wait: func(ctx context.Context, _ Config) error {
					captured = ctx
					deadline, ok := ctx.Deadline()
					if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 7*time.Second {
						t.Fatal("missing/configured deadline changed")
					}
					return record("wait")
				},
			})
			if (tc.fail != "") != errors.Is(err, failure) {
				t.Fatalf("error = %v", err)
			}
			if tc.fail == "" && (client == nil || client.Timeout != 7*time.Second) {
				t.Fatal("HTTP timeout changed")
			}
			if tc.fail != "" && client != nil {
				t.Fatal("failed network returned client")
			}
			if captured != nil && !errors.Is(captured.Err(), context.Canceled) {
				t.Fatal("connectivity context not canceled")
			}
			cleanup()
			if !reflect.DeepEqual(events, tc.want) {
				t.Fatalf("events = %v, want %v", events, tc.want)
			}
		})
	}
}
