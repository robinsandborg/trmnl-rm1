//go:build !linux

package trmnl

import (
	"context"
	"errors"
)

func bringWiFiUp(cfg Config) error {
	return errors.New("Wi-Fi control is only supported on Linux")
}

func bringWiFiDown(cfg Config) error {
	return errors.New("Wi-Fi control is only supported on Linux")
}

func waitForConnectivity(ctx context.Context, cfg Config) error {
	return errors.New("Wi-Fi connectivity checks are only supported on Linux")
}
