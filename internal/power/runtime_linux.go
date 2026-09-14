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

type ModeDeps struct {
	Stat       func(string) (os.FileInfo, error)
	USBActive  func(Options) (bool, error)
	ReadUptime func() (time.Duration, error)
}

func DetermineMode(sentinel string, cfg Options, failures int, now time.Time) (Mode, error) {
	return DetermineModeWithDeps(sentinel, cfg, failures, ModeDeps{
		Stat:       os.Stat,
		USBActive:  USBActive,
		ReadUptime: ReadUptime,
	})
}

func DetermineModeWithDeps(sentinel string, cfg Options, failures int, deps ModeDeps) (Mode, error) {
	if deps.Stat == nil {
		deps.Stat = os.Stat
	}
	if deps.USBActive == nil {
		deps.USBActive = USBActive
	}
	if deps.ReadUptime == nil {
		deps.ReadUptime = ReadUptime
	}

	if _, err := deps.Stat(sentinel); err == nil {
		return Mode{Name: "maintenance", MaintenanceReason: "sentinel-file", ShouldSuspend: false}, nil
	}

	active, err := deps.USBActive(cfg)
	if err != nil {
		return Mode{}, err
	}
	if active {
		return Mode{Name: "maintenance", MaintenanceReason: "usb-network", ShouldSuspend: false}, nil
	}

	uptime, err := deps.ReadUptime()
	if err != nil {
		return Mode{}, err
	}
	if uptime < cfg.BootGrace {
		return Mode{Name: "boot-grace", MaintenanceReason: "boot-grace", ShouldSuspend: false}, nil
	}

	if failures >= cfg.FailureThreshold {
		return Mode{Name: "recovery", MaintenanceReason: "failure-threshold", ShouldSuspend: false}, nil
	}

	return Mode{Name: "appliance", ShouldSuspend: true}, nil
}

func USBActive(cfg Options) (bool, error) {
	return USBActiveAt(cfg, "/sys/class/net")
}

func USBActiveAt(cfg Options, netRoot string) (bool, error) {
	iface := cfg.MaintenanceInterface
	base := filepath.Join(netRoot, iface)
	if _, err := os.Stat(base); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	operstate, _ := os.ReadFile(filepath.Join(base, "operstate"))
	carrier, _ := os.ReadFile(filepath.Join(base, "carrier"))
	return strings.TrimSpace(string(operstate)) == "up" ||
		strings.TrimSpace(string(carrier)) == "1", nil
}

func ReadUptime() (time.Duration, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("unexpected /proc/uptime contents")
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds * float64(time.Second)), nil
}
