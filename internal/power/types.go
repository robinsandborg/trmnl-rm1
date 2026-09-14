// Package power owns runtime mode, battery, wake scheduling and suspend.
package power

import (
	"errors"
	"time"
)

type Options struct {
	PowerSupplyPath, RTCWakealarmPath, MaintenanceInterface string
	BootGrace                                               time.Duration
	FailureThreshold                                        int
	SuspendCommand                                          []string
	Run                                                     func([]string) error
}
type BatterySample struct {
	Status         string `json:"status,omitempty"`
	CapacityPct    string `json:"capacity_pct,omitempty"`
	VoltageMicroV  string `json:"voltage_micro_v,omitempty"`
	CurrentMicroA  string `json:"current_micro_a,omitempty"`
	TemperatureDec string `json:"temperature_decic,omitempty"`
}

type Mode struct {
	Name              string
	MaintenanceReason string
	ShouldSuspend     bool
}

func firstSuccessful(run func([]string) error, commands ...[]string) error {
	var errs []error
	for _, cmd := range commands {
		if len(cmd) == 0 {
			continue
		}
		if err := run(cmd); err == nil {
			return nil
		} else {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return errors.New("no commands available")
	}
	return errors.Join(errs...)
}
