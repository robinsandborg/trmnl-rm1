//go:build linux

package trmnl

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	"math"
	"strconv"
	"strings"
)

func renderImage(cfg Config, imageBytes []byte, outputPath string, mode RefreshMode) error {
	prepared, err := prepareLandscapeImage(cfg, imageBytes)
	if err != nil {
		return err
	}
	final := applyImageRotation(prepared, cfg)
	if err := writePNG(outputPath, final); err != nil {
		return err
	}
	if len(cfg.RendererCommand) > 0 {
		return runCommand(expandRendererCommand(cfg.RendererCommand, outputPath, mode))
	}
	return renderWithFBInk(cfg, outputPath, mode)
}

// applyImageRotation rotates the prepared landscape image into the
// framebuffer's native orientation. RM1's panel is 1404x1872 portrait; if
// fbdepth can't rotate the framebuffer (as on RM1), we rotate the image
// ourselves by fbink_rotation quarter-turns clockwise.
func applyImageRotation(src image.Image, cfg Config) image.Image {
	if !cfg.FBInkSkipRotation {
		return src
	}
	n := ((cfg.fbinkRotation() % 4) + 4) % 4
	out := src
	for i := 0; i < n; i++ {
		out = rotate90CW(out)
	}
	return out
}

func prepareLandscapeImage(cfg Config, imageBytes []byte) (*image.Gray, error) {
	src, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	if bounds.Dx() < bounds.Dy() {
		src = rotate90CW(src)
		bounds = src.Bounds()
	}

	targetW, targetH := cfg.displayWidth(), cfg.displayHeight()
	scale := math.Max(float64(targetW)/float64(bounds.Dx()), float64(targetH)/float64(bounds.Dy()))
	scaledW := float64(bounds.Dx()) * scale
	scaledH := float64(bounds.Dy()) * scale
	offsetX := (scaledW - float64(targetW)) / 2
	offsetY := (scaledH - float64(targetH)) / 2

	dst := image.NewGray(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			srcX := (float64(x) + offsetX) / scale
			srcY := (float64(y) + offsetY) / scale
			dst.SetGray(x, y, color.Gray{Y: grayscaleAt(src, int(srcX), int(srcY))})
		}
	}
	return dst, nil
}

func rotate90CW(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Max.Y-y-1, x-b.Min.X, src.At(x, y))
		}
	}
	return dst
}

func grayscaleAt(img image.Image, x, y int) uint8 {
	b := img.Bounds()
	if x < b.Min.X {
		x = b.Min.X
	}
	if y < b.Min.Y {
		y = b.Min.Y
	}
	if x >= b.Max.X {
		x = b.Max.X - 1
	}
	if y >= b.Max.Y {
		y = b.Max.Y - 1
	}
	r, g, b2, _ := img.At(x, y).RGBA()
	lum := (299*r + 587*g + 114*b2 + 500) / 1000
	return uint8(lum >> 8)
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

func renderWithFBInk(cfg Config, imagePath string, mode RefreshMode) error {
	if err := prepareFBInkFramebuffer(cfg); err != nil {
		return fmt.Errorf("prepare FBInk framebuffer: %w", err)
	}

	args := []string{
		cfg.fbinkBinary(),
		"--image",
		"file=" + imagePath + ",x=0,y=0",
		"--waveform",
		fbinkWaveformForMode(cfg, mode),
	}
	if cfg.FBInkDitherMode != "" {
		args = append(args, "--dither", cfg.FBInkDitherMode)
	}
	if cfg.FBInkNoViewport {
		args = append(args, "--noviewport")
	}
	if mode == RefreshFull {
		args = append(args, "--flash")
	}
	return runCommandWithEnv(args, []string{"FBINK_NO_SW_ROTA=1"})
}

func prepareFBInkFramebuffer(cfg Config) error {
	if err := runCommand([]string{
		cfg.fbdepthBinary(),
		"-d",
		strconv.Itoa(cfg.fbinkBitDepth()),
	}); err != nil {
		return err
	}
	if cfg.FBInkSkipRotation {
		return nil
	}
	if err := runCommand([]string{
		cfg.fbdepthBinary(),
		"-R",
		strconv.Itoa(cfg.fbinkRotation()),
	}); err != nil {
		if strings.Contains(err.Error(), "not supported on your device") {
			return nil
		}
		return err
	}
	return nil
}

func fbinkWaveformForMode(cfg Config, mode RefreshMode) string {
	if mode == RefreshFull {
		return cfg.fbinkFullWaveform()
	}
	return cfg.fbinkPartialWaveform()
}
