// Package display prepares appliance images and drives the platform renderer.
package display

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"

	_ "golang.org/x/image/bmp"
)

// Prepare decodes, crops/scales to grayscale landscape, then applies software
// rotation when requested. It performs no file or device operations.
func Prepare(opts Options, imageBytes []byte) (image.Image, error) {
	prepared, err := prepareLandscapeImage(opts, imageBytes)
	if err != nil {
		return nil, err
	}
	return applyImageRotation(prepared, opts), nil
}

// applyImageRotation rotates the prepared landscape image into the
// framebuffer's native orientation. RM1's panel is 1404x1872 portrait; if
// fbdepth can't rotate the framebuffer (as on RM1), we rotate the image
// ourselves by fbink_rotation quarter-turns clockwise.
func applyImageRotation(src image.Image, opts Options) image.Image {
	if !opts.SkipRotation {
		return src
	}
	n := ((opts.Rotation % 4) + 4) % 4
	out := src
	for i := 0; i < n; i++ {
		out = rotate90CW(out)
	}
	return out
}

func prepareLandscapeImage(opts Options, imageBytes []byte) (*image.Gray, error) {
	src, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	if bounds.Dx() < bounds.Dy() {
		src = rotate90CW(src)
		bounds = src.Bounds()
	}

	targetW, targetH := opts.Width, opts.Height
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
