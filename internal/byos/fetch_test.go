package byos_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/robinsandborg/rm1-trmnl/internal/byos"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type responseBody struct {
	io.Reader
	name   string
	closed *[]string
}

func (b responseBody) Close() error {
	*b.closed = append(*b.closed, b.name)
	return errors.New("ignored close error")
}

func response(status int, body io.Reader, name string, closed *[]string) *http.Response {
	return &http.Response{
		StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header: make(http.Header), Body: responseBody{body, name, closed},
	}
}

func request() byos.Request {
	return byos.Request{
		BaseURL: "http://byos.test", DeviceID: "AA:BB:CC:DD:EE:FF", AccessToken: "test-token",
		Refresh: byos.RefreshPolicy{Fallback: 30 * time.Minute, Min: 5 * time.Minute, Max: time.Hour},
	}
}

func TestFetchPreservesPayloadAndRequests(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, base, image, endpoint, resolved string }{
		{"root relative", "http://byos.test", "/images/current.png", "http://byos.test/api/display", "http://byos.test/images/current.png"},
		{"path relative", "http://byos.test/screens/", "current.png", "http://byos.test/screens/api/display", "http://byos.test/screens/current.png"},
		{"base without trailing slash", "http://byos.test/screens", "current.png", "http://byos.test/screens/api/display", "http://byos.test/current.png"},
		{"repeated trailing slash", "http://byos.test///", "/current.png", "http://byos.test/api/display", "http://byos.test/current.png"},
		{"absolute image", "http://byos.test", "https://cdn.test/frame.bmp?version=2", "http://byos.test/api/display", "https://cdn.test/frame.bmp?version=2"},
		{"scheme relative", "https://byos.test/", "//cdn.test/current.png", "https://byos.test/api/display", "https://cdn.test/current.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := request()
			opts.BaseURL = tc.base
			// Arbitrary bytes are passed through; decoding belongs to rendering.
			image := []byte{0, 255, 1, 2, 3}
			data, err := json.Marshal(map[string]any{"image_url": tc.image, "filename": "screen.bmp", "refresh_rate": 900, "future_field": true})
			if err != nil {
				t.Fatal(err)
			}
			var calls, closed []string
			client := &http.Client{Timeout: 7 * time.Second, Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls = append(calls, r.URL.String())
				if r.Method != http.MethodGet {
					t.Fatal(r.Method)
				}
				deadline, ok := r.Context().Deadline()
				if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 7*time.Second {
					t.Fatal("caller timeout not applied")
				}
				if len(calls) == 1 {
					if r.URL.String() != tc.endpoint || r.Header.Get("ID") != opts.DeviceID || r.Header.Get("access-token") != opts.AccessToken || r.Header.Get("User-Agent") != "trmnl-rm1/0.1.0" {
						t.Fatalf("display request=%s %v", r.URL, r.Header)
					}
					return response(200, bytes.NewReader(data), "display", &closed), nil
				}
				if len(calls) != 2 || r.URL.String() != tc.resolved {
					t.Fatalf("unexpected image URL %s", r.URL)
				}
				if len(closed) != 0 {
					t.Fatalf("display body closed early: %v", closed)
				}
				if r.Header.Get("ID") != "" || r.Header.Get("access-token") != "" || r.Header.Get("User-Agent") != "" {
					t.Fatalf("image headers=%v", r.Header)
				}
				return response(200, bytes.NewReader(image), "image", &closed), nil
			})}
			got, err := byos.Fetch(client, opts)
			want := byos.Payload{Terminal: byos.TerminalResponse{ImageURL: tc.image, Filename: "screen.bmp", RefreshRate: 900}, Image: image, ResolvedImageURL: tc.resolved, Interval: 15 * time.Minute}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("payload=%+v, error=%v; want %+v", got, err, want)
			}
			if len(calls) != 2 || !reflect.DeepEqual(closed, []string{"image", "display"}) {
				t.Fatalf("calls=%v closed=%v", calls, closed)
			}
		})
	}
}

