//go:build linux

package trmnl

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Only these fake executables are visible on PATH. Their shell builtins record
// arguments/environment and verify that the prepared file exists before any
// command. No framebuffer tools or device paths are touched by these tests.
func fakeDisplayCommands(t *testing.T, output string) string {
	t.Helper()
	root := t.TempDir()
	trace := filepath.Join(root, "trace")
	t.Setenv("PATH", root)
	t.Setenv("FBINK_NO_SW_ROTA", "inherited")
	t.Setenv("TRMNL_DISPLAY_TEST_TRACE", trace)
	t.Setenv("TRMNL_DISPLAY_TEST_IMAGE", output)
	t.Setenv("TRMNL_DISPLAY_TEST_FAIL", "")
	t.Setenv("TRMNL_DISPLAY_TEST_ERROR", "")
	script := `#!/bin/sh
if ! test -s "$TRMNL_DISPLAY_TEST_IMAGE"; then
  printf 'image missing before command\n' >&2
  exit 99
fi
name=${0##*/}
{
  printf '%s' "$name"
  printf ' <%s>' "$@"
  printf ' env=%s\n' "$FBINK_NO_SW_ROTA"
} >> "$TRMNL_DISPLAY_TEST_TRACE"
case "$name:$1" in
  fbdepth:-R|depth-alt:-R) stage=rotation ;;
  fbink:*|ink-alt:*) stage=render ;;
  *) stage=other ;;
esac
if test "$TRMNL_DISPLAY_TEST_FAIL" = "$stage"; then
  printf '%s\n' "$TRMNL_DISPLAY_TEST_ERROR" >&2
  exit 1
fi
`
	for _, name := range []string{"fbink", "fbdepth", "ink-alt", "depth-alt", "paint"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return trace
}

func displayFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "display", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func compareDisplayPixels(t *testing.T, got, want image.Image) {
	t.Helper()
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds=%v want %v", got.Bounds(), want.Bounds())
	}
	b := want.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gr, gg, gb, ga := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb || ga != wa {
				t.Fatalf("pixel mismatch at (%d,%d)", x, y)
			}
		}
	}
}

func TestRenderFacadeLegacyPixelCorpus(t *testing.T) {
	var cases []struct {
		Name, Input, Golden     string
		Width, Height, Rotation int
		SkipRotation            bool
	}
	if err := json.Unmarshal(displayFixture(t, "cases.json"), &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "prepared image.png")
			trace := fakeDisplayCommands(t, path)
			cfg := Config{DisplayWidth: tc.Width, DisplayHeight: tc.Height, FBInkRotation: tc.Rotation, FBInkSkipRotation: tc.SkipRotation, RendererCommand: []string{"paint", "{image}", "{mode}"}}
			if err := renderImage(cfg, displayFixture(t, tc.Input), path, RefreshFull); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			want, err := png.Decode(bytes.NewReader(displayFixture(t, tc.Golden)))
			if err != nil {
				t.Fatal(err)
			}
			compareDisplayPixels(t, got, want)
			log, err := os.ReadFile(trace)
			if err != nil || string(log) != "paint <"+path+"> <full> env=inherited\n" {
				t.Fatalf("trace=%q error=%v", log, err)
			}
		})
	}
}

