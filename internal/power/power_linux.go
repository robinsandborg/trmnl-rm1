//go:build linux

package power

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func ReadBattery(cfg Options) (*BatterySample, error) {
	entries, err := os.ReadDir(cfg.PowerSupplyPath)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		base := filepath.Join(cfg.PowerSupplyPath, entry.Name())
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

type ScheduleDeps struct {
	WakeAlarm    func(Options, time.Duration) error
	TransientRun func(time.Duration) error
}

func Plan(cfg Options, interval time.Duration, mode Mode) (Mode, error) {
	return PlanWithDeps(cfg, interval, mode, ScheduleDeps{
		WakeAlarm:    WakeAlarm,
		TransientRun: func(d time.Duration) error { return Transient(cfg.Run, d) },
	})
}

func PlanWithDeps(cfg Options, interval time.Duration, mode Mode, deps ScheduleDeps) (Mode, error) {
	if !mode.ShouldSuspend {
		if err := deps.TransientRun(interval); err != nil {
			return mode, err
		}
		return mode, nil
	}

	if err := deps.WakeAlarm(cfg, interval); err != nil {
		fallback := Mode{Name: "awake-fallback", MaintenanceReason: "rtc-fallback", ShouldSuspend: false}
		if timerErr := deps.TransientRun(interval); timerErr != nil {
			return mode, fmt.Errorf("schedule wake alarm: %w; transient timer fallback failed: %v", err, timerErr)
		}
		return fallback, nil
	}

	return mode, nil
}

func Suspend(cfg Options) error {
	if len(cfg.SuspendCommand) > 0 {
		return cfg.Run(cfg.SuspendCommand)
	}
	return firstSuccessful(cfg.Run,
		[]string{"systemctl", "suspend"},
		[]string{"sh", "-c", "echo mem > /sys/power/state"},
	)
}

func WakeAlarm(cfg Options, interval time.Duration) error {
	wakePath := cfg.RTCWakealarmPath
	seconds := int(interval.Seconds())
	if seconds <= 0 {
		return fmt.Errorf("invalid wake interval: %s", interval)
	}

	if err := os.WriteFile(wakePath, []byte("0"), 0o644); err == nil {
		if err := os.WriteFile(wakePath, []byte(fmt.Sprintf("+%d", seconds)), 0o644); err == nil {
			return nil
		}
	}

	return firstSuccessful(cfg.Run,
		[]string{"rtcwake", "-m", "no", "-s", strconv.Itoa(seconds)},
	)
}

func Transient(run func([]string) error, interval time.Duration) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	return TransientWithRunner(run, exePath, ReadSelfUnit(), interval)
}

const (
	transientNextUnitA = "trmnl-rm1-next-a"
	transientNextUnitB = "trmnl-rm1-next-b"
)

// TransientWithRunner arms the next awake-mode cycle. RM1's
// systemd predates `systemd-run --replace`, and a cycle running *inside*
// `trmnl-rm1-next-X.service` can't stop or recreate its own unit without
// SIGTERM'ing itself. So we alternate between two unit names and always
// target the one we are NOT running inside.
func TransientWithRunner(run func([]string) error, exePath, selfUnit string, interval time.Duration) error {
	seconds := int(interval.Seconds())
	if seconds <= 0 {
		seconds = int((30 * time.Minute).Seconds())
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

// ReadSelfUnit returns the systemd unit name this process is running inside,
// derived from /proc/self/cgroup. Empty string if unavailable or not under
// a systemd-managed cgroup.
func ReadSelfUnit() string {
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
