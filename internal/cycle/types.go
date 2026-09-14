package cycle

import (
	"net/http"
	"os"
	"time"

	"github.com/robinsandborg/rm1-trmnl/internal/display"
	"github.com/robinsandborg/rm1-trmnl/internal/power"
)

type Options struct {
	DownloadedImage, LastRenderedImage string
	FullRefreshEvery, FailureThreshold int
	RefreshFallback                    time.Duration
}
type Operations struct {
	Now            func() time.Time
	ReadBattery    func() (*power.BatterySample, error)
	DetermineMode  func(State, time.Time) (power.Mode, error)
	PrepareNetwork func() (*http.Client, func(), error)
	FetchPayload   func(*http.Client) (string, []byte, string, time.Duration, error)
	Render         func([]byte, string, display.RefreshMode) error
	Plan           func(time.Duration, power.Mode) (power.Mode, error)
	Suspend        func() error
	WriteFile      func(string, []byte, os.FileMode) error
	AppendLog      func(CycleLog) error
	SaveState      func(State) error
}
type Error struct {
	Category string
	Err      error
}

func (e *Error) Error() string { return e.Err.Error() }

type State struct {
	MaskedNoise         map[string]bool `json:"masked_noise,omitempty"`
	LastImageHash       string          `json:"last_image_hash,omitempty"`
	LastImageURL        string          `json:"last_image_url,omitempty"`
	LastFilename        string          `json:"last_filename,omitempty"`
	RenderedUpdates     int             `json:"rendered_updates"`
	ConsecutiveFailures int             `json:"consecutive_failures"`
	LastSuccessAt       time.Time       `json:"last_success_at,omitempty"`
	LastFailureAt       time.Time       `json:"last_failure_at,omitempty"`
	LastFailureCategory string          `json:"last_failure_category,omitempty"`
	LastFailureMessage  string          `json:"last_failure_message,omitempty"`
	LastIntervalSeconds int             `json:"last_interval_seconds,omitempty"`
	LastCycleChanged    bool            `json:"last_cycle_changed"`
	LastMode            string          `json:"last_mode,omitempty"`
	StockSyncUnit       string          `json:"stock_sync_unit,omitempty"`
	XochitlWasEnabled   bool            `json:"xochitl_was_enabled,omitempty"`
	SyncWasEnabled      bool            `json:"sync_was_enabled,omitempty"`
}

type CycleLog struct {
	StartedAt         time.Time            `json:"started_at"`
	EndedAt           time.Time            `json:"ended_at"`
	Mode              string               `json:"mode"`
	ChangedScreen     bool                 `json:"changed_screen"`
	ImageHash         string               `json:"image_hash,omitempty"`
	ImageURL          string               `json:"image_url,omitempty"`
	RefreshIntervalS  int                  `json:"refresh_interval_seconds"`
	FailureCategory   string               `json:"failure_category,omitempty"`
	FailureMessage    string               `json:"failure_message,omitempty"`
	Battery           *power.BatterySample `json:"battery,omitempty"`
	SkippedRender     bool                 `json:"skipped_render"`
	FullRefresh       bool                 `json:"full_refresh"`
	ConsecutiveFails  int                  `json:"consecutive_failures"`
	MaintenanceReason string               `json:"maintenance_reason,omitempty"`
}
