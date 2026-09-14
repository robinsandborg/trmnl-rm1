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
