package storage_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robinsandborg/rm1-trmnl/internal/storage"
)

func TestJSONFailureContracts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	dst := struct {
		Count int `json:"count"`
	}{Count: 7}
	if err := storage.LoadJSON(path, "state", &dst); err != nil || dst.Count != 7 {
		t.Fatalf("missing: %+v %v", dst, err)
	}
	if err := os.WriteFile(path, []byte(`{"count":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	err := storage.LoadJSON(path, "state", &dst)
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) || !strings.HasPrefix(err.Error(), "parse state "+path+": ") {
		t.Fatalf("decode: %v", err)
	}
	// Save encodes before opening, while append opens before encoding.
	if err := storage.SaveJSON(path, make(chan int)); err == nil {
		t.Fatal("expected encoding error")
	}
	data, _ := os.ReadFile(path)
	if string(data) != `{"count":"bad"}` {
		t.Fatal("save truncated on encoding error")
	}
	log := filepath.Join(t.TempDir(), "cycles.log")
	if err := storage.AppendJSON(log, make(chan int)); err == nil {
		t.Fatal("expected encoding error")
	}
	info, err := os.Stat(log)
	if err != nil || info.Size() != 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("append open contract: %v %v", info, err)
	}
}

func TestWritesPreserveModeAndRawBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "downloaded.png")
	if err := os.WriteFile(path, nil, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(path, []byte{0, 255, 1}, 0o600); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(data) != string([]byte{0, 255, 1}) || info.Mode().Perm() != 0o640 {
		t.Fatal("bytes or mode changed")
	}
	if err := storage.SaveJSON(path, struct{ N int }{3}); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != `{
  "N": 3
}` {
		t.Fatalf("JSON: %q", data)
	}
}
