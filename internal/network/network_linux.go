//go:build linux

package network

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func BringUp(cfg Options, run func([]string) error) error {
	return BringUpWithOps(cfg, defaultLinkOps(run))
}
func BringUpWithOps(cfg Options, ops LinkOps) error {
	run := ops.Run
	if len(cfg.WiFiUpCommand) > 0 {
		return run(cfg.WiFiUpCommand)
	}
	if ops.Ensure != nil {
		ops.Ensure(cfg.Interface)
	}
	if ops.Start != nil {
		if err := ops.Start(); err != nil {
			return err
		}
	}
	iface := cfg.Interface
	if err := firstSuccessful(run,
		[]string{"ip", "link", "set", iface, "up"},
		[]string{"ifconfig", iface, "up"},
		[]string{"ifup", iface},
	); err != nil {
		return err
	}
	if ops.AfterUp != nil {
		ops.AfterUp()
	}
	return nil
}

func BringDown(cfg Options, run func([]string) error) error {
	return BringDownWithOps(cfg, defaultLinkOps(run))
}
func BringDownWithOps(cfg Options, ops LinkOps) error {
	run := ops.Run
	if len(cfg.WiFiDownCommand) > 0 {
		return run(cfg.WiFiDownCommand)
	}
	iface := cfg.Interface
	linkErr := firstSuccessful(run,
		[]string{"ip", "link", "set", iface, "down"},
		[]string{"ifconfig", iface, "down"},
		[]string{"ifdown", iface},
	)
	if ops.Stop != nil {
		ops.Stop()
	}
	return linkErr
}

func Wait(ctx context.Context, cfg Options) error {
	checkURL := cfg.ConnectivityCheckURL
	if checkURL == "" {
		checkURL = cfg.BaseURL
	}

	client := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, checkURL, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			msg := strings.TrimSpace(ctx.Err().Error())
			if msg == "" {
				msg = "timed out waiting for Wi-Fi connectivity"
			}
			return fmt.Errorf("timed out waiting for Wi-Fi connectivity to %s: %s", checkURL, msg)
		case <-ticker.C:
		}
	}
}
