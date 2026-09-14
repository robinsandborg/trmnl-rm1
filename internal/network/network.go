// Package network owns wireless acquisition, connectivity and device identity.
package network

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type Options struct {
	Interface, DeviceID, BaseURL, ConnectivityCheckURL string
	WiFiUpCommand, WiFiDownCommand                     []string
	DisableWiFiBetweenUpdates                          bool
	Timeout                                            time.Duration
}

type Operations struct {
	BringUp   func() error
	BringDown func() error
	Wait      func(context.Context) error
}

func Prepare(opts Options, ops Operations) (*http.Client, func(), error) {
	cleanup := func() {}
	if opts.DisableWiFiBetweenUpdates {
		if err := ops.BringUp(); err != nil {
			return nil, cleanup, err
		}
		cleanup = func() { _ = ops.BringDown() }
	}
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()
	if err := ops.Wait(ctx); err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return &http.Client{Timeout: opts.Timeout}, cleanup, nil
}

func firstSuccessful(run func([]string) error, commands ...[]string) error {
	var errs []error
	for _, cmd := range commands {
		if len(cmd) == 0 {
			continue
		}
		if err := run(cmd); err == nil {
			return nil
		} else {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return errors.New("no commands available")
	}
	return errors.Join(errs...)
}
