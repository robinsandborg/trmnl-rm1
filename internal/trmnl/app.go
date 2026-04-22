package trmnl

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type App struct {
	stdout io.Writer
	stderr io.Writer
	now    func() time.Time
}

func NewApp(stdout, stderr io.Writer) *App {
	return &App{
		stdout: stdout,
		stderr: stderr,
		now:    time.Now,
	}
}

func (a *App) Run(args []string) error {
	paths, err := defaultPaths()
	if err != nil {
		return err
	}
	if err := ensureRuntimeDirs(paths); err != nil {
		return err
	}

	if len(args) == 0 {
		return a.usageError()
	}

	switch args[0] {
	case "validate":
		return a.runValidate(paths)
	case "print-device-id":
		return a.runPrintDeviceID(paths)
	case "run-once":
		return a.runOnce(paths)
	case "install-appliance":
		return a.runInstall(paths, args[1:])
	case "restore-stock":
		return a.runRestore(paths)
	default:
		return a.usageError()
	}
}

func (a *App) usageError() error {
	return errors.New("usage: trmnl-rm1 <validate|print-device-id|run-once|install-appliance|restore-stock>")
}

func (a *App) runValidate(paths Paths) error {
	cfg, err := loadConfig(paths)
	if err != nil {
		return err
	}
	if err := validateConfig(paths, cfg); err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, "config is valid")
	return nil
}

func (a *App) runPrintDeviceID(paths Paths) error {
	cfg, err := loadConfig(paths)
	if err != nil {
		return err
	}
	deviceID, err := resolveDeviceID(cfg)
	if err != nil {
		return err
	}
	fmt.Fprintln(a.stdout, deviceID)
	return nil
}

