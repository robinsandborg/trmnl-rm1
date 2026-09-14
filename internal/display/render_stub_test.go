//go:build !linux

package display_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/robinsandborg/rm1-trmnl/internal/display"
)

func TestRenderUnsupportedBeforeEffects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.png")
	if err := os.WriteFile(path, []byte("previous"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := display.Render(display.Options{}, []byte("invalid image"), path, display.RefreshFull, func([]string, []string) error { t.Fatal("unexpected renderer command"); return nil })
	if err == nil || err.Error() != "rendering is only supported on Linux" {
		t.Fatalf("error=%v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "previous" {
		t.Fatalf("file changed: %q %v", data, err)
	}
}
