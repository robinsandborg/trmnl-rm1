//go:build linux

package trmnl

import (
	"errors"
	"fmt"
	"github.com/robinsandborg/rm1-trmnl/internal/appliance"
	"github.com/robinsandborg/rm1-trmnl/internal/network"
	"os"
)

// installDeps makes installation ordering observable without touching systemd
// or writing installed artifacts on the host.
type installDeps struct {
	ensureInterface func(Config)
	unitExists      func(string) bool
	warn            func(error)
	writeFile       func(string, []byte, os.FileMode) error
	executable      func() (string, error)
	sleepHookDir    func() (string, error)
	stockSyncUnit   func() (string, bool, error)
	unitEnabled     func(string) bool
	run             func([]string) error
	saveState       func(Paths, State) error
}

func (a *App) runInstall(paths Paths, args []string) error {
	return runInstallWithDeps(paths, args, installDeps{
		ensureInterface: func(cfg Config) { network.EnsureInterface(networkOptions(cfg)) },
		unitExists:      func(unit string) bool { return appliance.UnitExists(outputCommand, unit) },
		warn:            func(err error) { fmt.Fprintf(os.Stderr, "install-appliance: stock-noise masking partial: %v\n", err) },
		writeFile:       os.WriteFile,
		executable:      os.Executable,
		sleepHookDir:    detectSleepHookDir,
		stockSyncUnit:   detectStockSyncUnit,
		unitEnabled:     unitEnabled,
		run:             runCommand,
		saveState:       saveState,
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
	if deps.ensureInterface != nil {
		deps.ensureInterface(cfg)
	}
	if err := validateConfig(paths, cfg); err != nil {
		return err
	}

	state, err := loadState(paths)
	if err != nil {
		return err
	}

	return appliance.Install(snapshot(state), appliance.InstallDeps{UnitExists: deps.unitExists, Warn: deps.warn,
		WriteFile: deps.writeFile, Executable: deps.executable, SleepHookDir: deps.sleepHookDir, StockSyncUnit: deps.stockSyncUnit, UnitEnabled: deps.unitEnabled, Run: deps.run,
		SaveState: func(meta appliance.Snapshot) error {
			state.MaskedNoise = meta.MaskedNoise
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

	// Stock mode must regain the radio that appliance cleanup unbound.
	// Invalid app configuration must not prevent restoring the stock UI.
	cfg, _ := loadConfig(paths)
	return runRestoreWithOps(state, applianceOps{
		restoreNetwork:     func() error { return network.BringUp(networkOptions(cfg), runCommand) },
		run:                runCommand,
		remove:             os.Remove,
		detectSleepHookDir: detectSleepHookDir,
	})
}

func detectSleepHookDir() (string, error)        { return appliance.DetectSleepHookDir() }
func detectStockSyncUnit() (string, bool, error) { return appliance.DetectStockSyncUnit(outputCommand) }
func unitEnabled(unit string) bool               { return appliance.UnitEnabled(outputCommand, unit) }
