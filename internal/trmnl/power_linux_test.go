//go:build linux

package trmnl

import (
	"reflect"
	"testing"
	"time"
)

func TestScheduleTransientRunWithRunnerUsesReplaceAndInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval time.Duration
		want     []string
	}{
		{
			name:     "explicit interval",
			interval: 90 * time.Second,
			want: []string{
				"systemd-run",
				"--replace",
				"--unit=trmnl-rm1-next",
				"--on-active=90",
				"--property=Type=oneshot",
				"/usr/bin/trmnl-rm1",
				"run-once",
			},
		},
		{
			name:     "default fallback interval",
			interval: 0,
			want: []string{
				"systemd-run",
				"--replace",
				"--unit=trmnl-rm1-next",
				"--on-active=1800",
				"--property=Type=oneshot",
				"/usr/bin/trmnl-rm1",
				"run-once",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			err := scheduleTransientRunWithRunner(func(parts []string) error {
				got = append([]string(nil), parts...)
				return nil
			}, "/usr/bin/trmnl-rm1", tc.interval)
			if err != nil {
				t.Fatalf("scheduleTransientRunWithRunner error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("command = %#v, want %#v", got, tc.want)
			}
		})
	}
}
