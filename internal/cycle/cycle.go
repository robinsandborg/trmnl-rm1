// Package cycle owns the one-shot fetch/render/persist/schedule lifecycle.
package cycle

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/robinsandborg/rm1-trmnl/internal/display"
)

func Run(opts Options, state State, ops Operations) error {
	startedAt := ops.Now().UTC()
	battery, _ := ops.ReadBattery()
	runtimeMode, err := ops.DetermineMode(state, ops.Now())
	if err != nil {
		return err
	}

	client, cleanupNetwork, err := ops.PrepareNetwork()
	if err != nil {
		return finish(opts, ops, state, CycleLog{
			StartedAt:        startedAt,
			EndedAt:          ops.Now().UTC(),
			Mode:             runtimeMode.Name,
			Battery:          battery,
			FailureCategory:  "wifi",
			FailureMessage:   err.Error(),
			ConsecutiveFails: state.ConsecutiveFailures + 1,
		}, &Error{Category: "wifi", Err: err})
	}
	networkCleaned := false
	defer func() {
		if !networkCleaned {
			cleanupNetwork()
		}
	}()

	filename, imageBytes, resolvedImageURL, interval, err := ops.FetchPayload(client)
	if err != nil {
		ce := classifyCycleError("http", err)
		return finish(opts, ops, state, CycleLog{
			StartedAt:        startedAt,
			EndedAt:          ops.Now().UTC(),
			Mode:             runtimeMode.Name,
			Battery:          battery,
			FailureCategory:  ce.Category,
			FailureMessage:   ce.Error(),
			ConsecutiveFails: state.ConsecutiveFailures + 1,
		}, ce)
	}

	hashValue := sha256Hex(imageBytes)
	changed := hashValue != state.LastImageHash
	skipped := !changed
	fullRefresh := changed && ShouldUseFullRefresh(state.RenderedUpdates, opts.FullRefreshEvery)

	if err := ops.WriteFile(opts.DownloadedImage, imageBytes, 0o600); err != nil {
		return err
	}

	if changed {
		renderMode := display.RefreshPartial
		if fullRefresh {
			renderMode = display.RefreshFull
		}
		if err := ops.Render(imageBytes, opts.LastRenderedImage, renderMode); err != nil {
			ce := classifyCycleError("render", err)
			return finish(opts, ops, state, CycleLog{
				StartedAt:        startedAt,
				EndedAt:          ops.Now().UTC(),
				Mode:             runtimeMode.Name,
				ImageHash:        hashValue,
				ImageURL:         resolvedImageURL,
				RefreshIntervalS: int(interval.Seconds()),
				Battery:          battery,
				FailureCategory:  ce.Category,
				FailureMessage:   ce.Error(),
				ConsecutiveFails: state.ConsecutiveFailures + 1,
				FullRefresh:      fullRefresh,
			}, ce)
		}
		state.LastImageHash = hashValue
		state.LastImageURL = resolvedImageURL
		state.LastFilename = filename
		state.RenderedUpdates++
	}

	state.LastCycleChanged = changed
	state.LastIntervalSeconds = int(interval.Seconds())
	state.LastMode = runtimeMode.Name
	state.LastSuccessAt = ops.Now().UTC()
	state.LastFailureAt = time.Time{}
	state.LastFailureCategory = ""
	state.LastFailureMessage = ""
	state.ConsecutiveFailures = 0

	effectiveMode, err := ops.Plan(interval, runtimeMode)
	if err != nil {
		ce := classifyCycleError("schedule", err)
		return finish(opts, ops, state, CycleLog{
			StartedAt:        startedAt,
			EndedAt:          ops.Now().UTC(),
			Mode:             runtimeMode.Name,
			ChangedScreen:    changed,
			ImageHash:        hashValue,
			ImageURL:         resolvedImageURL,
			RefreshIntervalS: int(interval.Seconds()),
			Battery:          battery,
			SkippedRender:    skipped,
			FullRefresh:      fullRefresh,
			FailureCategory:  ce.Category,
			FailureMessage:   ce.Error(),
			ConsecutiveFails: state.ConsecutiveFailures + 1,
		}, ce)
	}

	logEntry := CycleLog{
		StartedAt:         startedAt,
		EndedAt:           ops.Now().UTC(),
		Mode:              effectiveMode.Name,
		ChangedScreen:     changed,
		ImageHash:         hashValue,
		ImageURL:          resolvedImageURL,
		RefreshIntervalS:  int(interval.Seconds()),
		Battery:           battery,
		SkippedRender:     skipped,
		FullRefresh:       fullRefresh,
		ConsecutiveFails:  state.ConsecutiveFailures,
		MaintenanceReason: effectiveMode.MaintenanceReason,
	}

	if err := ops.AppendLog(logEntry); err != nil {
		return err
	}
	if err := ops.SaveState(state); err != nil {
		return err
	}

	cleanupNetwork()
	networkCleaned = true

	if effectiveMode.ShouldSuspend {
		if err := ops.Suspend(); err != nil {
			return finish(opts, ops, state, CycleLog{
				StartedAt:        startedAt,
				EndedAt:          ops.Now().UTC(),
				Mode:             effectiveMode.Name,
				ChangedScreen:    changed,
				ImageHash:        hashValue,
				ImageURL:         resolvedImageURL,
				RefreshIntervalS: int(interval.Seconds()),
				Battery:          battery,
				SkippedRender:    skipped,
				FullRefresh:      fullRefresh,
			}, &Error{Category: "suspend", Err: err})
		}
	}
	return nil
}

func finish(opts Options, ops Operations, state State, entry CycleLog, err error) error {
	ended := ops.Now().UTC()
	entry.EndedAt = ended
	state.LastFailureAt = ended
	if ce, ok := err.(*Error); ok {
		state.LastFailureCategory = ce.Category
		state.LastFailureMessage = ce.Error()
		entry.FailureCategory = ce.Category
		entry.FailureMessage = ce.Error()
	} else {
		state.LastFailureMessage = err.Error()
		entry.FailureMessage = err.Error()
	}
	state.ConsecutiveFailures++
	state.LastMode = entry.Mode
	entry.ConsecutiveFails = state.ConsecutiveFailures

	if state.ConsecutiveFailures >= opts.FailureThreshold {
		entry.MaintenanceReason = "failure-threshold"
	}

	if logErr := ops.AppendLog(entry); logErr != nil {
		return errors.Join(err, logErr)
	}
	if saveErr := ops.SaveState(state); saveErr != nil {
		return errors.Join(err, saveErr)
	}

	runtimeMode, modeErr := ops.DetermineMode(state, ops.Now())
	if modeErr == nil {
		_, _ = ops.Plan(opts.RefreshFallback, runtimeMode)
	}
	return err
}

func classifyCycleError(category string, err error) *Error {
	if ce, ok := err.(*Error); ok {
		return ce
	}
	return &Error{Category: category, Err: err}
}

func sha256Hex(data []byte) string {
	return fmt.Sprintf("%x", sha256Sum(data))
}

func sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}

func ShouldUseFullRefresh(renderedUpdates, fullRefreshEvery int) bool {
	return fullRefreshEvery > 0 && (renderedUpdates+1)%fullRefreshEvery == 0
}
