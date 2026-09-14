//go:build !linux

package network_test

import (
	"context"
	"github.com/robinsandborg/rm1-trmnl/internal/network"
	"testing"
)

func TestUnsupportedAndIdentity(t *testing.T) {
	opts := network.Options{DeviceID: " explicit "}
	id, err := network.DeviceID(opts)
	if id != opts.DeviceID || err != nil {
		t.Fatalf("%q %v", id, err)
	}
	run := func([]string) error { t.Fatal("unexpected command"); return nil }
	if err := network.BringUp(opts, run); err == nil || err.Error() != "Wi-Fi control is only supported on Linux" {
		t.Fatal(err)
	}
	if err := network.BringDown(opts, run); err == nil {
		t.Fatal("expected unsupported")
	}
	if err := network.Wait(context.Background(), opts); err == nil {
		t.Fatal("expected unsupported")
	}
}
