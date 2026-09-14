//go:build linux

package trmnl

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Connect the cycle to the real Linux mode and scheduling policy, replacing
// only their hardware observations and wake/timer effects.
func TestCycleLinuxModeAndScheduling(t *testing.T) {
	for _, tc := range []struct {
		name, mode, reason, scheduling string
		sentinel, usb, rtcFails        bool
		uptime                         time.Duration
		failures                       int
	}{
		{"sentinel wins", "maintenance", "sentinel-file", "timer", true, true, false, 0, 3},
		{"USB wins", "maintenance", "usb-network", "timer", false, true, false, 0, 3},
		{"boot grace wins", "boot-grace", "boot-grace", "timer", false, false, false, time.Minute, 3},
		{"recovery", "recovery", "failure-threshold", "timer", false, false, false, time.Hour, 3},
		{"appliance", "appliance", "", "wake suspend", false, false, false, time.Hour, 0},
		{"RTC fallback", "awake-fallback", "rtc-fallback", "wake timer", false, false, true, time.Hour, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newCycleHarness(t)
			h.before.ConsecutiveFailures = tc.failures
			h.seed()
			if tc.sentinel {
				writeTestFile(t, h.paths.MaintenanceSentinel, nil)
			}
			h.app.cycle.determineRuntimeMode = func(paths Paths, cfg Config, state State, _ time.Time) (RuntimeMode, error) {
				return determineRuntimeModeWithDeps(paths, cfg, state, runtimeModeDeps{
					stat:             os.Stat,
					usbNetworkActive: func(Config) (bool, error) { return tc.usb, nil },
					readUptime:       func() (time.Duration, error) { return tc.uptime, nil },
				})
			}
			var scheduling []string
			h.app.cycle.planNextCycle = func(cfg Config, interval time.Duration, mode RuntimeMode) (RuntimeMode, error) {
				if interval != 15*time.Minute {
					t.Fatal(interval)
				}
				return planNextCycleWithDeps(cfg, interval, mode, scheduleDeps{
					wakeAlarm: func(Config, time.Duration) error {
						scheduling = append(scheduling, "wake")
						if tc.rtcFails {
							return errors.New("RTC unavailable")
						}
						return nil
					},
					transientRun: func(time.Duration) error { scheduling = append(scheduling, "timer"); return nil },
				})
			}
			h.app.cycle.suspendDevice = func(Config) error {
				if h.state().LastSuccessAt != cycleTime || len(h.logs()) != 1 {
					t.Fatal("suspend before persistence")
				}
				if len(h.events) == 0 || h.events[len(h.events)-1] != "wifi-down" {
					t.Fatal("suspend before network cleanup")
				}
				scheduling = append(scheduling, "suspend")
				return nil
			}
			if err := h.run(); err != nil {
				t.Fatal(err)
			}
			entry := h.logs()[0]
			if entry.Mode != tc.mode || entry.MaintenanceReason != tc.reason || strings.Join(scheduling, " ") != tc.scheduling {
				t.Fatalf("entry=%+v scheduling=%v", entry, scheduling)
			}
		})
	}
}
