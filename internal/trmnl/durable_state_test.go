package trmnl

import (
	"os"
	"reflect"
	"testing"
)

func TestRuntimeRecoveryPreservesIndependentInstallationSnapshot(t *testing.T) {
	for _, damage := range []string{"missing", "", `{`, `null`} {
		t.Run(damage, func(t *testing.T) {
			p := isolatedPaths(t)
			original := State{StockSyncUnit: "rm-sync.service", SyncWasEnabled: true, MaskedNoise: map[string]bool{"chronyd.service": true}}
			if err := saveState(p, original); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(p.StateFile + ".bak"); err != nil {
				t.Fatal(err)
			}
			if damage == "missing" {
				if err := os.Remove(p.StateFile); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(p.StateFile, []byte(damage), 0600); err != nil {
				t.Fatal(err)
			}
			recovered, err := loadState(p)
			if err != nil || !reflect.DeepEqual(recovered, original) {
				t.Fatalf("recovered = %+v %v", recovered, err)
			}
			if err := saveState(p, State{RenderedUpdates: 9}); err != nil {
				t.Fatal(err)
			}
			recovered, err = loadState(p)
			original.RenderedUpdates = 9
			if err != nil || !reflect.DeepEqual(recovered, original) {
				t.Fatalf("metadata overwritten = %+v %v", recovered, err)
			}
		})
	}
}

func TestExplicitDisabledInstallationSnapshotIsDurableMarker(t *testing.T) {
	p := isolatedPaths(t)
	if err := saveInstallationSnapshot(p, State{}); err != nil {
		t.Fatal(err)
	}
	if found, err := hasInstallationSnapshot(p); err != nil || !found {
		t.Fatalf("marker = %v %v", found, err)
	}
	if err := saveState(p, State{XochitlWasEnabled: true}); err != nil {
		t.Fatal(err)
	}
	s, err := loadState(p)
	if err != nil || s.XochitlWasEnabled {
		t.Fatalf("original disabled preference overwritten: %+v %v", s, err)
	}
}

func TestInstallationCorruptionRecoversBackupOrFailsClosed(t *testing.T) {
	p := isolatedPaths(t)
	original := State{StockSyncUnit: "rm-sync.service", XochitlWasEnabled: true}
	if err := saveInstallationSnapshot(p, original); err != nil {
		t.Fatal(err)
	}
	metadata := installationPath(p)
	if err := os.WriteFile(metadata, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := loadState(p)
	if err != nil || s.StockSyncUnit != original.StockSyncUnit || !s.XochitlWasEnabled {
		t.Fatalf("metadata backup: %+v %v", s, err)
	}
	if err := os.Remove(metadata + ".bak"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadata, []byte(`{`), 0600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := loadState(p); err == nil {
			t.Fatal("unrecoverable metadata ignored")
		}
	}
}

func TestReinstallAddsOnlyUnknownServicePreferences(t *testing.T) {
	p := isolatedPaths(t)
	original := State{StockSyncUnit: "rm-sync.service", MaskedNoise: map[string]bool{"chronyd.service": false}}
	if err := saveInstallationSnapshot(p, original); err != nil {
		t.Fatal(err)
	}
	next := State{StockSyncUnit: "replacement.service", XochitlWasEnabled: true, MaskedNoise: map[string]bool{"chronyd.service": true, "memfaultd.service": true}}
	if err := saveInstallationSnapshot(p, next); err != nil {
		t.Fatal(err)
	}
	// Simulate primary loss immediately after installation. Its independent
	// recovery copy must already include all services the installer may mask.
	if err := os.Remove(installationPath(p)); err != nil {
		t.Fatal(err)
	}
	s, err := loadState(p)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{"chronyd.service": false, "memfaultd.service": true}
	if s.StockSyncUnit != original.StockSyncUnit || s.XochitlWasEnabled || !reflect.DeepEqual(s.MaskedNoise, expected) {
		t.Fatalf("preferences = %+v", s)
	}
}

func TestReinstallRepairsBackupAfterInterruptedSnapshotUpdate(t *testing.T) {
	p := isolatedPaths(t)
	original := State{MaskedNoise: map[string]bool{"chronyd.service": false}}
	if err := saveInstallationSnapshot(p, original); err != nil {
		t.Fatal(err)
	}
	oldBackup, err := os.ReadFile(installationPath(p) + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	next := State{MaskedNoise: map[string]bool{"chronyd.service": false, "memfaultd.service": true}}
	if err := saveInstallationSnapshot(p, next); err != nil {
		t.Fatal(err)
	}
	// Power failed after the new primary became durable, before backup refresh.
	if err := os.WriteFile(installationPath(p)+".bak", oldBackup, 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveInstallationSnapshot(p, next); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(installationPath(p)); err != nil {
		t.Fatal(err)
	}
	s, err := loadState(p)
	if err != nil || !reflect.DeepEqual(s.MaskedNoise, next.MaskedNoise) {
		t.Fatalf("backup after reinstall = %+v %v", s, err)
	}
}
