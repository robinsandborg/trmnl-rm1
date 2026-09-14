//go:build linux

package trmnl

import (
	"github.com/robinsandborg/rm1-trmnl/internal/power"
	"os"
	"time"
)

type runtimeModeDeps struct {
	stat             func(string) (os.FileInfo, error)
	usbNetworkActive func(Config) (bool, error)
	readUptime       func() (time.Duration, error)
}

func determineRuntimeModeWithDeps(paths Paths, cfg Config, state State, deps runtimeModeDeps) (RuntimeMode, error) {
	ops := power.ModeDeps{Stat: deps.stat, ReadUptime: deps.readUptime}
	if deps.usbNetworkActive != nil {
		ops.USBActive = func(_ power.Options) (bool, error) { return deps.usbNetworkActive(cfg) }
	}
	out, err := power.DetermineModeWithDeps(paths.MaintenanceSentinel, powerOptions(cfg), state.ConsecutiveFailures, ops)
	return RuntimeMode(out), err
}
func usbNetworkActiveAt(cfg Config, root string) (bool, error) {
	return power.USBActiveAt(powerOptions(cfg), root)
}
