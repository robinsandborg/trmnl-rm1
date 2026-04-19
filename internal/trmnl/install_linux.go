//go:build linux

package trmnl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) runInstall(paths Paths, args []string) error {
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

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	sleepDir, err := detectSleepHookDir()
	if err != nil {
		return err
	}
	hookPath := filepath.Join(sleepDir, applianceResumeHookName)

	if err := os.WriteFile(applianceServicePath, []byte(renderApplianceService(exePath)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(hookPath, []byte(renderResumeHook()), 0o755); err != nil {
		return err
	}

	syncUnit, syncEnabled, err := detectStockSyncUnit()
	if err != nil {
		return err
	}
	state.StockSyncUnit = syncUnit
	state.SyncWasEnabled = syncEnabled
	state.XochitlWasEnabled = unitEnabled("xochitl.service")

	if err := runCommand([]string{"systemctl", "daemon-reload"}); err != nil {
		return err
	}
	if err := disableForAppliance("xochitl.service"); err != nil {
		return err
	}
	if syncUnit != "" {
		if err := disableForAppliance(syncUnit); err != nil {
			return err
		}
	}
	if err := runCommand([]string{"systemctl", "enable", applianceServiceName}); err != nil {
		return err
	}
	if err := runCommand([]string{"systemctl", "start", applianceServiceName}); err != nil {
		return err
	}
	return saveState(paths, state)
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
	return `#!/bin/sh
case "$1" in
  post)
    /bin/systemctl start trmnl-rm1-appliance.service >/dev/null 2>&1 || true
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
