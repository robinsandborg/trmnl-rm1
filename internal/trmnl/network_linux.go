//go:build linux

package trmnl

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func bringWiFiUp(cfg Config) error {
	if len(cfg.WiFiUpCommand) > 0 {
		return runCommand(cfg.WiFiUpCommand)
	}
	iface := cfg.wifiInterface()
	return firstSuccessful(
		[]string{"ip", "link", "set", iface, "up"},
		[]string{"ifconfig", iface, "up"},
		[]string{"ifup", iface},
	)
}

func bringWiFiDown(cfg Config) error {
	if len(cfg.WiFiDownCommand) > 0 {
		return runCommand(cfg.WiFiDownCommand)
	}
	iface := cfg.wifiInterface()
	return firstSuccessful(
		[]string{"ip", "link", "set", iface, "down"},
		[]string{"ifconfig", iface, "down"},
		[]string{"ifdown", iface},
	)
}

func waitForConnectivity(ctx context.Context, cfg Config) error {
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
