package appliance_test

import (
	"errors"
	"github.com/robinsandborg/rm1-trmnl/internal/appliance"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestRestoreRemovalErrorText(t *testing.T) {
	fail := errors.New("denied")
	err := appliance.Restore(appliance.Snapshot{}, appliance.RestoreOps{Run: func([]string) error { return nil }, Remove: func(string) error { return fail }, SleepHookDir: func() (string, error) { return "/sleep", nil }})
	want := "remove appliance unit file /etc/systemd/system/trmnl-rm1-appliance.service: denied\nremove appliance resume hook /sleep/trmnl-rm1-resume: denied"
	if err == nil || err.Error() != want || !errors.Is(err, fail) {
		t.Fatalf("%v", err)
	}
}

func TestStockUnitFallback(t *testing.T) {
	var calls [][]string
	unit, enabled, err := appliance.DetectStockSyncUnit(func(argv []string) (string, error) {
		calls = append(calls, argv)
		if argv[2] == "sync.service" {
			return "", errors.New("not found")
		}
		if argv[1] == "status" {
			return "", errors.New("Loaded: loaded, inactive")
		}
		return "enabled", nil
	})
	want := [][]string{{"systemctl", "status", "sync.service"}, {"systemctl", "status", "rm-sync.service"}, {"systemctl", "is-enabled", "rm-sync.service"}}
	if err != nil || unit != "rm-sync.service" || !enabled || !reflect.DeepEqual(calls, want) {
		t.Fatalf("%s %v %v %v", unit, enabled, err, calls)
	}
}

func TestInstallStopsAtFailedWrite(t *testing.T) {
	fail := errors.New("write failed")
	var paths []string
	err := appliance.Install(appliance.Snapshot{}, appliance.InstallDeps{
		Executable: func() (string, error) { return "/bin/client", nil }, SleepHookDir: func() (string, error) { return "/sleep", nil },
		WriteFile: func(path string, data []byte, mode os.FileMode) error {
			paths = append(paths, path)
			if !strings.Contains(string(data), "ExecStart=/bin/client run-once") || mode != 0o644 {
				t.Fatal("unit changed")
			}
			return fail
		},
		Run: func([]string) error { t.Fatal("command after failed write"); return nil },
	})
	if !errors.Is(err, fail) || !reflect.DeepEqual(paths, []string{appliance.ServicePath}) {
		t.Fatalf("%v %v", paths, err)
	}
}

func TestRestoreDeployedMaskedServices(t *testing.T) {
	var calls [][]string
	err := appliance.Restore(appliance.Snapshot{MaskedNoise: map[string]bool{"chronyd.service": true, "memfaultd.service": false, "unknown.service": true}}, appliance.RestoreOps{
		Run: func(argv []string) error { calls = append(calls, argv); return nil }, Remove: func(string) error { return nil }, SleepHookDir: func() (string, error) { return "/sleep", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"systemctl", "unmask", "chronyd.service"}, {"systemctl", "enable", "--now", "chronyd.service"}, {"systemctl", "unmask", "memfaultd.service"}}
	if !reflect.DeepEqual(calls[len(calls)-3:], want) {
		t.Fatalf("%v", calls)
	}
	for _, call := range calls {
		if strings.Contains(strings.Join(call, " "), "unknown.service") {
			t.Fatal("restored an unrecognized unit")
		}
	}
}

func TestReinstallKeepsDeployedOriginalMetadata(t *testing.T) {
	before := appliance.Snapshot{StockSyncUnit: "rm-sync.service", SyncWasEnabled: true, XochitlWasEnabled: true, MaskedNoise: map[string]bool{"chronyd.service": true}}
	var saved appliance.Snapshot
	err := appliance.Install(before, appliance.InstallDeps{
		Executable: func() (string, error) { return "/bin/client", nil }, SleepHookDir: func() (string, error) { return "/sleep", nil }, WriteFile: func(string, []byte, os.FileMode) error { return nil }, StockSyncUnit: func() (string, bool, error) { return "rm-sync.service", false, nil }, UnitEnabled: func(string) bool { return false }, UnitExists: func(unit string) bool { return unit == "chronyd.service" || unit == "memfaultd.service" }, Run: func([]string) error { return nil }, SaveState: func(s appliance.Snapshot) error { saved = s; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !saved.SyncWasEnabled || !saved.XochitlWasEnabled || !saved.MaskedNoise["chronyd.service"] {
		t.Fatalf("original state overwritten: %+v", saved)
	}
	if _, ok := saved.MaskedNoise["memfaultd.service"]; !ok {
		t.Fatal("newly masked service not recorded")
	}
	if len(before.MaskedNoise) != 1 {
		t.Fatal("input metadata mutated")
	}
}

func TestRestoreRecoversNetworkingBeforeStockUI(t *testing.T) {
	var trace []string
	fail := errors.New("radio failed")
	err := appliance.Restore(appliance.Snapshot{}, appliance.RestoreOps{
		Run: func(argv []string) error { trace = append(trace, strings.Join(argv, " ")); return nil }, Remove: func(string) error { return nil }, SleepHookDir: func() (string, error) { return "/sleep", nil }, RestoreNetwork: func() error { trace = append(trace, "network"); return fail },
	})
	if !errors.Is(err, fail) || err.Error() != "restore wireless networking: radio failed" {
		t.Fatal(err)
	}
	want := []string{"systemctl disable --now trmnl-rm1-appliance.service", "systemctl daemon-reload", "network", "systemctl unmask xochitl.service", "systemctl enable --now xochitl.service"}
	if !reflect.DeepEqual(trace, want) {
		t.Fatalf("%v", trace)
	}
}
