// Package byos fetches a display payload and its image from a BYOS server.
package byos

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Request contains only the configuration used by the BYOS exchange.
// DeviceID must already be resolved by the caller. Refresh contains the
// caller's effective bounds and fallback; this package supplies no defaults.
type Request struct {
	BaseURL     string
	DeviceID    string
	AccessToken string
	Refresh     RefreshPolicy
}

// Payload keeps both the response's original image URL and its resolved
// download URL. Image is the unmodified response body, not a decoded image.
type Payload struct {
	Terminal         TerminalResponse
	Image            []byte
	ResolvedImageURL string
	Interval         time.Duration
}

// TerminalResponse is the BYOS wire response. Its name is retained because
// encoding/json includes it in type-error messages recorded by the appliance.
type TerminalResponse struct {
	ImageURL    string `json:"image_url"`
	Filename    string `json:"filename"`
	RefreshRate int    `json:"refresh_rate"`
}

// Fetch uses the supplied client's transport, timeout, and redirect policy.
// Only the display request gets identity/token headers. Bodies are closed
// before return, and any failure returns a zero Payload with the original
// error semantics. Response status must be 200 for both requests.
func Fetch(client *http.Client, request Request) (Payload, error) {
	apiURL := strings.TrimRight(request.BaseURL, "/") + "/api/display"
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return Payload{}, err
	}
	req.Header.Set("ID", request.DeviceID)
	if request.AccessToken != "" {
		req.Header.Set("access-token", request.AccessToken)
	}
	req.Header.Set("User-Agent", "trmnl-rm1/0.1.0")

	resp, err := client.Do(req)
	if err != nil {
		return Payload{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Payload{}, fmt.Errorf("display endpoint returned %s", resp.Status)
	}

	var terminal TerminalResponse
	if err := json.NewDecoder(resp.Body).Decode(&terminal); err != nil {
		return Payload{}, fmt.Errorf("parse display response: %w", err)
	}

	if terminal.ImageURL == "" {
		return Payload{}, errors.New("display response missing image_url")
	}

	imageURL := terminal.ImageURL
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		base, err := url.Parse(request.BaseURL)
		if err != nil {
			return Payload{}, err
		}
		ref, err := url.Parse(imageURL)
		if err != nil {
			return Payload{}, err
		}
		imageURL = base.ResolveReference(ref).String()
	}

	imgResp, err := client.Get(imageURL)
	if err != nil {
		return Payload{}, fmt.Errorf("download image: %w", err)
	}
	defer imgResp.Body.Close()
	if imgResp.StatusCode != http.StatusOK {
		return Payload{}, fmt.Errorf("image download returned %s", imgResp.Status)
	}
	imageBytes, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return Payload{}, err
	}

	interval := request.Refresh.Interval(terminal.RefreshRate)
	return Payload{
		Terminal:         terminal,
		Image:            imageBytes,
		ResolvedImageURL: imageURL,
		Interval:         interval,
	}, nil
}
