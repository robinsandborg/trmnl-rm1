package trmnl

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClampRefreshUsesFallbackAndBounds(t *testing.T) {
	cfg := Config{
		DeviceID:               "AA:BB:CC:DD:EE:FF",
		RefreshFallbackSeconds: 1800,
		RefreshMinSeconds:      300,
		RefreshMaxSeconds:      3600,
	}

	if got := clampRefresh(0, cfg); got != 30*time.Minute {
		t.Fatalf("fallback refresh = %s, want 30m", got)
	}
	if got := clampRefresh(30, cfg); got != 5*time.Minute {
		t.Fatalf("min clamp = %s, want 5m", got)
	}
	if got := clampRefresh(7200, cfg); got != time.Hour {
		t.Fatalf("max clamp = %s, want 1h", got)
	}
}

func TestShouldUseFullRefresh(t *testing.T) {
	if shouldUseFullRefresh(0, 6) {
		t.Fatal("first render should not force full refresh")
	}
	if !shouldUseFullRefresh(5, 6) {
		t.Fatal("sixth render should force full refresh")
	}
}

func TestFetchCyclePayloadUsesBYOSHeadersAndResolvesRelativeImageURL(t *testing.T) {
	pngBytes := makeTestPNG(t)
	var gotID string
	var gotAccessToken string
	var gotBatteryVoltage string

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.String() {
			case "http://larapaper.test/api/display":
				gotID = r.Header.Get("ID")
				gotAccessToken = r.Header.Get("access-token")
				gotBatteryVoltage = r.Header.Get("Battery-Voltage")
				return jsonResponse(`{"image_url":"/images/current.png","filename":"screen.png","refresh_rate":900}`), nil
			case "http://larapaper.test/images/current.png":
				return binaryResponse("image/png", pngBytes), nil
			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Status:     "404 Not Found",
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     make(http.Header),
				}, nil
			}
		}),
	}

	cfg := Config{
		BaseURL:     "http://larapaper.test",
		DeviceID:    "AA:BB:CC:DD:EE:FF",
		AccessToken: "secret-token",
	}
	battery := &BatterySample{VoltageMicroV: "3950000"}

	terminal, imageBytes, imageURL, interval, err := fetchCyclePayload(client, cfg, battery)
	if err != nil {
		t.Fatalf("fetchCyclePayload error = %v", err)
	}

	if gotID != cfg.DeviceID {
		t.Fatalf("ID header = %q, want %q", gotID, cfg.DeviceID)
	}
	if gotAccessToken != cfg.AccessToken {
		t.Fatalf("access-token header = %q, want %q", gotAccessToken, cfg.AccessToken)
	}
	if gotBatteryVoltage != "3.95" {
		t.Fatalf("Battery-Voltage header = %q, want %q", gotBatteryVoltage, "3.95")
	}
	if terminal.Filename != "screen.png" {
		t.Fatalf("filename = %q", terminal.Filename)
	}
	if imageURL != "http://larapaper.test/images/current.png" {
		t.Fatalf("resolved image URL = %q", imageURL)
	}
	if len(imageBytes) == 0 {
		t.Fatal("expected image bytes")
	}
	if interval != 15*time.Minute {
		t.Fatalf("interval = %s, want 15m", interval)
	}
}

func TestBatteryVoltageHeader(t *testing.T) {
	tests := []struct {
		name   string
		sample *BatterySample
		want   string
	}{
		{"nil sample", nil, ""},
		{"empty voltage", &BatterySample{}, ""},
		{"invalid string", &BatterySample{VoltageMicroV: "abc"}, ""},
		{"zero voltage", &BatterySample{VoltageMicroV: "0"}, ""},
		{"negative voltage", &BatterySample{VoltageMicroV: "-1000000"}, ""},
		{"typical full charge", &BatterySample{VoltageMicroV: "4200000"}, "4.20"},
		{"typical mid charge", &BatterySample{VoltageMicroV: "3950000"}, "3.95"},
		{"typical low charge", &BatterySample{VoltageMicroV: "3500000"}, "3.50"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := batteryVoltageHeader(tc.sample); got != tc.want {
				t.Fatalf("batteryVoltageHeader = %q, want %q", got, tc.want)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func binaryResponse(contentType string, body []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{contentType}},
	}
}

func makeTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.Gray{Y: uint8(x * 64)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}
