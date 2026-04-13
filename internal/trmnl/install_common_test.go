package trmnl

import (
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDisableForApplianceWithRunnerAggregatesFailures(t *testing.T) {
	stopErr := errors.New("stop failed")
	maskErr := errors.New("mask failed")
	var calls [][]string

	run := func(parts []string) error {
		calls = append(calls, append([]string(nil), parts...))
		switch commandString(parts) {
		case "systemctl stop xochitl.service":
			return stopErr
		case "systemctl mask xochitl.service":
			return maskErr
		default:
			return nil
		}
	}

	err := disableForApplianceWithRunner(run, "xochitl.service")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, stopErr) {
		t.Fatalf("expected joined error to contain stop failure, got %v", err)
	}
	if !errors.Is(err, maskErr) {
		t.Fatalf("expected joined error to contain mask failure, got %v", err)
	}
	if !strings.Contains(err.Error(), "stop xochitl.service") {
		t.Fatalf("expected stop context in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "mask xochitl.service") {
		t.Fatalf("expected mask context in error, got %q", err.Error())
	}

	wantCalls := [][]string{
		{"systemctl", "stop", "xochitl.service"},
		{"systemctl", "disable", "xochitl.service"},
		{"systemctl", "mask", "xochitl.service"},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestRunRestoreWithOpsAggregatesCleanupAndCommandFailures(t *testing.T) {
	disableErr := errors.New("disable appliance failed")
	removeServiceErr := errors.New("remove appliance unit failed")
	removeHookErr := errors.New("remove hook failed")
	reloadErr := errors.New("reload failed")
	unmaskXochitlErr := errors.New("unmask xochitl failed")
	enableSyncErr := errors.New("enable sync failed")

	runErrors := map[string]error{
		"systemctl disable --now " + applianceServiceName: disableErr,
		"systemctl daemon-reload":                         reloadErr,
		"systemctl unmask xochitl.service":                unmaskXochitlErr,
		"systemctl enable --now sync.service":             enableSyncErr,
	}

	var runCalls []string
	var removed []string

	err := runRestoreWithOps(State{
		StockSyncUnit:     "sync.service",
		XochitlWasEnabled: false,
		SyncWasEnabled:    true,
	}, applianceOps{
		run: func(parts []string) error {
			cmd := commandString(parts)
			runCalls = append(runCalls, cmd)
			return runErrors[cmd]
		},
		remove: func(path string) error {
			removed = append(removed, path)
			switch path {
			case applianceServicePath:
				return removeServiceErr
			case filepath.Join("/system-sleep", applianceResumeHookName):
				return removeHookErr
			default:
				return nil
			}
		},
		detectSleepHookDir: func() (string, error) {
			return "/system-sleep", nil
		},
	})
	if err == nil {
		t.Fatal("expected restore to fail")
	}

	for _, want := range []error{
		disableErr,
		removeServiceErr,
		removeHookErr,
		reloadErr,
		unmaskXochitlErr,
		enableSyncErr,
	} {
		if !errors.Is(err, want) {
			t.Fatalf("expected joined error to contain %v, got %v", want, err)
		}
	}

	wantRunCalls := []string{
		"systemctl disable --now " + applianceServiceName,
		"systemctl daemon-reload",
		"systemctl unmask xochitl.service",
		"systemctl enable --now xochitl.service",
		"systemctl unmask sync.service",
		"systemctl enable --now sync.service",
	}
	if !reflect.DeepEqual(runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runCalls, wantRunCalls)
	}

	wantRemoved := []string{
		applianceServicePath,
		filepath.Join("/system-sleep", applianceResumeHookName),
	}
	if !reflect.DeepEqual(removed, wantRemoved) {
		t.Fatalf("removed = %#v, want %#v", removed, wantRemoved)
	}
}

func TestRunRestoreWithOpsIgnoresMissingFilesAndRespectsSyncState(t *testing.T) {
	var runCalls []string
	var removed []string

	err := runRestoreWithOps(State{
		StockSyncUnit:  "sync.service",
		SyncWasEnabled: false,
	}, applianceOps{
		run: func(parts []string) error {
			runCalls = append(runCalls, commandString(parts))
			return nil
		},
		remove: func(path string) error {
			removed = append(removed, path)
			return fs.ErrNotExist
		},
		detectSleepHookDir: func() (string, error) {
			return "/system-sleep", nil
		},
	})
	if err != nil {
		t.Fatalf("expected restore to succeed, got %v", err)
	}

	wantRunCalls := []string{
		"systemctl disable --now " + applianceServiceName,
		"systemctl daemon-reload",
		"systemctl unmask xochitl.service",
		"systemctl enable --now xochitl.service",
		"systemctl unmask sync.service",
	}
	if !reflect.DeepEqual(runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runCalls, wantRunCalls)
	}

	wantRemoved := []string{
		applianceServicePath,
		filepath.Join("/system-sleep", applianceResumeHookName),
	}
	if !reflect.DeepEqual(removed, wantRemoved) {
		t.Fatalf("removed = %#v, want %#v", removed, wantRemoved)
	}
}

func TestRunRestoreWithOpsReportsSleepHookLookupFailure(t *testing.T) {
	lookupErr := errors.New("lookup failed")
	var runCalls []string

	err := runRestoreWithOps(State{}, applianceOps{
		run: func(parts []string) error {
			runCalls = append(runCalls, commandString(parts))
			return nil
		},
		remove: func(path string) error {
			return nil
		},
		detectSleepHookDir: func() (string, error) {
			return "", lookupErr
		},
	})
	if err == nil {
		t.Fatal("expected restore to fail")
	}
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected joined error to contain lookup failure, got %v", err)
	}

	wantRunCalls := []string{
		"systemctl disable --now " + applianceServiceName,
		"systemctl daemon-reload",
		"systemctl unmask xochitl.service",
		"systemctl enable --now xochitl.service",
	}
	if !reflect.DeepEqual(runCalls, wantRunCalls) {
		t.Fatalf("run calls = %#v, want %#v", runCalls, wantRunCalls)
	}
}
