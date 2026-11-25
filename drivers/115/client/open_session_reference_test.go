package pan115client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type openHeaderTransport func(*http.Request) (*http.Response, error)

func (f openHeaderTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOpenQRStatusFreshSessionHeaders(t *testing.T) {
	c := New(&http.Client{Transport: openHeaderTransport(func(r *http.Request) (*http.Response, error) {
		if r.UserAgent() != "EmbyNginx" || r.Header.Get("Accept") != "*/*" || r.Header.Get("Accept-Encoding") != "gzip, deflate" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.Header.Get("Cookie") != "" || r.URL.Query().Get("app_ver") != "" {
			t.Fatalf("request=%v headers=%v", r.URL, r.Header)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"state":true,"data":{"status":0}}`))}, nil
	})}, "UID=must-not-leak")
	// Open polling must never inherit Cookie credentials.
	if _, err := c.OpenQRStatus(context.Background(), "uid", "123", "sign"); err != nil {
		t.Fatal(err)
	}
}
