package trmnl

import "time"

const (
	defaultBaseURL              = "http://larapaper.local"
	defaultRefreshFallback      = 30 * time.Minute
	defaultRefreshMin           = 5 * time.Minute
	defaultRefreshMax           = 24 * time.Hour
	defaultWiFiTimeout          = 45 * time.Second
	defaultFullRefreshEvery     = 6
	defaultBootGrace            = 10 * time.Minute
	defaultFailureThreshold     = 3
	defaultWiFiInterface        = "wlan0"
	defaultMaintenanceIface     = "usb0"
	defaultFramebufferDevice    = "/dev/fb0"
	defaultRTCWakealarmPath     = "/sys/class/rtc/rtc0/wakealarm"
	defaultPowerSupplyPath      = "/sys/class/power_supply"
	defaultDisplayWidth         = 1872
	defaultDisplayHeight        = 1404
	defaultFBInkBinary          = "fbink"
	defaultFBDepthBinary        = "fbdepth"
	defaultFBInkBitDepth        = 8
	defaultFBInkRotation        = 3
	defaultFBInkPartialWaveform = "GL16"
	defaultFBInkFullWaveform    = "GC16"
	defaultStateFilename        = "state.json"
	defaultLogFilename          = "cycles.log"
	defaultRenderedImageName    = "current.png"
	defaultDownloadedImageName  = "downloaded.png"
)

type Config struct {
	BaseURL                   string        `json:"base_url"`
	DeviceID                  string        `json:"device_id,omitempty"`
	AccessToken               string        `json:"access_token,omitempty"`
	RefreshFallbackSeconds    int           `json:"refresh_fallback_seconds,omitempty"`
	RefreshMinSeconds         int           `json:"refresh_min_seconds,omitempty"`
	RefreshMaxSeconds         int           `json:"refresh_max_seconds,omitempty"`
	WiFiTimeoutSeconds        int           `json:"wifi_timeout_seconds,omitempty"`
	DisableWiFiBetweenUpdates bool          `json:"disable_wifi_between_updates,omitempty"`
	FullRefreshEvery          int           `json:"full_refresh_every,omitempty"`
	BootGraceSeconds          int           `json:"boot_grace_seconds,omitempty"`
	FailureThreshold          int           `json:"failure_threshold,omitempty"`
	WiFiInterface             string        `json:"wifi_interface,omitempty"`
	MaintenanceInterface      string        `json:"maintenance_interface,omitempty"`
	FramebufferDevice         string        `json:"framebuffer_device,omitempty"`
	RTCWakealarmPath          string        `json:"rtc_wakealarm_path,omitempty"`
	PowerSupplyPath           string        `json:"power_supply_path,omitempty"`
	DisplayWidth              int           `json:"display_width,omitempty"`
	DisplayHeight             int           `json:"display_height,omitempty"`
	RendererCommand           []string      `json:"renderer_command,omitempty"`
	WiFiUpCommand             []string      `json:"wifi_up_command,omitempty"`
	WiFiDownCommand           []string      `json:"wifi_down_command,omitempty"`
	SuspendCommand            []string      `json:"suspend_command,omitempty"`
	ConnectivityCheckURL      string        `json:"connectivity_check_url,omitempty"`
	FBInkBinary               string        `json:"fbink_binary,omitempty"`
	FBDepthBinary             string        `json:"fbdepth_binary,omitempty"`
	FBInkBitDepth             int           `json:"fbink_bit_depth,omitempty"`
	FBInkRotation             int           `json:"fbink_rotation,omitempty"`
	FBInkWaveformPartial      string        `json:"fbink_waveform_partial,omitempty"`
	FBInkWaveformFull         string        `json:"fbink_waveform_full,omitempty"`
	FBInkDitherMode           string        `json:"fbink_dither_mode,omitempty"`
	FBInkNoViewport           bool          `json:"fbink_no_viewport,omitempty"`
	DisplayPower              *DisplayPower `json:"display_power,omitempty"`
}

type DisplayPower struct {
	FullRefreshEvery int `json:"full_refresh_every,omitempty"`
}

