//go:build !linux

package trmnl

import (
	"errors"
	"time"
)

func readBatterySample(cfg Config) (*BatterySample, error) {
	return nil, nil
}

func planNextCycle(cfg Config, interval time.Duration, mode RuntimeMode) (RuntimeMode, error) {
	return mode, errors.New("power scheduling is only supported on Linux")
}

func suspendDevice(cfg Config) error {
	return errors.New("suspend is only supported on Linux")
}
