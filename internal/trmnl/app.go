package trmnl

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/robinsandborg/rm1-trmnl/internal/cycle"
	"github.com/robinsandborg/rm1-trmnl/internal/network"
	"github.com/robinsandborg/rm1-trmnl/internal/storage"
)

type App struct {
	stdout io.Writer
	stderr io.Writer
	now    func() time.Time
	cycle  cycleDeps
}

func NewApp(stdout, stderr io.Writer) *App {
	app := &App{
		stdout: stdout,
		stderr: stderr,
		now:    time.Now,
	}
	app.cycle = defaultCycleDeps(app.prepareNetwork)
	return app
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
	case "run-once", "run-scheduled":
		unlock, acquired, err := cycleLock(paths)
		if err != nil {
			return err
		}
		if !acquired {
			fmt.Fprintln(a.stderr, "cycle already running; recovery timer retains ownership")
			return nil
		}
		defer unlock()
		blocked, err := blockedForRestore(paths)
		if err != nil {
			return err
		}
		if blocked {
			fmt.Fprintln(a.stderr, "cycle skipped: restore-stock is in progress")
			return nil
		}
		if args[0] == "run-scheduled" {
			return a.runScheduled(paths)
		}
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
	if strings.TrimSpace(cfg.DeviceID) == "" && a.cycle.ensureInterface != nil {
		a.cycle.ensureInterface(cfg)
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
	if strings.TrimSpace(cfg.DeviceID) == "" && a.cycle.ensureInterface != nil {
		a.cycle.ensureInterface(cfg)
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
		return fmt.Errorf("configuration: safety timer retries in at most 5 minutes: %w", err)
	}
	// Identity is resolved after network acquisition. Local battery/recovery
	// rendering must never enumerate or leave the radio powered up.
	validationConfig := cfg
	if strings.TrimSpace(validationConfig.DeviceID) == "" {
		validationConfig.DeviceID = "deferred-wireless-identity"
	}
	if err := validateConfig(paths, validationConfig); err != nil {
		return fmt.Errorf("configuration: safety timer retries in at most 5 minutes: %w", err)
	}

	state, err := loadState(paths)
	if err != nil {
		return fmt.Errorf("state: safety timer will retry: %w", err)
	}

	return cycle.Run(a.cycleOptions(paths, cfg), cycle.State(state), a.cycleOperations(paths, cfg))
}

func (a *App) prepareNetwork(cfg Config) (*http.Client, func(), error) {
	return prepareNetworkWithDeps(cfg, networkDeps{
		bringUp:   bringWiFiUp,
		bringDown: bringWiFiDown,
		wait:      waitForConnectivity,
	})
}

func prepareNetworkWithDeps(cfg Config, deps networkDeps) (*http.Client, func(), error) {
	return network.Prepare(networkOptions(cfg), network.Operations{
		BringUp:   func() error { return deps.bringUp(cfg) },
		BringDown: func() error { return deps.bringDown(cfg) },
		Wait:      func(ctx context.Context) error { return deps.wait(ctx, cfg) },
	})
}

func appendCycleLog(paths Paths, entry CycleLog) error {
	return storage.AppendJSON(paths.LogFile, entry)
}

func sha256Hex(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func shouldUseFullRefresh(renderedUpdates, fullRefreshEvery int) bool {
	return cycle.ShouldUseFullRefresh(renderedUpdates, fullRefreshEvery)
}
