package trmnl

import (
	"bytes"
	"encoding/json"
	"net/http"
	"runtime"
	"testing"
	"time"
)

// These wrapper cases also run against the pre-extraction implementation
// during validation, alongside the unchanged Phase 1 cycle fixtures.
func TestFetchCyclePayloadFacadeContract(t *testing.T) {
	for _, tc := range []struct {
		name string
		rate int
		cfg  Config
		want time.Duration
	}{
		{"effective defaults", 0, Config{}, 30 * time.Minute},
		{"default minimum", 1, Config{}, 5 * time.Minute},
		{"default maximum", 90000, Config{}, 24 * time.Hour},
		{"explicit policy", 100, Config{RefreshMinSeconds: 60, RefreshMaxSeconds: 120, RefreshFallbackSeconds: 90}, 100 * time.Second},
		{"explicit fallback", -1, Config{RefreshMinSeconds: 60, RefreshMaxSeconds: 120, RefreshFallbackSeconds: 90}, 90 * time.Second},
		{"nonpositive config fallback", 0, Config{RefreshMinSeconds: -1, RefreshMaxSeconds: -1, RefreshFallbackSeconds: -1}, 30 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			cfg.BaseURL = "http://byos.test/screens/"
			cfg.DeviceID = "  explicit  "
			wantID := cfg.DeviceID
			if runtime.GOOS == "linux" {
				wantID = "explicit"
			}
			body, err := json.Marshal(TerminalResponse{ImageURL: "current.png", Filename: "screen.png", RefreshRate: tc.rate})
			if err != nil {
				t.Fatal(err)
			}
			image := []byte{0, 1, 255, 7}
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					if r.URL.String() != "http://byos.test/screens/api/display" || r.Header.Get("ID") != wantID {
						t.Fatalf("request=%s %v", r.URL, r.Header)
					}
					return jsonResponse(string(body)), nil
				}
				if calls != 2 || r.URL.String() != "http://byos.test/screens/current.png" {
					t.Fatalf("request=%s", r.URL)
				}
				return binaryResponse("image/png", image), nil
			})}
			terminal, data, resolved, interval, err := fetchCyclePayload(client, cfg)
			wantTerminal := TerminalResponse{ImageURL: "current.png", Filename: "screen.png", RefreshRate: tc.rate}
			if err != nil || terminal != wantTerminal || !bytes.Equal(data, image) || resolved != "http://byos.test/screens/current.png" || interval != tc.want || calls != 2 {
				t.Fatalf("terminal=%+v image=%v resolved=%s interval=%s calls=%d error=%v", terminal, data, resolved, interval, calls, err)
			}
		})
	}
}

func TestFetchCyclePayloadFacadeDropsPartialResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/display" {
			return jsonResponse(`{"image_url":"/image","filename":"parsed.png","refresh_rate":900}`), nil
		}
		resp := binaryResponse("text/plain", []byte("missing"))
		resp.StatusCode, resp.Status = 404, "404 Not Found"
		return resp, nil
	})}
	terminal, data, resolved, interval, err := fetchCyclePayload(client, Config{BaseURL: "http://byos.test", DeviceID: "explicit"})
	if err == nil || err.Error() != "image download returned 404 Not Found" || terminal != (TerminalResponse{}) || data != nil || resolved != "" || interval != 0 {
		t.Fatalf("partial response leaked: %+v %v %s %s %v", terminal, data, resolved, interval, err)
	}
}

func TestFetchCyclePayloadPreservesJSONTypeError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"image_url":7}`), nil
	})}
	_, _, _, _, err := fetchCyclePayload(client, Config{BaseURL: "http://byos.test", DeviceID: "explicit"})
	want := "parse display response: json: cannot unmarshal number into Go struct field TerminalResponse.image_url of type string"
	if err == nil || err.Error() != want {
		t.Fatalf("error=%v; want %s", err, want)
	}
}
