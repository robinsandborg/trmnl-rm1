//go:build linux

package trmnl

import (
	"os"
	"path/filepath"
	"testing"
)

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
