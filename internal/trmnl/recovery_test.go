package trmnl

import (
	"bytes"
	"errors"
	"image"
	_ "image/png"
	"os"
	"strings"
	"testing"
	"time"
)

func recoveryHarness(t *testing.T) *cycleHarness {
	h := newCycleHarness(t)
	h.app.cycle.bootID = func() string { return "boot-one" }
	original := h.app.cycle.renderImage
	h.app.cycle.renderImage = func(cfg Config, b []byte, p string, m RefreshMode) error {
		if bytes.Equal(b, h.frame) {
			return original(cfg, b, p, m)
		}
		if _, _, err := image.Decode(bytes.NewReader(b)); err != nil {
			t.Fatal(err)
		}
		if m != RefreshFull {
			t.Fatal("local status must fully refresh")
		}
		if err := h.event("local"); err != nil {
			return err
		}
		return os.WriteFile(p, b, 0600)
	}
	return h
}
func TestRecoveryColdBootAndLocalScreenForceFullRedraw(t *testing.T) {
	h := recoveryHarness(t)
	h.before.LastImageHash = sha256Hex(h.frame)
	h.seed()
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(h.events, " "), "render:full") || h.state().BootID != "boot-one" {
		t.Fatal(h.events)
	}
	h.events = nil
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(h.events, " "), "render:") {
		t.Fatal("unchanged same-boot update rendered")
	}
	state := h.state()
	state.LocalScreen = "offline"
	if err := saveState(h.paths, state); err != nil {
		t.Fatal(err)
	}
	h.events = nil
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(h.events, " "), "render:full") || h.state().LocalScreen != "" {
		t.Fatal(h.events)
	}
}
func TestRecoveryBeforeFirstNetworkSuccessShowsLocalScreenAndRetries(t *testing.T) {
	for _, failure := range []string{"wifi-up", "connect", "mode:2", "cache", "save", "log:"} {
		t.Run(failure, func(t *testing.T) {
			h := recoveryHarness(t)
			h.fail[failure] = errors.New("injected")
			if err := h.run(); err == nil {
				t.Fatal("expected error")
			}
			trace := strings.Join(h.events, " ")
			if !strings.Contains(trace, "schedule:") {
				t.Fatalf("no retry: %s", trace)
			}
			if !strings.Contains(trace, "suspend") {
				t.Fatalf("discharging failure left awake: %s", trace)
			}
			if (failure == "wifi-up" || failure == "connect") && !strings.Contains(trace, "local") {
				t.Fatal("boot offline status missing")
			}
		})
	}
}
func TestLowBatteryNeverAcquiresRadioAndResumesAfterHysteresis(t *testing.T) {
	h := recoveryHarness(t)
	pct, status := "19", "Discharging"
	h.app.cycle.readBatterySample = func(Config) (*BatterySample, error) { return &BatterySample{CapacityPct: pct, Status: status}, nil }
	h.app.cycle.ensureInterface = func(Config) { t.Fatal("radio enumeration at low battery") }
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !h.state().BatteryLow || h.state().LocalScreen != "low-battery" {
		t.Fatal(h.state())
	}
	if trace := strings.Join(h.events, " "); strings.Contains(trace, "wifi") || !strings.Contains(trace, "schedule:30m0s:low-battery") {
		t.Fatal(trace)
	}
	h.events = nil
	pct = "25"
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(h.events, " "), "local") {
		t.Fatal("warning redrawn within same boot")
	}
	h.events = nil
	pct = "invalid"
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !h.state().BatteryLow {
		t.Fatal("invalid sensor cleared battery protection")
	}
	h.events = nil
	pct = "29"
	status = "Charging"
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !h.state().BatteryLow || strings.Contains(strings.Join(h.events, " "), "wifi") {
		t.Fatal("charging below threshold fetched")
	}
	h.events = nil
	pct = "30"
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if h.state().BatteryLow || !strings.Contains(strings.Join(h.events, " "), "render:full") {
		t.Fatal("charging recovery did not replace warning")
	}
}
func TestScheduledDeadlineAndLockPreventDuplicateCycle(t *testing.T) {
	h := recoveryHarness(t)
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	h.events = nil
	if err := h.app.Run([]string{"run-scheduled"}); err != nil {
		t.Fatal(err)
	}
	if len(h.events) != 0 {
		t.Fatal("safety timer fetched before due", h.events)
	}
	unlock, acquired, err := cycleLock(h.paths)
	if err != nil || !acquired {
		t.Fatal(err)
	}
	if err := h.app.Run([]string{"run-once"}); err != nil {
		t.Fatal(err)
	}
	unlock()
	h.errOut.Reset()
	if len(h.events) != 0 {
		t.Fatal("overlapping process ran")
	}
	state := h.state()
	state.NextAttemptAt = cycleTime.Add(20 * time.Second)
	if err := saveState(h.paths, state); err != nil {
		t.Fatal(err)
	}
	if err := h.app.Run([]string{"run-scheduled"}); err != nil {
		t.Fatal(err)
	}
	if len(h.events) == 0 {
		t.Fatal("slightly early RTC wake lost cycle")
	}
}
func TestCorruptRuntimeDoesNotAbandonCycle(t *testing.T) {
	h := recoveryHarness(t)
	writeTestFile(t, h.paths.StateFile, []byte("{"))
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if h.state().BootID != "boot-one" {
		t.Fatal("state not reconstructed")
	}
}
func TestCriticalBatteryShutdownIsExplicitAndInjectable(t *testing.T) {
	h := recoveryHarness(t)
	writeTestFile(t, h.paths.ConfigFile, []byte(`{"device_id":"test","critical_battery_shutdown":true}`))
	h.app.cycle.readBatterySample = func(Config) (*BatterySample, error) {
		return &BatterySample{CapacityPct: "4", Status: "Discharging"}, nil
	}
	called := false
	h.app.cycle.shutdown = func() error { called = true; return nil }
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("critical shutdown not invoked")
	}
	if strings.Contains(strings.Join(h.events, " "), "wifi") {
		t.Fatal("critical battery fetched")
	}
}

