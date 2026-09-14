//go:build !linux

package network

import (
	"context"
	"errors"
)

func BringUp(cfg Options, run func([]string) error) error {
	return errors.New("Wi-Fi control is only supported on Linux")
}

func BringDown(cfg Options, run func([]string) error) error {
	return errors.New("Wi-Fi control is only supported on Linux")
}

func Wait(ctx context.Context, cfg Options) error {
	return errors.New("Wi-Fi connectivity checks are only supported on Linux")
}

func EnsureInterface(opts Options) {}
