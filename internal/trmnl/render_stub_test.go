//go:build !linux

package trmnl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderFacadeUnsupported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.png")
	err := renderImage(Config{RendererCommand: []string{"must-not-run"}}, []byte("invalid"), path, RefreshFull)
	if err == nil || err.Error() != "rendering is only supported on Linux" {
		t.Fatalf("error=%v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected prepared file: %v", err)
	}
}
