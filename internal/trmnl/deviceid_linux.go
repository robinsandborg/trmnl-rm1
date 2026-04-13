//go:build linux

package trmnl

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func resolveDeviceID(cfg Config) (string, error) {
	if strings.TrimSpace(cfg.DeviceID) != "" {
		return strings.TrimSpace(cfg.DeviceID), nil
	}

	candidates := []string{cfg.wifiInterface()}
	matches, _ := filepath.Glob("/sys/class/net/*/wireless")
	for _, match := range matches {
		candidates = append(candidates, filepath.Base(filepath.Dir(match)))
	}

	seen := map[string]bool{}
	for _, iface := range candidates {
		if iface == "" || seen[iface] {
			continue
		}
		seen[iface] = true
		value, err := os.ReadFile(filepath.Join("/sys/class/net", iface, "address"))
		if err == nil {
			addr := strings.ToUpper(strings.TrimSpace(string(value)))
			if addr != "" {
				return addr, nil
			}
		}
	}

	return "", errors.New("unable to determine wireless MAC address")
}
