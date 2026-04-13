package trmnl

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

const (
	applianceServiceName    = "trmnl-rm1-appliance.service"
	applianceServicePath    = "/etc/systemd/system/trmnl-rm1-appliance.service"
	applianceResumeHookName = "trmnl-rm1-resume"
)

type applianceOps struct {
	run                func([]string) error
	remove             func(string) error
	detectSleepHookDir func() (string, error)
}

func disableForAppliance(unit string) error {
	return disableForApplianceWithRunner(runCommand, unit)
}

func disableForApplianceWithRunner(run func([]string) error, unit string) error {
	var errs []error
	for _, step := range []struct {
		label string
		cmd   []string
	}{
		{label: "stop", cmd: []string{"systemctl", "stop", unit}},
		{label: "disable", cmd: []string{"systemctl", "disable", unit}},
		{label: "mask", cmd: []string{"systemctl", "mask", unit}},
	} {
		if err := run(step.cmd); err != nil {
			errs = append(errs, fmt.Errorf("%s %s: %w", step.label, unit, err))
		}
	}
	return joinErrors(errs...)
}

func runRestoreWithOps(state State, ops applianceOps) error {
	var errs []error

	if err := ops.run([]string{"systemctl", "disable", "--now", applianceServiceName}); err != nil {
		errs = append(errs, fmt.Errorf("disable appliance service %s: %w", applianceServiceName, err))
	}
	if err := removeIfExists(ops.remove, applianceServicePath); err != nil {
		errs = append(errs, fmt.Errorf("remove appliance unit file %s: %w", applianceServicePath, err))
	}

	sleepDir, err := ops.detectSleepHookDir()
	if err != nil {
		errs = append(errs, fmt.Errorf("locate systemd system-sleep directory: %w", err))
	} else {
		hookPath := filepath.Join(sleepDir, applianceResumeHookName)
		if err := removeIfExists(ops.remove, hookPath); err != nil {
			errs = append(errs, fmt.Errorf("remove appliance resume hook %s: %w", hookPath, err))
		}
	}

	if err := ops.run([]string{"systemctl", "daemon-reload"}); err != nil {
		errs = append(errs, fmt.Errorf("reload systemd units: %w", err))
	}

	// restore-stock is defined as returning the tablet to stock UI behavior, so
	// xochitl must be enabled even if the saved install-time state is incomplete.
	errs = append(errs, restoreUnitWithRunner(ops.run, "xochitl.service", true)...)
	if state.StockSyncUnit != "" {
		errs = append(errs, restoreUnitWithRunner(ops.run, state.StockSyncUnit, state.SyncWasEnabled)...)
	}

	return joinErrors(errs...)
}

func restoreUnitWithRunner(run func([]string) error, unit string, enable bool) []error {
	var errs []error
	if err := run([]string{"systemctl", "unmask", unit}); err != nil {
		errs = append(errs, fmt.Errorf("unmask %s: %w", unit, err))
	}
	if enable {
		if err := run([]string{"systemctl", "enable", "--now", unit}); err != nil {
			errs = append(errs, fmt.Errorf("enable and start %s: %w", unit, err))
		}
	}
	return errs
}

func removeIfExists(remove func(string) error, path string) error {
	err := remove(path)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		// An already-absent artifact is safe to ignore because restore has
		// already reached the desired filesystem state for that path.
		return nil
	}
	return err
}

func joinErrors(errs ...error) error {
	var nonNil []error
	for _, err := range errs {
		if err != nil {
			nonNil = append(nonNil, err)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	return errors.Join(nonNil...)
}
