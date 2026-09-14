//go:build linux

package trmnl

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestPlanNextCycleSchedulingContract(t *testing.T) {
	wakeErr, timerErr := errors.New("wake failed"), errors.New("timer failed")
	for _, tc := range []struct {
		name                         string
		suspend, failWake, failTimer bool
		wantMode                     RuntimeMode
		wantCalls                    []string
		wantError                    string
	}{
		{"awake", false, false, false, RuntimeMode{Name: "maintenance"}, []string{"timer"}, ""},
		{"awake timer fails", false, false, true, RuntimeMode{Name: "maintenance"}, []string{"timer"}, "timer failed"},
		{"RTC", true, false, false, RuntimeMode{Name: "appliance", ShouldSuspend: true}, []string{"wake"}, ""},
		{"RTC fallback", true, true, false, RuntimeMode{Name: "awake-fallback", MaintenanceReason: "rtc-fallback"}, []string{"wake", "timer"}, ""},
		{"both fail", true, true, true, RuntimeMode{Name: "appliance", ShouldSuspend: true}, []string{"wake", "timer"}, "schedule wake alarm: wake failed; transient timer fallback failed: timer failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode := RuntimeMode{Name: "maintenance"}
			if tc.suspend {
				mode = RuntimeMode{Name: "appliance", ShouldSuspend: true}
			}
			var calls []string
			got, err := planNextCycleWithDeps(Config{}, 15*time.Minute, mode, scheduleDeps{
				wakeAlarm: func(_ Config, interval time.Duration) error {
					if interval != 15*time.Minute {
						t.Fatal(interval)
					}
					calls = append(calls, "wake")
					if tc.failWake {
						return wakeErr
					}
					return nil
				},
				transientRun: func(interval time.Duration) error {
					if interval != 15*time.Minute {
						t.Fatal(interval)
					}
					calls = append(calls, "timer")
					if tc.failTimer {
						return timerErr
					}
					return nil
				},
			})
			message := ""
			if err != nil {
				message = err.Error()
			}
			if got != tc.wantMode || !reflect.DeepEqual(calls, tc.wantCalls) || message != tc.wantError {
				t.Fatalf("mode=%+v calls=%v error=%v", got, calls, err)
			}
			if tc.failWake && tc.failTimer && (!errors.Is(err, wakeErr) || errors.Is(err, timerErr)) {
				t.Fatal("combined error wrapping changed")
			}
		})
	}
}

func TestScheduleTransientRunWithRunnerAlternatesUnits(t *testing.T) {
	tests := []struct {
		name     string
		selfUnit string
		interval time.Duration
		want     [][]string
	}{
		{
			name:     "from appliance, targets unit-a",
			selfUnit: "trmnl-rm1-appliance",
			interval: 90 * time.Second,
			want: [][]string{
				{"systemctl", "stop", "trmnl-rm1-next-a.timer", "trmnl-rm1-next-a.service"},
				{"systemctl", "reset-failed", "trmnl-rm1-next-a.timer", "trmnl-rm1-next-a.service"},
				{
					"systemd-run",
					"--unit=trmnl-rm1-next-a",
					"--on-active=90",
					"--property=Type=oneshot",
					"--property=Environment=HOME=/home/root",
					"/usr/bin/trmnl-rm1",
					"run-once",
				},
			},
		},
		{
			name:     "from unit-a, targets unit-b",
			selfUnit: "trmnl-rm1-next-a",
			interval: 900 * time.Second,
			want: [][]string{
				{"systemctl", "stop", "trmnl-rm1-next-b.timer", "trmnl-rm1-next-b.service"},
				{"systemctl", "reset-failed", "trmnl-rm1-next-b.timer", "trmnl-rm1-next-b.service"},
				{
					"systemd-run",
					"--unit=trmnl-rm1-next-b",
					"--on-active=900",
					"--property=Type=oneshot",
					"--property=Environment=HOME=/home/root",
					"/usr/bin/trmnl-rm1",
					"run-once",
				},
			},
		},
		{
			name:     "from unit-b, targets unit-a",
			selfUnit: "trmnl-rm1-next-b",
			interval: 300 * time.Second,
			want: [][]string{
				{"systemctl", "stop", "trmnl-rm1-next-a.timer", "trmnl-rm1-next-a.service"},
				{"systemctl", "reset-failed", "trmnl-rm1-next-a.timer", "trmnl-rm1-next-a.service"},
				{
					"systemd-run",
					"--unit=trmnl-rm1-next-a",
					"--on-active=300",
					"--property=Type=oneshot",
					"--property=Environment=HOME=/home/root",
					"/usr/bin/trmnl-rm1",
					"run-once",
				},
			},
		},
		{
			name:     "zero interval falls back to default",
			selfUnit: "",
			interval: 0,
			want: [][]string{
				{"systemctl", "stop", "trmnl-rm1-next-a.timer", "trmnl-rm1-next-a.service"},
				{"systemctl", "reset-failed", "trmnl-rm1-next-a.timer", "trmnl-rm1-next-a.service"},
				{
					"systemd-run",
					"--unit=trmnl-rm1-next-a",
					"--on-active=1800",
					"--property=Type=oneshot",
					"--property=Environment=HOME=/home/root",
					"/usr/bin/trmnl-rm1",
					"run-once",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got [][]string
			err := scheduleTransientRunWithRunner(func(parts []string) error {
				got = append(got, append([]string(nil), parts...))
				return nil
			}, "/usr/bin/trmnl-rm1", tc.selfUnit, tc.interval)
			if err != nil {
				t.Fatalf("scheduleTransientRunWithRunner error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("commands = %#v, want %#v", got, tc.want)
			}
		})
	}
}
