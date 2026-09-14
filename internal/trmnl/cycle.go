package trmnl

import (
	"net/http"
	"time"

	"github.com/robinsandborg/rm1-trmnl/internal/cycle"
	"github.com/robinsandborg/rm1-trmnl/internal/display"
	"github.com/robinsandborg/rm1-trmnl/internal/power"
)

func (a *App) cycleOperations(paths Paths, cfg Config) cycle.Operations {
	return cycle.Operations{
		Now: a.now,
		ReadBattery: func() (*power.BatterySample, error) {
			v, err := a.cycle.readBatterySample(cfg)
			return (*power.BatterySample)(v), err
		},
		DetermineMode: func(s cycle.State, t time.Time) (power.Mode, error) {
			m, err := a.cycle.determineRuntimeMode(paths, cfg, State(s), t)
			return power.Mode(m), err
		},
		PrepareNetwork: func() (*http.Client, func(), error) { return a.cycle.prepareNetwork(cfg) },
		FetchPayload: func(c *http.Client) (string, []byte, string, time.Duration, error) {
			v, b, u, d, err := fetchCyclePayload(c, cfg)
			return v.Filename, b, u, d, err
		},
		Render: func(b []byte, path string, m display.RefreshMode) error {
			return a.cycle.renderImage(cfg, b, path, RefreshMode(m))
		},
		Plan: func(d time.Duration, m power.Mode) (power.Mode, error) {
			out, err := a.cycle.planNextCycle(cfg, d, RuntimeMode(m))
			return power.Mode(out), err
		},
		Suspend:   func() error { return a.cycle.suspendDevice(cfg) },
		WriteFile: a.cycle.writeFile,
		AppendLog: func(entry cycle.CycleLog) error {
			return a.cycle.appendCycleLog(paths, CycleLog{Battery: (*BatterySample)(entry.Battery),
				StartedAt:         entry.StartedAt,
				EndedAt:           entry.EndedAt,
				Mode:              entry.Mode,
				ChangedScreen:     entry.ChangedScreen,
				ImageHash:         entry.ImageHash,
				ImageURL:          entry.ImageURL,
				RefreshIntervalS:  entry.RefreshIntervalS,
				FailureCategory:   entry.FailureCategory,
				FailureMessage:    entry.FailureMessage,
				SkippedRender:     entry.SkippedRender,
				FullRefresh:       entry.FullRefresh,
				ConsecutiveFails:  entry.ConsecutiveFails,
				MaintenanceReason: entry.MaintenanceReason,
			})
		},
		SaveState: func(s cycle.State) error { return a.cycle.saveState(paths, State(s)) },
	}
}