func (a *App) runOnce(paths Paths) error {
	cfg, err := loadConfig(paths)
	if err != nil {
		return err
	}
	if err := validateConfig(paths, cfg); err != nil {
		return err
	}

	state, err := loadState(paths)
	if err != nil {
		return err
	}

	startedAt := a.now().UTC()
	battery, _ := readBatterySample(cfg)
	runtimeMode, err := determineRuntimeMode(paths, cfg, state, a.now())
	if err != nil {
		return err
	}

	client, cleanupNetwork, err := a.prepareNetwork(cfg)
	if err != nil {
		return a.finishCycle(paths, cfg, state, CycleLog{
			StartedAt:        startedAt,
			EndedAt:          a.now().UTC(),
			Mode:             runtimeMode.Name,
			Battery:          battery,
			FailureCategory:  "wifi",
			FailureMessage:   err.Error(),
			ConsecutiveFails: state.ConsecutiveFailures + 1,
		}, &cycleError{Category: "wifi", Err: err})
	}
	networkCleaned := false
	defer func() {
		if !networkCleaned {
			cleanupNetwork()
		}
	}()

	terminal, imageBytes, resolvedImageURL, interval, err := fetchCyclePayload(client, cfg, battery)
	if err != nil {
		ce := classifyCycleError("http", err)
		return a.finishCycle(paths, cfg, state, CycleLog{
			StartedAt:        startedAt,
			EndedAt:          a.now().UTC(),
			Mode:             runtimeMode.Name,
			Battery:          battery,
			FailureCategory:  ce.Category,
			FailureMessage:   ce.Error(),
			ConsecutiveFails: state.ConsecutiveFailures + 1,
		}, ce)
	}

	hashValue := sha256Hex(imageBytes)
	changed := hashValue != state.LastImageHash
	skipped := !changed
	fullRefresh := changed && shouldUseFullRefresh(state.RenderedUpdates, cfg.fullRefreshEvery())

	if err := os.WriteFile(paths.DownloadedImage, imageBytes, 0o600); err != nil {
		return err
	}

	if changed {
		renderMode := RefreshPartial
		if fullRefresh {
			renderMode = RefreshFull
		}
		if err := renderImage(cfg, imageBytes, paths.LastRenderedImage, renderMode); err != nil {
			ce := classifyCycleError("render", err)
			return a.finishCycle(paths, cfg, state, CycleLog{
				StartedAt:        startedAt,
				EndedAt:          a.now().UTC(),
				Mode:             runtimeMode.Name,
				ImageHash:        hashValue,
				ImageURL:         resolvedImageURL,
				RefreshIntervalS: int(interval.Seconds()),
				Battery:          battery,
				FailureCategory:  ce.Category,
				FailureMessage:   ce.Error(),
				ConsecutiveFails: state.ConsecutiveFailures + 1,
				FullRefresh:      fullRefresh,
			}, ce)
		}
		state.LastImageHash = hashValue
		state.LastImageURL = resolvedImageURL
		state.LastFilename = terminal.Filename
		state.RenderedUpdates++
	}

	state.LastCycleChanged = changed
	state.LastIntervalSeconds = int(interval.Seconds())
	state.LastMode = runtimeMode.Name
	state.LastSuccessAt = a.now().UTC()
	state.LastFailureAt = time.Time{}
	state.LastFailureCategory = ""
	state.LastFailureMessage = ""
	state.ConsecutiveFailures = 0

	effectiveMode, err := planNextCycle(cfg, interval, runtimeMode)
	if err != nil {
		ce := classifyCycleError("schedule", err)
		return a.finishCycle(paths, cfg, state, CycleLog{
			StartedAt:        startedAt,
			EndedAt:          a.now().UTC(),
			Mode:             runtimeMode.Name,
			ChangedScreen:    changed,
			ImageHash:        hashValue,
			ImageURL:         resolvedImageURL,
			RefreshIntervalS: int(interval.Seconds()),
			Battery:          battery,
			SkippedRender:    skipped,
			FullRefresh:      fullRefresh,
			FailureCategory:  ce.Category,
			FailureMessage:   ce.Error(),
			ConsecutiveFails: state.ConsecutiveFailures + 1,
		}, ce)
	}

	logEntry := CycleLog{
		StartedAt:         startedAt,
		EndedAt:           a.now().UTC(),
		Mode:              effectiveMode.Name,
		ChangedScreen:     changed,
		ImageHash:         hashValue,
		ImageURL:          resolvedImageURL,
		RefreshIntervalS:  int(interval.Seconds()),
		Battery:           battery,
		SkippedRender:     skipped,
		FullRefresh:       fullRefresh,
		ConsecutiveFails:  state.ConsecutiveFailures,
		MaintenanceReason: effectiveMode.MaintenanceReason,
	}

	if err := appendCycleLog(paths, logEntry); err != nil {
		return err
	}
	if err := saveState(paths, state); err != nil {
		return err
	}

	cleanupNetwork()
	networkCleaned = true

	if effectiveMode.ShouldSuspend {
		if err := suspendDevice(cfg); err != nil {
			return a.finishCycle(paths, cfg, state, CycleLog{
				StartedAt:        startedAt,
				EndedAt:          a.now().UTC(),
				Mode:             effectiveMode.Name,
				ChangedScreen:    changed,
				ImageHash:        hashValue,
				ImageURL:         resolvedImageURL,
				RefreshIntervalS: int(interval.Seconds()),
				Battery:          battery,
				SkippedRender:    skipped,
				FullRefresh:      fullRefresh,
			}, &cycleError{Category: "suspend", Err: err})
		}
	}
	return nil
}

