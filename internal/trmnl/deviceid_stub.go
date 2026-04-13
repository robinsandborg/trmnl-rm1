//go:build !linux

package trmnl

import "errors"

func resolveDeviceID(cfg Config) (string, error) {
	if cfg.DeviceID != "" {
		return cfg.DeviceID, nil
	}
	return "", errors.New("auto-detecting device_id is only supported on Linux")
}
