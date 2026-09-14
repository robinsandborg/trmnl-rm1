//go:build linux

package trmnl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	exePath, err := deps.executable()
	if err != nil {
		return err
	}
	sleepDir, err := deps.sleepHookDir()
	if err != nil {
		return err
	}
	hookPath := filepath.Join(sleepDir, applianceResumeHookName)

	if err := deps.writeFile(applianceServicePath, []byte(renderApplianceService(exePath)), 0o644); err != nil {
		return err
	}
	if err := deps.writeFile(hookPath, []byte(renderResumeHook()), 0o755); err != nil {
		return err
	}

	syncUnit, syncEnabled, err := deps.stockSyncUnit()
	if err != nil {
		return err
	}
	state.StockSyncUnit = syncUnit
	state.SyncWasEnabled = syncEnabled
	state.XochitlWasEnabled = deps.unitEnabled("xochitl.service")

	if err := deps.run([]string{"systemctl", "daemon-reload"}); err != nil {
		return err
	}
	if err := disableForApplianceWithRunner(deps.run, "xochitl.service"); err != nil {
		return err
	}
	if syncUnit != "" {
		if err := disableForApplianceWithRunner(deps.run, syncUnit); err != nil {
			return err
		}
	}
	if err := deps.run([]string{"systemctl", "enable", applianceServiceName}); err != nil {
		return err
	}
	// Persist stock-service metadata before the first run. `systemctl start`
	// on a oneshot service blocks until the service exits, and run-once
	// saves its own state at the end of that run. If we save after the start
	// call instead, we overwrite whatever run-once just persisted.
	if err := deps.saveState(paths, state); err != nil {
		return err
	}
	return deps.run([]string{"systemctl", "start", applianceServiceName})
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

func detectSleepHookDir() (string, error) {
	for _, dir := range []string{"/usr/lib/systemd/system-sleep", "/lib/systemd/system-sleep"} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}
	return "", errors.New("unable to locate systemd system-sleep directory")
}

func renderApplianceService(exePath string) string {
	return strings.TrimSpace(fmt.Sprintf(`
[Unit]
Description=TRMNL RM1 appliance cycle
After=network.target
Wants=network.target

[Service]
Type=oneshot
Environment=HOME=/home/root
ExecStart=%s run-once
User=root

[Install]
WantedBy=multi-user.target
`, exePath)) + "\n"
}

func renderResumeHook() string {
	// --no-block is critical: systemctl start on a Type=oneshot service
	// otherwise blocks until ExecStart exits. Blocking inside a
	// system-sleep hook holds the current suspend/resume lifecycle open,
	// so when run-once eventually calls `systemctl suspend` again it
	// nests inside the not-yet-unwound outer suspend. The next wake then
	// tears down the outer wrapper without ever firing post-hooks, and
	// the appliance service never starts again.
	return `#!/bin/sh
case "$1" in
  post)
    /bin/systemctl start --no-block trmnl-rm1-appliance.service >/dev/null 2>&1 || true
    ;;
esac
`
}

func detectStockSyncUnit() (string, bool, error) {
	for _, unit := range []string{"sync.service", "rm-sync.service"} {
		if unitExists(unit) {
			return unit, unitEnabled(unit), nil
		}
	}
	return "", false, nil
}

func unitExists(unit string) bool {
	_, err := outputCommand([]string{"systemctl", "status", unit})
	return err == nil || strings.Contains(err.Error(), "Loaded:")
}

func unitEnabled(unit string) bool {
	out, err := outputCommand([]string{"systemctl", "is-enabled", unit})
	return err == nil && out == "enabled"
}
