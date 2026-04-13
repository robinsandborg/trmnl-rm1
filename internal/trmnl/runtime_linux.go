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

func determineRuntimeMode(paths Paths, cfg Config, state State, now time.Time) (RuntimeMode, error) {
	if _, err := os.Stat(paths.MaintenanceSentinel); err == nil {
		return RuntimeMode{Name: "maintenance", MaintenanceReason: "sentinel-file", ShouldSuspend: false}, nil
	}

	active, err := usbNetworkActive(cfg)
	if err != nil {
		return RuntimeMode{}, err
	}
	if active {
		return RuntimeMode{Name: "maintenance", MaintenanceReason: "usb-network", ShouldSuspend: false}, nil
	}

	uptime, err := readUptime()
	if err != nil {
		return RuntimeMode{}, err
	}
	if uptime < cfg.bootGrace() {
		return RuntimeMode{Name: "boot-grace", MaintenanceReason: "boot-grace", ShouldSuspend: false}, nil
	}

	if state.ConsecutiveFailures >= cfg.failureThreshold() {
		return RuntimeMode{Name: "recovery", MaintenanceReason: "failure-threshold", ShouldSuspend: false}, nil
	}

	return RuntimeMode{Name: "appliance", ShouldSuspend: true}, nil
}

func usbNetworkActive(cfg Config) (bool, error) {
	return usbNetworkActiveAt(cfg, "/sys/class/net")
}

func usbNetworkActiveAt(cfg Config, netRoot string) (bool, error) {
	iface := cfg.maintenanceInterface()
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

func readUptime() (time.Duration, error) {
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
