//go:build linux

package network

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func DeviceID(cfg Options) (string, error) { return DeviceIDAt(cfg, "/sys/class/net") }

func DeviceIDAt(cfg Options, netRoot string) (string, error) {
	if strings.TrimSpace(cfg.DeviceID) != "" {
		return strings.TrimSpace(cfg.DeviceID), nil
	}

	candidates := []string{cfg.Interface}
	matches, _ := filepath.Glob(filepath.Join(netRoot, "*", "wireless"))
	for _, match := range matches {
		candidates = append(candidates, filepath.Base(filepath.Dir(match)))
	}

	seen := map[string]bool{}
	for _, iface := range candidates {
		if iface == "" || seen[iface] {
			continue
		}
		seen[iface] = true
		value, err := os.ReadFile(filepath.Join(netRoot, iface, "address"))
		if err == nil {
			addr := strings.ToUpper(strings.TrimSpace(string(value)))
			if addr != "" {
				return addr, nil
			}
		}
	}

	return "", errors.New("unable to determine wireless MAC address")
}
