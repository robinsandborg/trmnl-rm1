//go:build linux

package power_test

import (
	"errors"
	"github.com/robinsandborg/rm1-trmnl/internal/power"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestBatteryMissingOptionalAndFirstCandidate(t *testing.T) {
	root := t.TempDir()
	if got, err := power.ReadBattery(power.Options{PowerSupplyPath: root}); got != nil || err != nil {
		t.Fatalf("empty %v %v", got, err)
	}
	for _, name := range []string{"a", "z"} {
		p := filepath.Join(root, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "capacity"), []byte(" 42\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "a", "status"), []byte("Discharging\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := power.ReadBattery(power.Options{PowerSupplyPath: root})
	if err != nil || got == nil || got.Status != "Discharging" || got.CapacityPct != "42" || got.VoltageMicroV != "" {
		t.Fatalf("%+v %v", got, err)
	}
	if got, err := power.ReadBattery(power.Options{PowerSupplyPath: filepath.Join(root, "missing")}); got != nil || err == nil {
		t.Fatal("missing path")
	}
}

func TestWakeAlarmAndFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wakealarm")
	var calls [][]string
	failure := errors.New("rtcwake failed")
	opts := power.Options{RTCWakealarmPath: path, Run: func(argv []string) error { calls = append(calls, argv); return failure }}
	if err := power.WakeAlarm(opts, 90*time.Second); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "+90" || len(calls) != 0 {
		t.Fatalf("%q %v", got, calls)
	}
	if err := power.WakeAlarm(opts, 0); err == nil || err.Error() != "invalid wake interval: 0s" {
		t.Fatal(err)
	}
	opts.RTCWakealarmPath = filepath.Join(t.TempDir(), "missing", "wakealarm")
	if err := power.WakeAlarm(opts, 90*time.Second); !errors.Is(err, failure) || !reflect.DeepEqual(calls, [][]string{{"rtcwake", "-m", "no", "-s", "90"}}) {
		t.Fatalf("%v %v", calls, err)
	}
}

func TestSuspendFallbackAndOverride(t *testing.T) {
	var calls [][]string
	failure := errors.New("suspend failed")
	opts := power.Options{Run: func(argv []string) error { calls = append(calls, argv); return failure }}
	if err := power.Suspend(opts); !errors.Is(err, failure) || !reflect.DeepEqual(calls, [][]string{{"systemctl", "suspend"}, {"sh", "-c", "echo mem > /sys/power/state"}}) {
		t.Fatalf("%v %v", calls, err)
	}
	calls = nil
	opts.SuspendCommand = []string{"custom", "sleep"}
	if err := power.Suspend(opts); !errors.Is(err, failure) || !reflect.DeepEqual(calls, [][]string{{"custom", "sleep"}}) {
		t.Fatalf("%v %v", calls, err)
	}
}
