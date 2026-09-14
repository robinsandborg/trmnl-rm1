package storage_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/robinsandborg/rm1-trmnl/internal/storage"
)

type durableValue struct {
	Count int `json:"count"`
}

func TestDurableBackupRecoversInterruptedAndMissingState(t *testing.T) {
	for _, damage := range []string{"missing", "", `{`, `{"count":"wrong-type"}`, `null`} {
		t.Run(damage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := storage.SaveJSON(path, durableValue{7}); err != nil {
				t.Fatal(err)
			}
			if err := storage.SaveJSON(path, durableValue{8}); err != nil {
				t.Fatal(err)
			}
			if damage == "missing" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte(damage), 0600); err != nil {
				t.Fatal(err)
			}
			value, found, err := storage.LoadRecoverableJSON[durableValue](path, "state", true)
			if err != nil || !found || value.Count != 7 {
				t.Fatalf("recovery = %+v %v %v", value, found, err)
			}
			var restored durableValue
			if err := storage.LoadJSON(path, "state", &restored); err != nil || restored.Count != 7 {
				t.Fatalf("persisted recovery = %+v %v", restored, err)
			}
			if damage != "missing" {
				files, _ := filepath.Glob(path + ".corrupt-*")
				if len(files) != 1 {
					t.Fatalf("diagnostic files: %v", files)
				}
				data, _ := os.ReadFile(files[0])
				if !bytes.Equal(data, []byte(damage)) {
					t.Fatalf("damage changed: %q", data)
				}
			}
		})
	}
}

func TestDamagedStateWithoutBackupCanResetButStrictMetadataCannot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"count":99,`), 0600); err != nil {
		t.Fatal(err)
	}
	value, found, err := storage.LoadRecoverableJSON[durableValue](path, "state", true)
	if err != nil || found || value.Count != 0 {
		t.Fatalf("fresh = %+v %v %v", value, found, err)
	}
	for range 2 {
		if _, _, err := storage.LoadRecoverableJSON[durableValue](path, "metadata", false); err == nil {
			t.Fatal("metadata damage silently ignored")
		}
	}
}

func TestFailedBackupWriteLeavesPrimaryIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	original := []byte(`{"count":7}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".bak", 0700); err != nil {
		t.Fatal(err)
	}
	if err := storage.SaveJSON(path, durableValue{8}); err == nil {
		t.Fatal("expected backup replacement error")
	}
	data, _ := os.ReadFile(path)
	if !bytes.Equal(data, original) {
		t.Fatalf("primary changed on failed save: %q", data)
	}
	temps, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".*.tmp-*"))
	if len(temps) != 0 {
		t.Fatalf("temporary files leaked: %v", temps)
	}
}

func TestReadErrorsAreNotRecoveredAsCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := storage.LoadRecoverableJSON[durableValue](path, "state", true); err == nil {
		t.Fatal("directory read failure swallowed")
	}
}
