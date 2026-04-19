//go:build linux

package trmnl

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func readBatterySample(cfg Config) (*BatterySample, error) {
	entries, err := os.ReadDir(cfg.powerSupplyPath())
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		base := filepath.Join(cfg.powerSupplyPath(), entry.Name())
		if _, err := os.Stat(filepath.Join(base, "capacity")); err == nil {
			sample := &BatterySample{
				Status:         readOptional(base, "status"),
				CapacityPct:    readOptional(base, "capacity"),
				VoltageMicroV:  readOptional(base, "voltage_now"),
				CurrentMicroA:  readOptional(base, "current_now"),
				TemperatureDec: readOptional(base, "temp"),
			}
			return sample, nil
		}
	}
	return nil, nil
}

func planNextCycle(cfg Config, interval time.Duration, mode RuntimeMode) (RuntimeMode, error) {
	if !mode.ShouldSuspend {
		if err := scheduleTransientRun(interval); err != nil {
			return mode, err
		}
		return mode, nil
	}

	if err := scheduleWakeAlarm(cfg, interval); err != nil {
		fallback := RuntimeMode{Name: "awake-fallback", MaintenanceReason: "rtc-fallback", ShouldSuspend: false}
		if timerErr := scheduleTransientRun(interval); timerErr != nil {
			return mode, fmt.Errorf("schedule wake alarm: %w; transient timer fallback failed: %v", err, timerErr)
		}
		return fallback, nil
	}

	return mode, nil
}

func suspendDevice(cfg Config) error {
	if len(cfg.SuspendCommand) > 0 {
		return runCommand(cfg.SuspendCommand)
	}
	return firstSuccessful(
		[]string{"systemctl", "suspend"},
		[]string{"sh", "-c", "echo mem > /sys/power/state"},
	)
}

func scheduleWakeAlarm(cfg Config, interval time.Duration) error {
	wakePath := cfg.rtcWakealarmPath()
	seconds := int(interval.Seconds())
	if seconds <= 0 {
		return fmt.Errorf("invalid wake interval: %s", interval)
	}

	if err := os.WriteFile(wakePath, []byte("0"), 0o644); err == nil {
		if err := os.WriteFile(wakePath, []byte(fmt.Sprintf("+%d", seconds)), 0o644); err == nil {
			return nil
		}
	}

	return firstSuccessful(
		[]string{"rtcwake", "-m", "no", "-s", strconv.Itoa(seconds)},
	)
}

func scheduleTransientRun(interval time.Duration) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	return scheduleTransientRunWithRunner(runCommand, exePath, interval)
}

func scheduleTransientRunWithRunner(run func([]string) error, exePath string, interval time.Duration) error {
	seconds := int(interval.Seconds())
	if seconds <= 0 {
		seconds = int(defaultRefreshFallback.Seconds())
	}

	// Replace any stale schedule from a previous cycle. Older systemd versions
	// (RM1) lack `systemd-run --replace`, so we stop the unit first and ignore
	// any "not loaded" error.
	_ = run([]string{"systemctl", "stop", "trmnl-rm1-next.timer", "trmnl-rm1-next.service"})
	return run([]string{
		"systemd-run",
		"--unit=trmnl-rm1-next",
		"--on-active=" + strconv.Itoa(seconds),
		"--property=Type=oneshot",
		"--property=Environment=HOME=/home/root",
		exePath,
		"run-once",
	})
}

func readOptional(base, name string) string {
	data, err := os.ReadFile(filepath.Join(base, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
