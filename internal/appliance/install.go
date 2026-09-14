package appliance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Snapshot struct {
	MaskedNoise                       map[string]bool
	StockSyncUnit                     string
	SyncWasEnabled, XochitlWasEnabled bool
}
type InstallDeps struct {
	UnitExists    func(string) bool
	Warn          func(error)
	WriteFile     func(string, []byte, os.FileMode) error
	Executable    func() (string, error)
	SleepHookDir  func() (string, error)
	StockSyncUnit func() (string, bool, error)
	UnitEnabled   func(string) bool
	Run           func([]string) error
	SaveState     func(Snapshot) error
}

func Install(state Snapshot, deps InstallDeps) error {
	exePath, err := deps.Executable()
	if err != nil {
		return err
	}
	sleepDir, err := deps.SleepHookDir()
	if err != nil {
		return err
	}
	hookPath := filepath.Join(sleepDir, ResumeHookName)

	if err := deps.WriteFile(ServicePath, []byte(RenderService(exePath)), 0o644); err != nil {
		return err
	}
	if err := deps.WriteFile(hookPath, []byte(RenderResumeHook()), 0o755); err != nil {
		return err
	}

	syncUnit, syncEnabled, err := deps.StockSyncUnit()
	if err != nil {
		return err
	}
	if state.StockSyncUnit == "" && state.MaskedNoise == nil && !state.XochitlWasEnabled {
		state.StockSyncUnit = syncUnit
		state.SyncWasEnabled = syncEnabled
		state.XochitlWasEnabled = deps.UnitEnabled("xochitl.service")
	}

	if err := deps.Run([]string{"systemctl", "daemon-reload"}); err != nil {
		return err
	}
	if err := Disable(deps.Run, "xochitl.service"); err != nil {
		return err
	}
	if syncUnit != "" {
		if err := Disable(deps.Run, syncUnit); err != nil {
			return err
		}
	}
	if deps.UnitExists != nil {
		saved := make(map[string]bool, len(state.MaskedNoise))
		for unit, enabled := range state.MaskedNoise {
			saved[unit] = enabled
		}
		for _, unit := range stockNoiseUnits {
			if !deps.UnitExists(unit) {
				continue
			}
			if _, ok := saved[unit]; !ok {
				saved[unit] = deps.UnitEnabled(unit)
			}
			if err := Disable(deps.Run, unit); err != nil && deps.Warn != nil {
				deps.Warn(err)
			}
		}
		if len(saved) > 0 {
			state.MaskedNoise = saved
		}
	}
	if err := deps.Run([]string{"systemctl", "enable", ServiceName}); err != nil {
		return err
	}
	// Persist stock-service metadata before the first run. `systemctl start`
	// on a oneshot service blocks until the service exits, and run-once
	// saves its own state at the end of that run. If we save after the start
	// call instead, we overwrite whatever run-once just persisted.
	if err := deps.SaveState(state); err != nil {
		return err
	}
	return deps.Run([]string{"systemctl", "start", ServiceName})
}

func DetectSleepHookDir() (string, error) {
	for _, dir := range []string{"/usr/lib/systemd/system-sleep", "/lib/systemd/system-sleep"} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}
	return "", errors.New("unable to locate systemd system-sleep directory")
}

func RenderService(exePath string) string {
	return strings.TrimSpace(fmt.Sprintf(`
[Unit]
Description=TRMNL RM1 appliance cycle
After=network.target
Wants=network.target
RequiresMountsFor=/home/root

[Service]
Type=oneshot
Environment=HOME=/home/root
ExecStart=%s run-once
User=root

[Install]
WantedBy=multi-user.target
`, exePath)) + "\n"
}

func RenderResumeHook() string {
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

func DetectStockSyncUnit(outputCommand func([]string) (string, error)) (string, bool, error) {
	for _, unit := range []string{"sync.service", "rm-sync.service"} {
		if UnitExists(outputCommand, unit) {
			return unit, UnitEnabled(outputCommand, unit), nil
		}
	}
	return "", false, nil
}

func UnitExists(outputCommand func([]string) (string, error), unit string) bool {
	_, err := outputCommand([]string{"systemctl", "status", unit})
	return err == nil || strings.Contains(err.Error(), "Loaded:")
}

func UnitEnabled(outputCommand func([]string) (string, error), unit string) bool {
	out, err := outputCommand([]string{"systemctl", "is-enabled", unit})
	return err == nil && out == "enabled"
}
