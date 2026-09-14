package trmnl

import (
	"context"
	"github.com/robinsandborg/rm1-trmnl/internal/network"
)

func networkOptions(cfg Config) network.Options {
	return network.Options{Interface: cfg.wifiInterface(), DeviceID: cfg.DeviceID, BaseURL: cfg.BaseURL,
		ConnectivityCheckURL: cfg.ConnectivityCheckURL, WiFiUpCommand: cfg.WiFiUpCommand, WiFiDownCommand: cfg.WiFiDownCommand,
		DisableWiFiBetweenUpdates: cfg.DisableWiFiBetweenUpdates, Timeout: cfg.wifiTimeout()}
}
func bringWiFiUp(cfg Config) error   { return network.BringUp(networkOptions(cfg), runCommand) }
func bringWiFiDown(cfg Config) error { return network.BringDown(networkOptions(cfg), runCommand) }
func waitForConnectivity(ctx context.Context, cfg Config) error {
	return network.Wait(ctx, networkOptions(cfg))
}
func resolveDeviceID(cfg Config) (string, error) { return network.DeviceID(networkOptions(cfg)) }
