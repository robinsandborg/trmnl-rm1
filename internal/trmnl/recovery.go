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
	if state.ScheduleBootID != "" && state.ScheduleBootID == a.cycle.bootID() && a.now().Add(30*time.Second).Before(state.NextAttemptAt) {
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

func restoreMarkerPath(paths Paths) string {
	return filepath.Join(paths.StateDir, "restore-in-progress")
}
func blockedForRestore(paths Paths) (bool, error) {
	_, err := os.Stat(restoreMarkerPath(paths))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
func beginRestore(paths Paths) error {
	return os.WriteFile(restoreMarkerPath(paths), []byte("restore-stock in progress; retry restore-stock after an error\n"), 0600)
}
func finishRestore(paths Paths) error {
	err := os.Remove(restoreMarkerPath(paths))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
func lockForRestore(paths Paths) (func(), error) {
	unlock, acquired, err := cycleLock(paths)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, fmt.Errorf("a manual cycle is still running; restore remains blocked until restore-stock is retried")
	}
	return unlock, nil
}