type State struct {
	LastImageHash       string    `json:"last_image_hash,omitempty"`
	LastImageURL        string    `json:"last_image_url,omitempty"`
	LastFilename        string    `json:"last_filename,omitempty"`
	RenderedUpdates     int       `json:"rendered_updates"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastSuccessAt       time.Time `json:"last_success_at,omitempty"`
	LastFailureAt       time.Time `json:"last_failure_at,omitempty"`
	LastFailureCategory string    `json:"last_failure_category,omitempty"`
	LastFailureMessage  string    `json:"last_failure_message,omitempty"`
	LastIntervalSeconds int       `json:"last_interval_seconds,omitempty"`
	LastCycleChanged    bool      `json:"last_cycle_changed"`
	LastMode            string    `json:"last_mode,omitempty"`
	StockSyncUnit       string    `json:"stock_sync_unit,omitempty"`
	XochitlWasEnabled   bool      `json:"xochitl_was_enabled,omitempty"`
	SyncWasEnabled      bool      `json:"sync_was_enabled,omitempty"`
}

type TerminalResponse struct {
	ImageURL    string `json:"image_url"`
	Filename    string `json:"filename"`
	RefreshRate int    `json:"refresh_rate"`
}

type BatterySample struct {
	Status         string `json:"status,omitempty"`
	CapacityPct    string `json:"capacity_pct,omitempty"`
	VoltageMicroV  string `json:"voltage_micro_v,omitempty"`
	CurrentMicroA  string `json:"current_micro_a,omitempty"`
	TemperatureDec string `json:"temperature_decic,omitempty"`
}

type CycleLog struct {
	StartedAt         time.Time      `json:"started_at"`
	EndedAt           time.Time      `json:"ended_at"`
	Mode              string         `json:"mode"`
	ChangedScreen     bool           `json:"changed_screen"`
	ImageHash         string         `json:"image_hash,omitempty"`
	ImageURL          string         `json:"image_url,omitempty"`
	RefreshIntervalS  int            `json:"refresh_interval_seconds"`
	FailureCategory   string         `json:"failure_category,omitempty"`
	FailureMessage    string         `json:"failure_message,omitempty"`
	Battery           *BatterySample `json:"battery,omitempty"`
	SkippedRender     bool           `json:"skipped_render"`
	FullRefresh       bool           `json:"full_refresh"`
	ConsecutiveFails  int            `json:"consecutive_failures"`
	MaintenanceReason string         `json:"maintenance_reason,omitempty"`
}

type RuntimeMode struct {
	Name              string
	MaintenanceReason string
	ShouldSuspend     bool
}

type RefreshMode string

const (
	RefreshPartial RefreshMode = "partial"
	RefreshFull    RefreshMode = "full"
)

type cycleError struct {
	Category string
	Err      error
}

func (e *cycleError) Error() string {
	return e.Err.Error()
}

func (c Config) refreshFallback() time.Duration {
	if c.RefreshFallbackSeconds > 0 {
		return time.Duration(c.RefreshFallbackSeconds) * time.Second
	}
	return defaultRefreshFallback
}

func (c Config) refreshMin() time.Duration {
	if c.RefreshMinSeconds > 0 {
		return time.Duration(c.RefreshMinSeconds) * time.Second
	}
	return defaultRefreshMin
}

func (c Config) refreshMax() time.Duration {
	if c.RefreshMaxSeconds > 0 {
		return time.Duration(c.RefreshMaxSeconds) * time.Second
	}
	return defaultRefreshMax
}

func (c Config) wifiTimeout() time.Duration {
	if c.WiFiTimeoutSeconds > 0 {
		return time.Duration(c.WiFiTimeoutSeconds) * time.Second
	}
	return defaultWiFiTimeout
}

func (c Config) fullRefreshEvery() int {
	if c.DisplayPower != nil && c.DisplayPower.FullRefreshEvery > 0 {
		return c.DisplayPower.FullRefreshEvery
	}
	if c.FullRefreshEvery > 0 {
		return c.FullRefreshEvery
	}
	return defaultFullRefreshEvery
}

func (c Config) bootGrace() time.Duration {
	if c.BootGraceSeconds > 0 {
		return time.Duration(c.BootGraceSeconds) * time.Second
	}
	return defaultBootGrace
}

func (c Config) failureThreshold() int {
	if c.FailureThreshold > 0 {
		return c.FailureThreshold
	}
	return defaultFailureThreshold
}

func (c Config) wifiInterface() string {
	if c.WiFiInterface != "" {
		return c.WiFiInterface
	}
	return defaultWiFiInterface
}

func (c Config) maintenanceInterface() string {
	if c.MaintenanceInterface != "" {
		return c.MaintenanceInterface
	}
	return defaultMaintenanceIface
}

func (c Config) framebufferDevice() string {
	if c.FramebufferDevice != "" {
		return c.FramebufferDevice
	}
	return defaultFramebufferDevice
}

func (c Config) rtcWakealarmPath() string {
	if c.RTCWakealarmPath != "" {
		return c.RTCWakealarmPath
	}
	return defaultRTCWakealarmPath
}

func (c Config) powerSupplyPath() string {
	if c.PowerSupplyPath != "" {
		return c.PowerSupplyPath
	}
	return defaultPowerSupplyPath
}

func (c Config) displayWidth() int {
	if c.DisplayWidth > 0 {
		return c.DisplayWidth
	}
	return defaultDisplayWidth
}

func (c Config) displayHeight() int {
	if c.DisplayHeight > 0 {
		return c.DisplayHeight
	}
	return defaultDisplayHeight
}

func (c Config) fbinkBinary() string {
	if c.FBInkBinary != "" {
		return c.FBInkBinary
	}
	return defaultFBInkBinary
}

func (c Config) fbdepthBinary() string {
	if c.FBDepthBinary != "" {
		return c.FBDepthBinary
	}
	return defaultFBDepthBinary
}

func (c Config) fbinkBitDepth() int {
	if c.FBInkBitDepth > 0 {
		return c.FBInkBitDepth
	}
	return defaultFBInkBitDepth
}

func (c Config) fbinkRotation() int {
	if c.FBInkRotation > 0 {
		return c.FBInkRotation
	}
	return defaultFBInkRotation
}

func (c Config) fbinkPartialWaveform() string {
	if c.FBInkWaveformPartial != "" {
		return c.FBInkWaveformPartial
	}
	return defaultFBInkPartialWaveform
}

func (c Config) fbinkFullWaveform() string {
	if c.FBInkWaveformFull != "" {
		return c.FBInkWaveformFull
	}
	return defaultFBInkFullWaveform
}
