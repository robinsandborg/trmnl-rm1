//go:build linux

package trmnl

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInstallPersistsBeforeFirstCycle(t *testing.T) {
	for _, failSave := range []bool{false, true} {
		t.Run(map[bool]string{false: "first cycle state survives", true: "save failure prevents first cycle"}[failSave], func(t *testing.T) {
			paths := isolatedPaths(t)
			writeTestFile(t, paths.ConfigFile, []byte(`{"device_id":"explicit"}`))
			var trace []string
			var hook, service string
			saveErr := errors.New("save failed")
			deps := installDeps{
				writeFile: func(path string, data []byte, mode os.FileMode) error {
					switch path {
					case applianceServicePath:
						if mode != 0o644 {
							t.Fatal("service mode changed")
						}
						service = string(data)
						trace = append(trace, "write service")
					case filepath.Join("/fake-sleep", applianceResumeHookName):
						if mode != 0o755 {
							t.Fatal("hook mode changed")
						}
						hook = string(data)
						trace = append(trace, "write hook")
					default:
						t.Fatalf("unexpected installed path %s", path)
					}
					return nil
				},
				executable:    func() (string, error) { return "/home/root/bin/trmnl-rm1", nil },
				sleepHookDir:  func() (string, error) { return "/fake-sleep", nil },
				stockSyncUnit: func() (string, bool, error) { return "rm-sync.service", true, nil },
				unitEnabled: func(unit string) bool {
					if unit != "xochitl.service" {
						t.Fatal(unit)
					}
					return true
				},
				saveState: func(paths Paths, state State) error {
					trace = append(trace, "save")
					if failSave {
						return saveErr
					}
					return saveState(paths, state)
				},
				run: func(cmd []string) error {
					trace = append(trace, strings.Join(cmd, " "))
					if reflect.DeepEqual(cmd, []string{"systemctl", "start", applianceServiceName}) {
						state, err := loadState(paths)
						if err != nil {
							t.Fatal(err)
						}
						if state.StockSyncUnit != "rm-sync.service" || !state.SyncWasEnabled || !state.XochitlWasEnabled {
							t.Fatalf("first cycle missing restore metadata: %+v", state)
						}
						state.RenderedUpdates = 1
						state.LastImageHash = "first-cycle"
						return saveState(paths, state)
					}
					return nil
				},
			}
			err := runInstallWithDeps(paths, nil, deps)
			if failSave != errors.Is(err, saveErr) || (!failSave && err != nil) {
				t.Fatalf("error=%v", err)
			}
			want := []string{"write service", "write hook", "systemctl daemon-reload", "systemctl stop xochitl.service", "systemctl disable xochitl.service", "systemctl mask xochitl.service", "systemctl stop rm-sync.service", "systemctl disable rm-sync.service", "systemctl mask rm-sync.service", "systemctl enable trmnl-rm1-appliance.service", "save"}
			if !failSave {
				want = append(want, "systemctl start trmnl-rm1-appliance.service")
			}
			if !reflect.DeepEqual(trace, want) {
				t.Fatalf("trace=%v, want %v", trace, want)
			}
			if !failSave {
				state, err := loadState(paths)
				if err != nil || state.RenderedUpdates != 1 || state.LastImageHash != "first-cycle" {
					t.Fatalf("first cycle overwritten: %+v, %v", state, err)
				}
			}
			wantHook := "#!/bin/sh\ncase \"$1\" in\n  post)\n    /bin/systemctl start --no-block trmnl-rm1-appliance.service >/dev/null 2>&1 || true\n    ;;\nesac\n"
			if hook != wantHook {
				t.Fatalf("resume hook changed:\n%s", hook)
			}
			for _, line := range []string{"RequiresMountsFor=/home/root\n", "Type=oneshot\n", "Environment=HOME=/home/root\n", "ExecStart=/home/root/bin/trmnl-rm1 run-once\n", "User=root\n"} {
				if !strings.Contains(service, line) {
					t.Fatalf("service missing %q", line)
				}
			}
		})
	}
}

func TestInstallRejectsArgumentsBeforeEffects(t *testing.T) {
	err := runInstallWithDeps(Paths{}, []string{"unexpected"}, installDeps{})
	if err == nil || err.Error() != "install-appliance does not accept arguments" {
		t.Fatalf("error=%v", err)
	}
}
