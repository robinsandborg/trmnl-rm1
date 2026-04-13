//go:build linux

package trmnl

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetermineRuntimeModeWithDepsSelectsExpectedMode(t *testing.T) {
	tests := []struct {
		name           string
		sentinelExists bool
		usbActive      bool
		uptime         time.Duration
		state          State
		cfg            Config
		want           RuntimeMode
	}{
		{
			name:           "sentinel maintenance",
			sentinelExists: true,
			usbActive:      true,
			uptime:         0,
			state:          State{ConsecutiveFailures: 99},
			want:           RuntimeMode{Name: "maintenance", MaintenanceReason: "sentinel-file", ShouldSuspend: false},
		},
		{
			name:      "usb maintenance",
			usbActive: true,
			uptime:    12 * time.Minute,
			want:      RuntimeMode{Name: "maintenance", MaintenanceReason: "usb-network", ShouldSuspend: false},
		},
		{
			name:   "boot grace",
			uptime: 5 * time.Minute,
			state:  State{ConsecutiveFailures: 5},
			want:   RuntimeMode{Name: "boot-grace", MaintenanceReason: "boot-grace", ShouldSuspend: false},
		},
		{
			name:   "failure threshold recovery",
			uptime: 12 * time.Minute,
			state:  State{ConsecutiveFailures: 3},
			want:   RuntimeMode{Name: "recovery", MaintenanceReason: "failure-threshold", ShouldSuspend: false},
		},
		{
			name:   "normal appliance",
			uptime: 12 * time.Minute,
			state:  State{ConsecutiveFailures: 1},
			want:   RuntimeMode{Name: "appliance", ShouldSuspend: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := determineRuntimeModeWithDeps(Paths{
				MaintenanceSentinel: "/tmp/maintenance",
			}, Config{
				BootGraceSeconds: 600,
				FailureThreshold: 3,
			}, tc.state, runtimeModeDeps{
				stat: func(string) (os.FileInfo, error) {
					if tc.sentinelExists {
						return nil, nil
					}
					return nil, os.ErrNotExist
				},
				usbNetworkActive: func(Config) (bool, error) {
					return tc.usbActive, nil
				},
				readUptime: func() (time.Duration, error) {
					return tc.uptime, nil
				},
			})
			if err != nil {
				t.Fatalf("determineRuntimeModeWithDeps error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("mode = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestDetermineRuntimeModeWithDepsPropagatesDependencyErrors(t *testing.T) {
	usbErr := errors.New("usb check failed")
	got, err := determineRuntimeModeWithDeps(Paths{
		MaintenanceSentinel: "/tmp/maintenance",
	}, Config{}, State{}, runtimeModeDeps{
		stat: func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
		usbNetworkActive: func(Config) (bool, error) {
			return false, usbErr
		},
		readUptime: func() (time.Duration, error) {
			return 0, nil
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, usbErr) {
		t.Fatalf("expected usb error, got %v", err)
	}
	if got != (RuntimeMode{}) {
		t.Fatalf("mode = %#v, want zero RuntimeMode", got)
	}
}

func TestUSBNetworkActiveAtIgnoresStaticMACAddress(t *testing.T) {
	root := t.TempDir()
	ifaceDir := filepath.Join(root, "usb0")
	if err := os.MkdirAll(ifaceDir, 0o755); err != nil {
		t.Fatalf("mkdir iface dir: %v", err)
	}

	writeSysfsFile(t, ifaceDir, "operstate", "down\n")
	writeSysfsFile(t, ifaceDir, "carrier", "0\n")
	writeSysfsFile(t, ifaceDir, "address", "02:00:00:00:00:01\n")

	active, err := usbNetworkActiveAt(Config{}, root)
	if err != nil {
		t.Fatalf("usbNetworkActiveAt error = %v", err)
	}
	if active {
		t.Fatal("expected inactive USB network when only the interface MAC address is present")
	}
}

func TestUSBNetworkActiveAtReportsActiveLinkState(t *testing.T) {
	tests := []struct {
		name      string
		operstate string
		carrier   string
	}{
		{name: "operstate-up", operstate: "up\n", carrier: "0\n"},
		{name: "carrier-present", operstate: "down\n", carrier: "1\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			ifaceDir := filepath.Join(root, "usb0")
			if err := os.MkdirAll(ifaceDir, 0o755); err != nil {
				t.Fatalf("mkdir iface dir: %v", err)
			}

			writeSysfsFile(t, ifaceDir, "operstate", tc.operstate)
			writeSysfsFile(t, ifaceDir, "carrier", tc.carrier)

			active, err := usbNetworkActiveAt(Config{}, root)
			if err != nil {
				t.Fatalf("usbNetworkActiveAt error = %v", err)
			}
			if !active {
				t.Fatal("expected active USB network when link state is up")
			}
		})
	}
}

func writeSysfsFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
