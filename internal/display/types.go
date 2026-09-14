package display

// Options contains effective rendering values; the application resolves
// configuration defaults. Width and Height must be positive.
type Options struct {
	Width           int
	Height          int
	Rotation        int
	SkipRotation    bool
	RawRotation     bool
	RendererCommand []string
	FBInkBinary     string
	FBDepthBinary   string
	BitDepth        int
	WaveformPartial string
	WaveformFull    string
	DitherMode      string
	NoViewport      bool
}

type RefreshMode string

const (
	RefreshPartial RefreshMode = "partial"
	RefreshFull    RefreshMode = "full"
)

// CommandRunner executes argv with env appended to the inherited environment.
type CommandRunner func(argv []string, env []string) error
