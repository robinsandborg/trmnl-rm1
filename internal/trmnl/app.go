package trmnl

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
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

	return cycle.Run(cycle.Options{DownloadedImage: paths.DownloadedImage, LastRenderedImage: paths.LastRenderedImage, FullRefreshEvery: cfg.fullRefreshEvery(), FailureThreshold: cfg.failureThreshold(), RefreshFallback: cfg.refreshFallback()}, cycle.State(state), a.cycleOperations(paths, cfg))
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
