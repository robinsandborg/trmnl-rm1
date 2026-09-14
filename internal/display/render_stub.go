//go:build !linux

package display

import "errors"

// Render retains the appliance's non-Linux unsupported behavior without
// preparing an image, writing a file, or invoking a command.
func Render(opts Options, imageBytes []byte, outputPath string, mode RefreshMode, run CommandRunner) error {
	return errors.New("rendering is only supported on Linux")
}
