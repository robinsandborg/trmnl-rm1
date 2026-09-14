package cycle

import (
	"errors"
	"fmt"
	"github.com/robinsandborg/rm1-trmnl/internal/display"
	"github.com/robinsandborg/rm1-trmnl/internal/power"
	"time"
)

func retryInterval(failures int) time.Duration {
	d := 5 * time.Minute
	for i := 1; i < failures && d < time.Hour; i++ {
		d *= 2
	}
	if d > time.Hour {
		d = time.Hour
	}
	return d
}

func localScreen(opts Options, ops Operations, state *State, name, message string) (bool, error) {
	if state.LocalScreen == name && state.BootID == opts.BootID {
		return false, nil
	}
	var cache []byte
	if name == "offline" && ops.ReadFile != nil {
		cache, _ = ops.ReadFile(opts.DownloadedImage)
	}
	if err := ops.Render(display.Status(message, cache), opts.LastRenderedImage, display.RefreshFull); err != nil {
		return false, err
	}
	state.LocalScreen = name
	state.BootID = opts.BootID
	return true, nil
}

func batteryCycle(opts Options, ops Operations, state *State, battery *power.BatterySample, decision power.BatteryDecision, started time.Time) error {
	if decision.Shutdown {
		return criticalBatteryCycle(opts, ops, state, battery, started)
	}
	message := "Please charge the device"
	if !decision.Valid {
		message = "Battery unavailable - check device"
	}
	if decision.External {
		message = "Charging - updates paused"
	}
	rendered, err := localScreen(opts, ops, state, decision.Reason, message)
	if err != nil {
		return err
	}
	interval := opts.BatteryPolicy.Defaults().CheckInterval
	state.NextAttemptAt = ops.Now().Add(interval)
	state.LastMode = decision.Reason
	mode := power.Mode{Name: decision.Reason, MaintenanceReason: decision.Reason, ShouldSuspend: true}
	// External power with sufficient charge reaches the ordinary maintenance
	// policy. Below recovery threshold, even USB service waits between checks.
	effective, err := ops.Plan(interval, mode)
	if err != nil {
		return err
	}
	if err = ops.SaveState(*state); err != nil {
		return err
	}
	if err = ops.AppendLog(CycleLog{StartedAt: started, EndedAt: ops.Now(), Mode: mode.Name, Battery: battery, ChangedScreen: rendered, SkippedRender: !rendered, FullRefresh: rendered, RefreshIntervalS: int(interval.Seconds()), MaintenanceReason: decision.Reason}); err != nil {
		return err
	}
	if effective.ShouldSuspend {
		return ops.Suspend()
	}
	return nil
}

func recoverCycle(opts Options, ops Operations, state State, battery *power.BatterySample, started time.Time, cause error) error {
	state.ConsecutiveFailures++
	state.LastFailureAt = ops.Now()
	state.LastFailureCategory = "startup"
	var ce *Error
	if errors.As(cause, &ce) {
		state.LastFailureCategory = ce.Category
	}
	state.LastFailureMessage = cause.Error()
	state.LastMode = "recovery"
	interval := retryInterval(state.ConsecutiveFailures)
	state.NextAttemptAt = ops.Now().Add(interval)
	// Do not overwrite an already useful normal display for a transient failure
	// within the same boot; boot failures need an honest locally rendered status.
	var errs []error
	rendered := false
	if !state.BatteryLow && (state.BootID != opts.BootID || state.LocalScreen != "") {
		var drawErr error
		rendered, drawErr = localScreen(opts, ops, &state, "offline", "Offline - retrying automatically")
		errs = append(errs, drawErr)
	}
	mode, err := ops.DetermineMode(state, ops.Now())
	if err != nil {
		mode = power.Mode{Name: "recovery"}
		errs = append(errs, err)
	}
	decision := opts.BatteryPolicy.Decide(battery, state.BatteryLow)
	if decision.Valid && !decision.External {
		mode = power.Mode{Name: "recovery", MaintenanceReason: "battery-retry", ShouldSuspend: true}
	}
	effective, planErr := ops.Plan(interval, mode)
	errs = append(errs, planErr)
	errs = append(errs, ops.SaveState(state), ops.AppendLog(CycleLog{StartedAt: started, EndedAt: ops.Now(), Mode: "recovery", Battery: battery, ChangedScreen: rendered, SkippedRender: !rendered, FullRefresh: rendered, FailureCategory: state.LastFailureCategory, FailureMessage: cause.Error(), ConsecutiveFails: state.ConsecutiveFailures, RefreshIntervalS: int(interval.Seconds()), MaintenanceReason: "bounded-retry"}))
	if planErr == nil && effective.ShouldSuspend {
		errs = append(errs, ops.Suspend())
	}
	return errors.Join(append([]error{fmt.Errorf("%s: retry in %s: %w", state.LastFailureCategory, interval, cause)}, errs...)...)
}

// Critical shutdown owns its outcome independently of ordinary recovery.
// No RTC/timer plan is necessary before powering off. Rendering and durable
// diagnostics are best effort: failure of either must not prevent shutdown.
func criticalBatteryCycle(opts Options, ops Operations, state *State, battery *power.BatterySample, started time.Time) error {
	rendered, drawErr := localScreen(opts, ops, state, "critical-battery", "Please charge the device")
	state.LastMode = "critical-battery"
	state.NextAttemptAt = ops.Now().Add(5 * time.Minute)
	saveErr := ops.SaveState(*state)
	logErr := ops.AppendLog(CycleLog{StartedAt: started, EndedAt: ops.Now(), Mode: "critical-battery", Battery: battery, ChangedScreen: rendered, SkippedRender: !rendered, FullRefresh: rendered, MaintenanceReason: "critical-battery"})
	var shutdownErr error
	if ops.Shutdown == nil {
		shutdownErr = errors.New("critical battery shutdown unavailable")
	} else {
		shutdownErr = ops.Shutdown()
	}
	if err := errors.Join(drawErr, saveErr, logErr, shutdownErr); err != nil {
		return fmt.Errorf("critical-battery: %w", err)
	}
	return nil
}
