//go:build linux

package display

import (
	"fmt"
	"strconv"
	"strings"
)

// Render writes the prepared PNG before invoking FBInk or a custom renderer.
// run receives argv and additional environment entries, preserving the host
// command runner's existing error behavior.
func Render(opts Options, imageBytes []byte, outputPath string, mode RefreshMode, run CommandRunner) error {
	final, err := Prepare(opts, imageBytes)
	if err != nil {
		return err
	}
	if err := writePNG(outputPath, final); err != nil {
		return err
	}
	if len(opts.RendererCommand) > 0 {
		return run(expandRendererCommand(opts.RendererCommand, outputPath, mode), nil)
	}
	return renderWithFBInk(opts, outputPath, mode, run)
}

func expandRendererCommand(parts []string, imagePath string, mode RefreshMode) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ReplaceAll(part, "{image}", imagePath)
		part = strings.ReplaceAll(part, "{mode}", string(mode))
		out = append(out, part)
	}
	return out
}

func renderWithFBInk(opts Options, imagePath string, mode RefreshMode, run CommandRunner) error {
	if err := prepareFBInkFramebuffer(opts, run); err != nil {
		return fmt.Errorf("prepare FBInk framebuffer: %w", err)
	}

	args := []string{
		opts.FBInkBinary,
		"--image",
		"file=" + imagePath + ",x=0,y=0",
		"--waveform",
		fbinkWaveformForMode(opts, mode),
	}
	if opts.DitherMode != "" {
		args = append(args, "--dither", opts.DitherMode)
	}
	if opts.NoViewport {
		args = append(args, "--noviewport")
	}
	if mode == RefreshFull {
		args = append(args, "--flash")
	}
	return run(args, []string{"FBINK_NO_SW_ROTA=1"})
}

func prepareFBInkFramebuffer(opts Options, run CommandRunner) error {
	if err := run([]string{
		opts.FBDepthBinary,
		"-d",
		strconv.Itoa(opts.BitDepth),
	}, nil); err != nil {
		return err
	}
	if opts.SkipRotation {
		return nil
	}
	if err := run([]string{
		opts.FBDepthBinary,
		"-R",
		strconv.Itoa(opts.Rotation),
	}, nil); err != nil {
		if strings.Contains(err.Error(), "not supported on your device") {
			return nil
		}
		return err
	}
	return nil
}

func fbinkWaveformForMode(opts Options, mode RefreshMode) string {
	if mode == RefreshFull {
		return opts.WaveformFull
	}
	return opts.WaveformPartial
}