func TestBatteryStatusLogsActualPanelRefreshes(t *testing.T) {
	h := recoveryHarness(t)
	h.app.cycle.readBatterySample = func(Config) (*BatterySample, error) {
		return &BatterySample{CapacityPct: "19", Status: "Discharging"}, nil
	}
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	first := h.logs()[0]
	if !first.ChangedScreen || !first.FullRefresh || first.SkippedRender {
		t.Fatal(first)
	}
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
	second := h.logs()[1]
	if second.ChangedScreen || second.FullRefresh || !second.SkippedRender {
		t.Fatal(second)
	}
}
func TestFailedChangedRenderPreservesLastUsableDownload(t *testing.T) {
	h := recoveryHarness(t)
	writeTestFile(t, h.paths.DownloadedImage, h.frame)
	h.fail["render:full"] = errors.New("render rejected")
	if err := h.run(); err == nil {
		t.Fatal("expected failure")
	}
	actual, err := os.ReadFile(h.paths.DownloadedImage)
	if err != nil || !bytes.Equal(actual, h.frame) {
		t.Fatal("last good cache lost")
	}
	if !strings.Contains(strings.Join(h.events, " "), "local") {
		t.Fatal("offline banner missing")
	}
	entry := h.logs()[0]
	if !entry.FullRefresh || entry.SkippedRender {
		t.Fatal("offline draw not reported", entry)
	}
}

