package trmnl

import "github.com/robinsandborg/rm1-trmnl/internal/display"

// Keep configuration defaults and the existing runner at the composition
// root. Platform support and rendering behavior live in the display module.
func renderImage(cfg Config, imageBytes []byte, outputPath string, mode RefreshMode) error {
	return display.Render(display.Options{
		Width:           cfg.displayWidth(),
		Height:          cfg.displayHeight(),
		Rotation:        cfg.fbinkRotation(),
		SkipRotation:    cfg.FBInkSkipRotation,
		RendererCommand: cfg.RendererCommand,
		FBInkBinary:     cfg.fbinkBinary(),
		FBDepthBinary:   cfg.fbdepthBinary(),
		BitDepth:        cfg.fbinkBitDepth(),
		WaveformPartial: cfg.fbinkPartialWaveform(),
		WaveformFull:    cfg.fbinkFullWaveform(),
		DitherMode:      cfg.FBInkDitherMode,
		NoViewport:      cfg.FBInkNoViewport,
	}, imageBytes, outputPath, display.RefreshMode(mode), runCommandWithEnv)
}
