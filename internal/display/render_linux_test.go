//go:build linux

package display_test

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/robinsandborg/rm1-trmnl/internal/display"
)

type command struct{ Args, Env []string }

func renderOptions() display.Options {
	return display.Options{Width: 4, Height: 2, Rotation: 3, FBInkBinary: "fbink-test", FBDepthBinary: "fbdepth-test", BitDepth: 8, WaveformPartial: "GL16", WaveformFull: "GC16", NoViewport: true}
}

func TestRenderCommandsAndFailureOrder(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		mode                       display.RefreshMode
		change                     func(*display.Options)
		failAt                     int
		failure, wantError, golden string
		commands                   func(string) []command
	}{
		{"partial", display.RefreshPartial, nil, 0, "", "", "landscape-golden.png", func(path string) []command {
			return []command{
				{Args: []string{"fbdepth-test", "-d", "8"}}, {Args: []string{"fbdepth-test", "-R", "3"}}, {[]string{"fbink-test", "--image", "file=" + path + ",x=0,y=0", "--waveform", "GL16", "--noviewport"}, []string{"FBINK_NO_SW_ROTA=1"}},
			}
		}},
		{"full custom knobs", display.RefreshFull, func(o *display.Options) {
			o.BitDepth = 16
			o.Rotation = 1
			o.WaveformFull = "GC4"
			o.DitherMode = "ORDERED"
			o.NoViewport = false
		}, 0, "", "", "landscape-golden.png", func(path string) []command {
			return []command{
				{Args: []string{"fbdepth-test", "-d", "16"}}, {Args: []string{"fbdepth-test", "-R", "1"}}, {[]string{"fbink-test", "--image", "file=" + path + ",x=0,y=0", "--waveform", "GC4", "--dither", "ORDERED", "--flash"}, []string{"FBINK_NO_SW_ROTA=1"}},
			}
		}},
		{"software rotation", display.RefreshFull, func(o *display.Options) { o.SkipRotation = true }, 0, "", "", "rotate3-golden.png", func(path string) []command {
			return []command{
				{Args: []string{"fbdepth-test", "-d", "8"}}, {[]string{"fbink-test", "--image", "file=" + path + ",x=0,y=0", "--waveform", "GC16", "--noviewport", "--flash"}, []string{"FBINK_NO_SW_ROTA=1"}},
			}
		}},
		{"custom command", display.RefreshFull, func(o *display.Options) {
			o.RendererCommand = []string{"paint-test", "image={image}", "mode={mode}", "again={image}:{mode}"}
		}, 0, "", "", "landscape-golden.png", func(path string) []command {
			return []command{
				{Args: []string{"paint-test", "image=" + path, "mode=full", "again=" + path + ":full"}},
			}
		}},
		{"unknown mode uses partial waveform", display.RefreshMode("custom-mode"), func(o *display.Options) { o.SkipRotation = true; o.Rotation = 4; o.WaveformPartial = "DU" }, 0, "", "", "rotate4-golden.png", func(path string) []command {
			return []command{
				{Args: []string{"fbdepth-test", "-d", "8"}}, {[]string{"fbink-test", "--image", "file=" + path + ",x=0,y=0", "--waveform", "DU", "--noviewport"}, []string{"FBINK_NO_SW_ROTA=1"}},
			}
		}},
		{"depth fails", display.RefreshPartial, nil, 1, "depth failed", "prepare FBInk framebuffer: depth failed", "landscape-golden.png", func(string) []command { return []command{{Args: []string{"fbdepth-test", "-d", "8"}}} }},
		{"rotation fails", display.RefreshPartial, nil, 2, "rotation failed", "prepare FBInk framebuffer: rotation failed", "landscape-golden.png", func(string) []command {
			return []command{{Args: []string{"fbdepth-test", "-d", "8"}}, {Args: []string{"fbdepth-test", "-R", "3"}}}
		}},
		{"unsupported rotation remains best effort", display.RefreshPartial, nil, 2, "rotation not supported on your device", "", "landscape-golden.png", func(path string) []command {
			return []command{
				{Args: []string{"fbdepth-test", "-d", "8"}}, {Args: []string{"fbdepth-test", "-R", "3"}}, {[]string{"fbink-test", "--image", "file=" + path + ",x=0,y=0", "--waveform", "GL16", "--noviewport"}, []string{"FBINK_NO_SW_ROTA=1"}},
			}
		}},
		{"FBInk fails after PNG write", display.RefreshPartial, nil, 3, "FBInk failed", "FBInk failed", "landscape-golden.png", func(path string) []command {
			return []command{
				{Args: []string{"fbdepth-test", "-d", "8"}}, {Args: []string{"fbdepth-test", "-R", "3"}}, {[]string{"fbink-test", "--image", "file=" + path + ",x=0,y=0", "--waveform", "GL16", "--noviewport"}, []string{"FBINK_NO_SW_ROTA=1"}},
			}
		}},
		{"custom fails after PNG write", display.RefreshPartial, func(o *display.Options) { o.RendererCommand = []string{"paint-test", "{image}", "{mode}"} }, 1, "custom failed", "custom failed", "landscape-golden.png", func(path string) []command { return []command{{Args: []string{"paint-test", path, "partial"}}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := renderOptions()
			if tc.change != nil {
				tc.change(&opts)
			}
			before := append([]string(nil), opts.RendererCommand...)
			path := filepath.Join(t.TempDir(), "prepared image.png")
			failure := errors.New(tc.failure)
			var calls []command
			err := display.Render(opts, readFixture(t, "landscape.png"), path, tc.mode, func(argv, env []string) error {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("command before image write: %v", err)
				}
				img, err := png.Decode(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				assertPixels(t, img, golden(t, tc.golden))
				calls = append(calls, command{append([]string(nil), argv...), append([]string(nil), env...)})
				if len(calls) == tc.failAt {
					return failure
				}
				return nil
			})
			message := ""
			if err != nil {
				message = err.Error()
			}
			if message != tc.wantError {
				t.Fatalf("error=%v want %q", err, tc.wantError)
			}
			if tc.wantError != "" && !errors.Is(err, failure) {
				t.Fatalf("error identity lost: %v", err)
			}
			if !reflect.DeepEqual(calls, tc.commands(path)) {
				t.Fatalf("commands=%#v want %#v", calls, tc.commands(path))
			}
			if !reflect.DeepEqual(opts.RendererCommand, before) {
				t.Fatal("custom command configuration mutated")
			}
		})
	}
}

func TestRenderDecodeAndWriteFailuresAvoidCommands(t *testing.T) {
	t.Run("decode preserves existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "current.png")
		before := []byte("previous file")
		if err := os.WriteFile(path, before, 0o600); err != nil {
			t.Fatal(err)
		}
		err := display.Render(renderOptions(), []byte("invalid image"), path, display.RefreshPartial, func([]string, []string) error { t.Fatal("command after decode failure"); return nil })
		if err == nil || err.Error() != "decode image: image: unknown format" {
			t.Fatalf("error=%v", err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("file changed on decode error: %v", err)
		}
	})
	t.Run("write failure", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing", "current.png")
		err := display.Render(renderOptions(), readFixture(t, "landscape.png"), path, display.RefreshPartial, func([]string, []string) error { t.Fatal("command after write failure"); return nil })
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) || pathErr.Path != path || pathErr.Op != "open" {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("existing permissions survive write", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "current.png")
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := display.Render(renderOptions(), readFixture(t, "landscape.png"), path, display.RefreshPartial, func([]string, []string) error { return nil }); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("file mode changed: %v %v", info, err)
		}
	})
}
