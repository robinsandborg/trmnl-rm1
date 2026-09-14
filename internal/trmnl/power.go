package trmnl

import (
	"github.com/robinsandborg/rm1-trmnl/internal/power"
	"time"
)

func powerOptions(cfg Config) power.Options {
	return power.Options{PowerSupplyPath: cfg.powerSupplyPath(), RTCWakealarmPath: cfg.rtcWakealarmPath(), MaintenanceInterface: cfg.maintenanceInterface(), BootGrace: cfg.bootGrace(), FailureThreshold: cfg.failureThreshold(), SuspendCommand: cfg.SuspendCommand, Run: runCommand}
}
func readBatterySample(cfg Config) (*BatterySample, error) {
	out, err := power.ReadBattery(powerOptions(cfg))
	if out == nil {
		return nil, err
	}
	v := BatterySample(*out)
	return &v, err
}
func planNextCycle(cfg Config, d time.Duration, m RuntimeMode) (RuntimeMode, error) {
	out, err := power.Plan(powerOptions(cfg), d, power.Mode(m))
	return RuntimeMode(out), err
}
func suspendDevice(cfg Config) error { return power.Suspend(powerOptions(cfg)) }
func determineRuntimeMode(paths Paths, cfg Config, state State, now time.Time) (RuntimeMode, error) {
	out, err := power.DetermineMode(paths.MaintenanceSentinel, powerOptions(cfg), state.ConsecutiveFailures, now)
	return RuntimeMode(out), err
}
