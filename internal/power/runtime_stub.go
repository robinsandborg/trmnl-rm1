//go:build !linux

package power

import "time"

func DetermineMode(sentinel string, cfg Options, failures int, now time.Time) (Mode, error) {
	if failures >= cfg.FailureThreshold {
		return Mode{Name: "recovery", MaintenanceReason: "failure-threshold", ShouldSuspend: false}, nil
	}
	return Mode{Name: "appliance", ShouldSuspend: true}, nil
}