func (a *App) prepareNetwork(cfg Config) (*http.Client, func(), error) {
	cleanup := func() {}
	if cfg.DisableWiFiBetweenUpdates {
		if err := bringWiFiUp(cfg); err != nil {
			return nil, cleanup, err
		}
		cleanup = func() {
			_ = bringWiFiDown(cfg)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.wifiTimeout())
	defer cancel()
	if err := waitForConnectivity(ctx, cfg); err != nil {
		cleanup()
		return nil, func() {}, err
	}

	return &http.Client{Timeout: cfg.wifiTimeout()}, cleanup, nil
}

func (a *App) finishCycle(paths Paths, cfg Config, state State, entry CycleLog, err error) error {
	ended := a.now().UTC()
	entry.EndedAt = ended
	state.LastFailureAt = ended
	if ce, ok := err.(*cycleError); ok {
		state.LastFailureCategory = ce.Category
		state.LastFailureMessage = ce.Error()
		entry.FailureCategory = ce.Category
		entry.FailureMessage = ce.Error()
	} else {
		state.LastFailureMessage = err.Error()
		entry.FailureMessage = err.Error()
	}
	state.ConsecutiveFailures++
	state.LastMode = entry.Mode
	entry.ConsecutiveFails = state.ConsecutiveFailures

	if state.ConsecutiveFailures >= cfg.failureThreshold() {
		entry.MaintenanceReason = "failure-threshold"
	}

	if logErr := appendCycleLog(paths, entry); logErr != nil {
		return errors.Join(err, logErr)
	}
	if saveErr := saveState(paths, state); saveErr != nil {
		return errors.Join(err, saveErr)
	}

	runtimeMode, modeErr := determineRuntimeMode(paths, cfg, state, a.now())
	if modeErr == nil {
		_, _ = planNextCycle(cfg, cfg.refreshFallback(), runtimeMode)
	}
	return err
}

func fetchCyclePayload(client *http.Client, cfg Config, battery *BatterySample) (TerminalResponse, []byte, string, time.Duration, error) {
	deviceID, err := resolveDeviceID(cfg)
	if err != nil {
		return TerminalResponse{}, nil, "", 0, err
	}

	apiURL := strings.TrimRight(cfg.BaseURL, "/") + "/api/display"
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return TerminalResponse{}, nil, "", 0, err
	}
	req.Header.Set("ID", deviceID)
	if cfg.AccessToken != "" {
		req.Header.Set("access-token", cfg.AccessToken)
	}
	req.Header.Set("User-Agent", "trmnl-rm1/0.1.0")
	if v := batteryVoltageHeader(battery); v != "" {
		req.Header.Set("Battery-Voltage", v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return TerminalResponse{}, nil, "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return TerminalResponse{}, nil, "", 0, fmt.Errorf("display endpoint returned %s", resp.Status)
	}

	var terminal TerminalResponse
	if err := json.NewDecoder(resp.Body).Decode(&terminal); err != nil {
		return TerminalResponse{}, nil, "", 0, fmt.Errorf("parse display response: %w", err)
	}

	if terminal.ImageURL == "" {
		return TerminalResponse{}, nil, "", 0, errors.New("display response missing image_url")
	}

	imageURL := terminal.ImageURL
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		base, err := url.Parse(cfg.BaseURL)
		if err != nil {
			return TerminalResponse{}, nil, "", 0, err
		}
		ref, err := url.Parse(imageURL)
		if err != nil {
			return TerminalResponse{}, nil, "", 0, err
		}
		imageURL = base.ResolveReference(ref).String()
	}

	imgResp, err := client.Get(imageURL)
	if err != nil {
		return TerminalResponse{}, nil, "", 0, fmt.Errorf("download image: %w", err)
	}
	defer imgResp.Body.Close()
	if imgResp.StatusCode != http.StatusOK {
		return TerminalResponse{}, nil, "", 0, fmt.Errorf("image download returned %s", imgResp.Status)
	}
	imageBytes, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return TerminalResponse{}, nil, "", 0, err
	}

	interval := clampRefresh(terminal.RefreshRate, cfg)
	return terminal, imageBytes, imageURL, interval, nil
}

func clampRefresh(refreshRate int, cfg Config) time.Duration {
	interval := cfg.refreshFallback()
	if refreshRate > 0 {
		interval = time.Duration(refreshRate) * time.Second
	}
	if interval < cfg.refreshMin() {
		interval = cfg.refreshMin()
	}
	if interval > cfg.refreshMax() {
		interval = cfg.refreshMax()
	}
	return interval
}

func appendCycleLog(paths Paths, entry CycleLog) error {
	f, err := os.OpenFile(paths.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

func classifyCycleError(category string, err error) *cycleError {
	if ce, ok := err.(*cycleError); ok {
		return ce
	}
	return &cycleError{Category: category, Err: err}
}

func sha256Hex(data []byte) string {
	return fmt.Sprintf("%x", sha256Sum(data))
}

func sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}

func shouldUseFullRefresh(renderedUpdates, fullRefreshEvery int) bool {
	return fullRefreshEvery > 0 && (renderedUpdates+1)%fullRefreshEvery == 0
}

// batteryVoltageHeader converts a BatterySample's raw sysfs microvolts string
// into the volts float string expected by the Battery-Voltage request header
// (e.g. "3900000" → "3.90"). Returns "" when the sample is unavailable or
// unparseable so callers can omit the header gracefully.
func batteryVoltageHeader(b *BatterySample) string {
	if b == nil || b.VoltageMicroV == "" {
		return ""
	}
	microV, err := strconv.ParseInt(b.VoltageMicroV, 10, 64)
	if err != nil || microV <= 0 {
		return ""
	}
	volts := float64(microV) / 1_000_000.0
	return fmt.Sprintf("%.2f", volts)
}
