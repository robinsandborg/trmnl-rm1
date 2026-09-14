package trmnl

import (
	"net/http"
	"time"

	"github.com/robinsandborg/rm1-trmnl/internal/byos"
)

// Keep identity resolution, effective config defaults, and the legacy return
// shape here while BYOS protocol behavior moves behind its own interface.
func fetchCyclePayload(client *http.Client, cfg Config) (TerminalResponse, []byte, string, time.Duration, error) {
	deviceID, err := resolveDeviceID(cfg)
	if err != nil {
		return TerminalResponse{}, nil, "", 0, err
	}
	payload, err := byos.Fetch(client, byos.Request{
		BaseURL:     cfg.BaseURL,
		DeviceID:    deviceID,
		AccessToken: cfg.AccessToken,
		Refresh:     refreshPolicy(cfg),
	})
	if err != nil {
		return TerminalResponse{}, nil, "", 0, err
	}
	return TerminalResponse{
		ImageURL:    payload.Terminal.ImageURL,
		Filename:    payload.Terminal.Filename,
		RefreshRate: payload.Terminal.RefreshRate,
	}, payload.Image, payload.ResolvedImageURL, payload.Interval, nil
}

func refreshPolicy(cfg Config) byos.RefreshPolicy {
	return byos.RefreshPolicy{
		Fallback: cfg.refreshFallback(),
		Min:      cfg.refreshMin(),
		Max:      cfg.refreshMax(),
	}
}

func clampRefresh(refreshRate int, cfg Config) time.Duration {
	return refreshPolicy(cfg).Interval(refreshRate)
}
