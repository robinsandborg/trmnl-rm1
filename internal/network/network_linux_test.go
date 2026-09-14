//go:build linux

package network_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/robinsandborg/rm1-trmnl/internal/network"
)

func TestLinkFallbacksAndOverrides(t *testing.T) {
	for _, down := range []bool{false, true} {
		var calls [][]string
		fail := errors.New("link failed")
		run := func(argv []string) error { calls = append(calls, argv); return fail }
		opts := network.Options{Interface: "wlan7"}
		action, verb, legacy := func(o network.Options, r func([]string) error) error {
			return network.BringUpWithOps(o, network.LinkOps{Run: r})
		}, "up", "ifup"
		if down {
			action, verb, legacy = func(o network.Options, r func([]string) error) error {
				return network.BringDownWithOps(o, network.LinkOps{Run: r})
			}, "down", "ifdown"
		}
		err := action(opts, run)
		want := [][]string{{"ip", "link", "set", "wlan7", verb}, {"ifconfig", "wlan7", verb}, {legacy, "wlan7"}}
		if !errors.Is(err, fail) || !reflect.DeepEqual(calls, want) {
			t.Fatalf("%v: %v", calls, err)
		}
		calls = nil
		if err := action(opts, func(argv []string) error { calls = append(calls, argv); return nil }); err != nil || len(calls) != 1 {
			t.Fatalf("did not stop after success: %v %v", calls, err)
		}
		opts.WiFiUpCommand = []string{"custom", "up"}
		opts.WiFiDownCommand = []string{"custom", "down"}
		calls = nil
		if err := action(opts, run); !errors.Is(err, fail) || !reflect.DeepEqual(calls, [][]string{{"custom", verb}}) {
			t.Fatalf("override: %v %v", calls, err)
		}
	}
}

func TestIdentityCandidateOrder(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"wlan0", "wlan1"} {
		base := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Join(base, "wireless"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "address"), []byte(" aa:"+name+" \n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		opts network.Options
		want string
	}{
		{network.Options{DeviceID: " explicit "}, "explicit"},
		{network.Options{Interface: "wlan1"}, "AA:wlan1"},
		{network.Options{Interface: "missing"}, "AA:wlan0"},
	} {
		got, err := network.DeviceIDAt(tc.opts, root)
		if err != nil || got != strings.ToUpper(tc.want) && got != tc.want {
			t.Fatalf("%q %v", got, err)
		}
	}
	if _, err := network.DeviceIDAt(network.Options{}, t.TempDir()); err == nil || err.Error() != "unable to determine wireless MAC address" {
		t.Fatal(err)
	}
}

func TestConnectivityStatusAndCancellation(t *testing.T) {
	for _, status := range []int{200, 404, 499, 500} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodHead {
				t.Errorf("method %s", r.Method)
			}
			w.WriteHeader(status)
		}))
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		err := network.Wait(ctx, network.Options{BaseURL: "http://invalid", ConnectivityCheckURL: server.URL})
		cancel()
		server.Close()
		if status < 500 && err != nil {
			t.Fatal(err)
		}
		if status == 500 && (err == nil || !strings.Contains(err.Error(), "timed out waiting for Wi-Fi connectivity to "+server.URL)) {
			t.Fatal(err)
		}
	}
	if err := network.Wait(context.Background(), network.Options{BaseURL: ":bad"}); err == nil {
		t.Fatal("malformed URL accepted")
	}
}