func TestCriticalShutdownSurvivesAncillaryFailures(t *testing.T) {
	for _, stage := range []string{"local", "save", "log:", "shutdown", "none"} {
		t.Run(stage, func(t *testing.T) {
			h := recoveryHarness(t)
			writeTestFile(t, h.paths.ConfigFile, []byte(`{"device_id":"test","critical_battery_shutdown":true}`))
			h.app.cycle.readBatterySample = func(Config) (*BatterySample, error) {
				return &BatterySample{CapacityPct: "4", Status: "Discharging"}, nil
			}
			failure := errors.New("injected ancillary failure")
			if stage != "shutdown" && stage != "none" {
				h.fail[stage] = failure
			}
			calls := 0
			h.app.cycle.shutdown = func() error {
				calls++
				if stage == "shutdown" {
					return failure
				}
				return nil
			}
			h.app.cycle.planNextCycle = func(Config, time.Duration, RuntimeMode) (RuntimeMode, error) {
				t.Fatal("critical shutdown must not depend on planning a future wake")
				return RuntimeMode{}, failure
			}
			h.app.cycle.suspendDevice = func(Config) error { t.Fatal("critical shutdown fell back to sleep"); return nil }
			err := h.app.Run([]string{"run-once"})
			if calls != 1 {
				t.Fatalf("shutdown calls=%d; error=%v", calls, err)
			}
			if stage != "none" && !errors.Is(err, failure) {
				t.Fatalf("diagnostic failure lost: %v", err)
			}
			if stage == "none" && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBrokenBootRendererRetainsSchedulingBackoff(t *testing.T) {
	h := recoveryHarness(t)
	h.before.ConsecutiveFailures = 0
	h.before.BootID = "previous-boot"
	h.seed()
	now := cycleTime
	h.app.now = func() time.Time { return now }
	h.app.cycle.determineRuntimeMode = func(Paths, Config, State, time.Time) (RuntimeMode, error) {
		return RuntimeMode{Name: "appliance", ShouldSuspend: true}, nil
	}
	renders := 0
	h.app.cycle.renderImage = func(Config, []byte, string, RefreshMode) error { renders++; return errors.New("panel unavailable") }
	run := func() {
		t.Helper()
		if err := h.app.Run([]string{"run-scheduled"}); err == nil {
			t.Fatal("expected rendering failure")
		}
	}
	run()
	state := h.state()
	if state.BootID != "previous-boot" || state.ScheduleBootID != "boot-one" || !state.NextAttemptAt.Equal(now.Add(5*time.Minute)) {
		t.Fatal(state)
	}
	now = now.Add(5 * time.Minute)
	run()
	state = h.state()
	if state.ConsecutiveFailures != 2 || !state.NextAttemptAt.Equal(now.Add(10*time.Minute)) {
		t.Fatal(state)
	}
	before := renders
	now = now.Add(5 * time.Minute)
	if err := h.app.Run([]string{"run-scheduled"}); err != nil {
		t.Fatal(err)
	}
	if renders != before {
		t.Fatal("safety timer bypassed backoff because boot display was not restored")
	}
	now = now.Add(5 * time.Minute)
	run()
	if state = h.state(); state.ConsecutiveFailures != 3 || !state.NextAttemptAt.Equal(now.Add(20*time.Minute)) {
		t.Fatal(state)
	}
}

func TestLateCycleFailuresRetainPreviousBackoff(t *testing.T) {
	for _, stage := range []string{"plan", "log", "save"} {
		t.Run(stage, func(t *testing.T) {
			h := recoveryHarness(t)
			h.before.ConsecutiveFailures = 0
			h.seed()
			h.app.cycle.determineRuntimeMode = func(Paths, Config, State, time.Time) (RuntimeMode, error) {
				return RuntimeMode{Name: "appliance", ShouldSuspend: true}, nil
			}
			failure := errors.New("persistent late failure")
			if stage == "plan" {
				h.app.cycle.planNextCycle = func(_ Config, _ time.Duration, m RuntimeMode) (RuntimeMode, error) {
					if m.Name == "appliance" {
						return m, failure
					}
					return m, nil
				}
			}
			if stage == "log" {
				h.app.cycle.appendCycleLog = func(p Paths, e CycleLog) error {
					if e.FailureCategory == "" {
						return failure
					}
					return appendCycleLog(p, e)
				}
			}
			if stage == "save" {
				h.app.cycle.saveState = func(p Paths, s State) error {
					if s.ConsecutiveFailures == 0 {
						return failure
					}
					return saveState(p, s)
				}
			}
			for i, delay := range []time.Duration{5, 10, 20} {
				if err := h.app.Run([]string{"run-once"}); err == nil || !strings.Contains(err.Error(), failure.Error()) {
					t.Fatal(err)
				}
				state := h.state()
				if state.ConsecutiveFailures != i+1 || !state.NextAttemptAt.Equal(cycleTime.Add(delay*time.Minute)) {
					t.Fatalf("attempt%d: %+v", i+1, state)
				}
			}
		})
	}
}

func TestRestoreMarkerExcludesManualAndScheduledCycles(t *testing.T) {
	h := recoveryHarness(t)
	if err := beginRestore(h.paths); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"run-once", "run-scheduled"} {
		if err := h.app.Run([]string{command}); err != nil {
			t.Fatal(err)
		}
	}
	if len(h.events) != 0 {
		t.Fatal("restoration marker allowed a cycle", h.events)
	}
	unlock, acquired, err := cycleLock(h.paths)
	if err != nil || !acquired {
		t.Fatal(err)
	}
	if _, err := lockForRestore(h.paths); err == nil {
		t.Fatal("restore ignored already-running manual cycle")
	}
	unlock()
	if blocked, err := blockedForRestore(h.paths); err != nil || !blocked {
		t.Fatal("failed restore lost exclusion marker")
	}
	if err := finishRestore(h.paths); err != nil {
		t.Fatal(err)
	}
	h.errOut.Reset()
	if err := h.run(); err != nil {
		t.Fatal(err)
	}
}
