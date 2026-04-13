//go:build !linux

package trmnl

import "time"

func determineRuntimeMode(paths Paths, cfg Config, state State, now time.Time) (RuntimeMode, error) {
	if state.ConsecutiveFailures >= cfg.failureThreshold() {
		return RuntimeMode{Name: "recovery", MaintenanceReason: "failure-threshold", ShouldSuspend: false}, nil
	}
	return RuntimeMode{Name: "appliance", ShouldSuspend: true}, nil
}
