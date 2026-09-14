//go:build linux

package trmnl

import (
	"github.com/robinsandborg/rm1-trmnl/internal/power"
	"time"
)

type scheduleDeps struct {
	wakeAlarm    func(Config, time.Duration) error
	transientRun func(time.Duration) error
}

func planNextCycleWithDeps(cfg Config, d time.Duration, m RuntimeMode, deps scheduleDeps) (RuntimeMode, error) {
	out, err := power.PlanWithDeps(powerOptions(cfg), d, power.Mode(m), power.ScheduleDeps{WakeAlarm: func(_ power.Options, d time.Duration) error { return deps.wakeAlarm(cfg, d) }, TransientRun: deps.transientRun})
	return RuntimeMode(out), err
}
func scheduleTransientRunWithRunner(run func([]string) error, exe, self string, d time.Duration) error {
	return power.TransientWithRunner(run, exe, self, d)
}
