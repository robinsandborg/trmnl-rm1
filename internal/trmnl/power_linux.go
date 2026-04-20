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
	return scheduleTransientRunWithRunner(runCommand, exePath, readSelfUnit(), interval)
}

const (
	transientNextUnitA = "trmnl-rm1-next-a"
	transientNextUnitB = "trmnl-rm1-next-b"
)

// scheduleTransientRunWithRunner arms the next awake-mode cycle. RM1's
// systemd predates `systemd-run --replace`, and a cycle running *inside*
// `trmnl-rm1-next-X.service` can't stop or recreate its own unit without
// SIGTERM'ing itself. So we alternate between two unit names and always
// target the one we are NOT running inside.
func scheduleTransientRunWithRunner(run func([]string) error, exePath, selfUnit string, interval time.Duration) error {
	seconds := int(interval.Seconds())
	if seconds <= 0 {
		seconds = int(defaultRefreshFallback.Seconds())
	}

	target := transientNextUnitA
	if selfUnit == transientNextUnitA {
		target = transientNextUnitB
	}

	_ = run([]string{"systemctl", "stop", target + ".timer", target + ".service"})
	_ = run([]string{"systemctl", "reset-failed", target + ".timer", target + ".service"})
	return run([]string{
		"systemd-run",
		"--unit=" + target,
		"--on-active=" + strconv.Itoa(seconds),
		"--property=Type=oneshot",
		"--property=Environment=HOME=/home/root",
		exePath,
		"run-once",
	})
}

// readSelfUnit returns the systemd unit name this process is running inside,
// derived from /proc/self/cgroup. Empty string if unavailable or not under
// a systemd-managed cgroup.
func readSelfUnit() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		path := parts[2]
		idx := strings.LastIndex(path, "/")
		if idx < 0 {
			continue
		}
		name := path[idx+1:]
		if strings.HasSuffix(name, ".service") {
			return strings.TrimSuffix(name, ".service")
		}
	}
	return ""
}

func readOptional(base, name string) string {
	data, err := os.ReadFile(filepath.Join(base, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
