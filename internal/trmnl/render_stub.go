//go:build !linux

package trmnl

import "errors"

func renderImage(cfg Config, imageBytes []byte, outputPath string, mode RefreshMode) error {
	return errors.New("rendering is only supported on Linux")
}