func TestRenderFacadeCommandMapping(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		cfg                               Config
		mode                              RefreshMode
		failStage, failMessage, wantError string
		want                              func(string) string
	}{
		{"defaults", Config{}, RefreshPartial, "", "", "", func(path string) string {
			return "fbdepth <-d> <8> env=inherited\nfbdepth <-R> <3> env=inherited\nfbink <--image> <file=" + path + ",x=0,y=0> <--waveform> <GL16> env=1\n"
		}},
		{"configured full", Config{FBInkBinary: "ink-alt", FBDepthBinary: "depth-alt", FBInkBitDepth: 16, FBInkRotation: 1, FBInkWaveformFull: "GC4", FBInkDitherMode: "ORDERED", FBInkNoViewport: true}, RefreshFull, "", "", "", func(path string) string {
			return "depth-alt <-d> <16> env=inherited\ndepth-alt <-R> <1> env=inherited\nink-alt <--image> <file=" + path + ",x=0,y=0> <--waveform> <GC4> <--dither> <ORDERED> <--noviewport> <--flash> env=1\n"
		}},
		{"configured partial skips framebuffer rotation", Config{FBInkSkipRotation: true, FBInkWaveformPartial: "DU"}, RefreshPartial, "", "", "", func(path string) string {
			return "fbdepth <-d> <8> env=inherited\nfbink <--image> <file=" + path + ",x=0,y=0> <--waveform> <DU> env=1\n"
		}},
		{"unsupported framebuffer rotation", Config{}, RefreshPartial, "rotation", "not supported on your device", "", func(path string) string {
			return "fbdepth <-d> <8> env=inherited\nfbdepth <-R> <3> env=inherited\nfbink <--image> <file=" + path + ",x=0,y=0> <--waveform> <GL16> env=1\n"
		}},
		{"renderer fails after write", Config{}, RefreshPartial, "render", "renderer failed", "renderer failed", func(path string) string {
			return "fbdepth <-d> <8> env=inherited\nfbdepth <-R> <3> env=inherited\nfbink <--image> <file=" + path + ",x=0,y=0> <--waveform> <GL16> env=1\n"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "current.png")
			trace := fakeDisplayCommands(t, path)
			t.Setenv("TRMNL_DISPLAY_TEST_FAIL", tc.failStage)
			t.Setenv("TRMNL_DISPLAY_TEST_ERROR", tc.failMessage)
			cfg := tc.cfg
			cfg.DisplayWidth = 4
			cfg.DisplayHeight = 2
			err := renderImage(cfg, displayFixture(t, "landscape.png"), path, tc.mode)
			if tc.wantError == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantError != "" && (err == nil || err.Error() != "fbink --image file="+path+",x=0,y=0 --waveform GL16: "+tc.wantError) {
				t.Fatalf("error=%v", err)
			}
			log, err := os.ReadFile(trace)
			if err != nil || string(log) != tc.want(path) {
				t.Fatalf("trace=%q want %q, error=%v", log, tc.want(path), err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			name := "landscape-golden.png"
			if cfg.FBInkSkipRotation {
				name = "rotate3-golden.png"
			}
			want, err := png.Decode(bytes.NewReader(displayFixture(t, name)))
			if err != nil {
				t.Fatal(err)
			}
			compareDisplayPixels(t, got, want)
		})
	}
}

func TestRenderFacadeEffectiveDimensions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.png")
	fakeDisplayCommands(t, path)
	if err := renderImage(Config{DisplayWidth: -1, DisplayHeight: 0, RendererCommand: []string{"paint", "{image}"}}, displayFixture(t, "landscape.png"), path, RefreshPartial); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := png.DecodeConfig(f)
	if err != nil || info.Width != 1872 || info.Height != 1404 {
		t.Fatalf("dimensions=%+v, %v", info, err)
	}
}

func TestRenderFacadeEarlyFailures(t *testing.T) {
	for _, badData := range []bool{true, false} {
		t.Run(map[bool]string{true: "decode", false: "write"}[badData], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "missing", "current.png")
			trace := fakeDisplayCommands(t, path)
			data := displayFixture(t, "landscape.png")
			if badData {
				data = []byte("invalid")
			}
			err := renderImage(Config{DisplayWidth: 4, DisplayHeight: 2}, data, path, RefreshPartial)
			if err == nil {
				t.Fatal("expected error")
			}
			if badData && err.Error() != "decode image: image: unknown format" {
				t.Fatal(err)
			}
			if !badData && !strings.Contains(err.Error(), "open "+path) {
				t.Fatal(err)
			}
			if _, err := os.Stat(trace); !os.IsNotExist(err) {
				t.Fatalf("unexpected command trace: %v", err)
			}
		})
	}
}
