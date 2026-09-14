//go:build !linux

package network

import "errors"

func DeviceID(cfg Options) (string, error) {
	if cfg.DeviceID != "" {
		return cfg.DeviceID, nil
	}
	return "", errors.New("auto-detecting device_id is only supported on Linux")
}
