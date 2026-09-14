package trmnl

import "github.com/robinsandborg/rm1-trmnl/internal/appliance"

const (
	applianceServiceName    = appliance.ServiceName
	applianceServicePath    = appliance.ServicePath
	applianceResumeHookName = appliance.ResumeHookName
)

type applianceOps struct {
	restoreNetwork     func() error
	run                func([]string) error
	remove             func(string) error
	detectSleepHookDir func() (string, error)
}

func snapshot(state State) appliance.Snapshot {
	return appliance.Snapshot{MaskedNoise: state.MaskedNoise, StockSyncUnit: state.StockSyncUnit, SyncWasEnabled: state.SyncWasEnabled, XochitlWasEnabled: state.XochitlWasEnabled}
}
func disableForApplianceWithRunner(run func([]string) error, unit string) error {
	return appliance.Disable(run, unit)
}
func runRestoreWithOps(state State, ops applianceOps) error {
	return appliance.Restore(snapshot(state), appliance.RestoreOps{RestoreNetwork: ops.restoreNetwork, Run: ops.run, Remove: ops.remove, SleepHookDir: ops.detectSleepHookDir})
}
