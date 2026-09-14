//go:build !linux

package power

import (
	"errors"
	"time"
)

func ReadBattery(cfg Options) (*BatterySample, error) {
	return nil, nil
}

func Plan(cfg Options, interval time.Duration, mode Mode) (Mode, error) {
	return mode, errors.New("power scheduling is only supported on Linux")
}

func Suspend(cfg Options) error {
	return errors.New("suspend is only supported on Linux")
}
