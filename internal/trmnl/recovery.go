package trmnl

import (
	"fmt"
	"github.com/robinsandborg/rm1-trmnl/internal/cycle"
	"github.com/robinsandborg/rm1-trmnl/internal/power"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bootID() string {
	b, _ := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return strings.TrimSpace(string(b))
}
func (c Config) batteryPolicy() power.BatteryPolicy {
	return power.BatteryPolicy{Low: c.BatteryLowPercent, Recovery: c.BatteryRecoveryPercent, Critical: c.BatteryCriticalPercent, CheckInterval: time.Duration(c.BatteryCheckSeconds) * time.Second, Shutdown: c.CriticalBatteryShutdown}.Defaults()
}

func (a *App) runScheduled(paths Paths) error {
	state, err := loadState(paths)
	if err != nil {
		return fmt.Errorf("state: recovery timer will retry: %w", err)
	}
	if state.BootID != "" && state.BootID == a.cycle.bootID() && a.now().Add(30*time.Second).Before(state.NextAttemptAt) {
		return nil
	}
	return a.runOnce(paths)
}
func (a *App) cycleOptions(paths Paths, cfg Config) cycle.Options {
	opts := cycle.Options{DownloadedImage: paths.DownloadedImage, LastRenderedImage: paths.LastRenderedImage, FullRefreshEvery: cfg.fullRefreshEvery(), FailureThreshold: cfg.failureThreshold(), RefreshFallback: cfg.refreshFallback()}
	if a.cycle.bootID != nil {
		opts.Recovery = true
		opts.BootID = a.cycle.bootID()
		opts.BatteryPolicy = cfg.batteryPolicy()
	}
	return opts
}
func cycleLock(paths Paths) (func(), bool, error) {
	return acquireCycleLock(filepath.Join(paths.StateDir, "cycle.lock"))
}
