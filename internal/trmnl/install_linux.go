//go:build linux

package trmnl

import (
	"errors"
	"github.com/robinsandborg/rm1-trmnl/internal/appliance"
	"os"
)

// installDeps makes installation ordering observable without touching systemd
// or writing installed artifacts on the host.
type installDeps struct {
	writeFile     func(string, []byte, os.FileMode) error
	executable    func() (string, error)
	sleepHookDir  func() (string, error)
	stockSyncUnit func() (string, bool, error)
	unitEnabled   func(string) bool
	run           func([]string) error
	saveState     func(Paths, State) error
}

func (a *App) runInstall(paths Paths, args []string) error {
	return runInstallWithDeps(paths, args, installDeps{
		writeFile:     os.WriteFile,
		executable:    os.Executable,
		sleepHookDir:  detectSleepHookDir,
		stockSyncUnit: detectStockSyncUnit,
		unitEnabled:   unitEnabled,
		run:           runCommand,
		saveState:     saveState,
	})
}

func runInstallWithDeps(paths Paths, args []string, deps installDeps) error {
	if len(args) > 0 {
		return errors.New("install-appliance does not accept arguments")
	}

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

	return appliance.Install(snapshot(state), appliance.InstallDeps{
		WriteFile: deps.writeFile, Executable: deps.executable, SleepHookDir: deps.sleepHookDir, StockSyncUnit: deps.stockSyncUnit, UnitEnabled: deps.unitEnabled, Run: deps.run,
		SaveState: func(meta appliance.Snapshot) error {
			state.StockSyncUnit = meta.StockSyncUnit
			state.SyncWasEnabled = meta.SyncWasEnabled
			state.XochitlWasEnabled = meta.XochitlWasEnabled
			return deps.saveState(paths, state)
		},
	})
}

func (a *App) runRestore(paths Paths) error {
	state, err := loadState(paths)
	if err != nil {
		return err
	}

	return runRestoreWithOps(state, applianceOps{
		run:                runCommand,
		remove:             os.Remove,
		detectSleepHookDir: detectSleepHookDir,
	})
}

func detectSleepHookDir() (string, error)        { return appliance.DetectSleepHookDir() }
func detectStockSyncUnit() (string, bool, error) { return appliance.DetectStockSyncUnit(outputCommand) }
func unitEnabled(unit string) bool               { return appliance.UnitEnabled(outputCommand, unit) }
