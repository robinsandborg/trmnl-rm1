//go:build linux

package trmnl

import (
	"reflect"
	"testing"
	"time"
)

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
