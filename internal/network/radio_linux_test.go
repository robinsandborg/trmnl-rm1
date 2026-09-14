//go:build linux

package network

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestRadioRebindWaitAndUnbind(t *testing.T) {
	root := t.TempDir()
	netRoot := filepath.Join(root, "net")
	driver := filepath.Join(root, "driver")
	devices := filepath.Join(root, "devices")
	for _, p := range []string{netRoot, driver, filepath.Join(devices, "mmc2:0001:1"), filepath.Join(driver, "mmc2:0001:1"), filepath.Join(devices, "unrelated")} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var writes []string
	now := time.Unix(0, 0)
	polls := 0
	r := radio{netRoot, driver, devices, os.Stat, os.ReadDir, func(p string, b []byte, _ os.FileMode) error {
		writes = append(writes, filepath.Base(p)+":"+string(b))
		return nil
	}, func() time.Time { return now }, func(d time.Duration) {
		polls++
		now = now.Add(d)
		if polls == 2 {
			if err := os.Mkdir(filepath.Join(netRoot, "wlan0"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}}
	r.ensure("wlan0")
	r.ensure("wlan0")
	r.unbind()
	if !reflect.DeepEqual(writes, []string{"bind:mmc2:0001:1", "unbind:mmc2:0001:1"}) || polls != 2 {
		t.Fatalf("writes=%v polls=%d", writes, polls)
	}
	// Unsupported hosts do not wait or write.
	r.driverDir = filepath.Join(root, "missing")
	r.ensure("absent")
	if polls != 2 || len(writes) != 2 {
		t.Fatal("unsupported host effects")
	}
}

func TestRadioTimeoutIsBounded(t *testing.T) {
	root := t.TempDir()
	now := time.Unix(0, 0)
	polls := 0
	r := radio{root, root, root, os.Stat, os.ReadDir, func(string, []byte, os.FileMode) error { return errors.New("denied") }, func() time.Time { return now }, func(d time.Duration) { polls++; now = now.Add(d) }}
	r.ensure("absent")
	if polls != 50 {
		t.Fatalf("polls %d", polls)
	}
}

func TestRadioLifecycleOrdering(t *testing.T) {
	var trace []string
	ops := LinkOps{Run: func(argv []string) error { trace = append(trace, argv[0]+":"+argv[len(argv)-1]); return nil }, Ensure: func(string) { trace = append(trace, "ensure") }, Start: func() error { trace = append(trace, "unblock"); return nil }, AfterUp: func() { trace = append(trace, "supplicant-start") }, Stop: func() { trace = append(trace, "radio-stop") }}
	opts := Options{Interface: "wlan0"}
	if err := BringUpWithOps(opts, ops); err != nil {
		t.Fatal(err)
	}
	if err := BringDownWithOps(opts, ops); err != nil {
		t.Fatal(err)
	}
	want := []string{"ensure", "unblock", "ip:up", "supplicant-start", "ip:down", "radio-stop"}
	if !reflect.DeepEqual(trace, want) {
		t.Fatalf("%v", trace)
	}
	trace = nil
	opts.WiFiUpCommand = []string{"custom", "up"}
	opts.WiFiDownCommand = []string{"custom", "down"}
	_ = BringUpWithOps(opts, ops)
	_ = BringDownWithOps(opts, ops)
	if !reflect.DeepEqual(trace, []string{"custom:up", "custom:down"}) {
		t.Fatalf("override %v", trace)
	}
}