func TestFetchAcceptsExistingJSONAndEmptyImageBehavior(t *testing.T) {
	for _, body := range []string{
		`{"image_url":"/image"}`,
		`{"image_url":"/image","filename":"","refresh_rate":null,"unknown":true}`,
		`{"image_url":"/image"} trailing content`,
	} {
		t.Run(body, func(t *testing.T) {
			opts := request()
			opts.AccessToken = ""
			var closed []string
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if _, ok := r.Header["Access-Token"]; ok {
					t.Fatal("empty token was sent")
				}
				if r.URL.Path == "/api/display" {
					return response(200, strings.NewReader(body), "display", &closed), nil
				}
				return response(200, strings.NewReader(""), "image", &closed), nil
			})}
			got, err := byos.Fetch(client, opts)
			if err != nil || got.Image == nil || len(got.Image) != 0 || got.Terminal.Filename != "" || got.Terminal.RefreshRate != 0 || got.Interval != 30*time.Minute {
				t.Fatalf("payload=%+v error=%v", got, err)
			}
		})
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestFetchFailuresReturnNoPartialPayload(t *testing.T) {
	t.Parallel()
	displayErr, imageErr, readErr := errors.New("display transport failed"), errors.New("image transport failed"), errors.New("body read failed")
	for _, tc := range []struct {
		name, base, body, message                                               string
		displayStatus, imageStatus                                              int
		displayTransport, imageTransport, displayRead, imageRead, errorIdentity error
		wantCalls                                                               int
		wantClosed                                                              []string
	}{
		{name: "invalid base", base: "http://%zz", message: `invalid URL escape "%zz"`, wantCalls: 0},
		{name: "display transport", displayTransport: displayErr, message: "display transport failed", errorIdentity: displayErr, wantCalls: 1},
		{name: "display status", displayStatus: 503, message: "display endpoint returned 503 Service Unavailable", wantCalls: 1, wantClosed: []string{"display"}},
		{name: "malformed JSON", body: "{", message: "parse display response: unexpected EOF", errorIdentity: io.ErrUnexpectedEOF, wantCalls: 1, wantClosed: []string{"display"}},
		{name: "wrong JSON type", body: `{"image_url":7}`, message: "parse display response: json: cannot unmarshal number into Go struct field TerminalResponse.image_url of type string", wantCalls: 1, wantClosed: []string{"display"}},
		{name: "missing image", body: `{}`, message: "display response missing image_url", wantCalls: 1, wantClosed: []string{"display"}},
		{name: "null display", body: `null`, message: "display response missing image_url", wantCalls: 1, wantClosed: []string{"display"}},
		{name: "display read", displayRead: readErr, message: "parse display response: body read failed", errorIdentity: readErr, wantCalls: 1, wantClosed: []string{"display"}},
		{name: "invalid relative image", body: `{"image_url":"%zz"}`, message: `parse "%zz": invalid URL escape "%zz"`, wantCalls: 1, wantClosed: []string{"display"}},
		{name: "invalid absolute image", body: `{"image_url":"http://cdn.test/%zz"}`, message: `download image: parse "http://cdn.test/%zz": invalid URL escape "%zz"`, wantCalls: 1, wantClosed: []string{"display"}},
		{name: "image transport", imageTransport: imageErr, message: "download image:", errorIdentity: imageErr, wantCalls: 2, wantClosed: []string{"display"}},
		{name: "image status", imageStatus: 404, message: "image download returned 404 Not Found", wantCalls: 2, wantClosed: []string{"image", "display"}},
		{name: "image read", imageRead: readErr, message: "body read failed", errorIdentity: readErr, wantCalls: 2, wantClosed: []string{"image", "display"}},
		{name: "caller deadline", displayTransport: context.DeadlineExceeded, message: "context deadline exceeded", errorIdentity: context.DeadlineExceeded, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := request()
			if tc.base != "" {
				opts.BaseURL = tc.base
			}
			body := tc.body
			if body == "" {
				body = `{"image_url":"/image","filename":"partially-parsed.png","refresh_rate":900}`
			}
			calls := 0
			var closed []string
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				status := 200
				if calls == 1 {
					if tc.displayTransport != nil {
						return nil, tc.displayTransport
					}
					if tc.displayStatus != 0 {
						status = tc.displayStatus
					}
					var reader io.Reader = strings.NewReader(body)
					if tc.displayRead != nil {
						reader = errorReader{tc.displayRead}
					}
					return response(status, reader, "display", &closed), nil
				}
				if tc.imageTransport != nil {
					return nil, tc.imageTransport
				}
				if tc.imageStatus != 0 {
					status = tc.imageStatus
				}
				var reader io.Reader = strings.NewReader("image bytes")
				if tc.imageRead != nil {
					reader = errorReader{tc.imageRead}
				}
				return response(status, reader, "image", &closed), nil
			})}
			got, err := byos.Fetch(client, opts)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error=%v; want %q", err, tc.message)
			}
			if tc.errorIdentity != nil && !errors.Is(err, tc.errorIdentity) {
				t.Fatalf("error lost identity: %v", err)
			}
			if !reflect.DeepEqual(got, byos.Payload{}) || calls != tc.wantCalls || !reflect.DeepEqual(closed, tc.wantClosed) {
				t.Fatalf("payload=%+v calls=%d closed=%v", got, calls, closed)
			}
		})
	}
}

func TestFetchUsesCallerRedirectPolicy(t *testing.T) {
	var closed []string
	redirects := 0
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { redirects++; return http.ErrUseLastResponse },
		Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			resp := response(302, strings.NewReader(""), "redirect", &closed)
			resp.Header.Set("Location", "http://redirect.test/display")
			return resp, nil
		}),
	}
	_, err := byos.Fetch(client, request())
	if err == nil || err.Error() != "display endpoint returned 302 Found" || redirects != 1 || !reflect.DeepEqual(closed, []string{"redirect"}) {
		t.Fatalf("error=%v redirects=%d closed=%v", err, redirects, closed)
	}
}
