package playback

import (
	"context"
	"litepan/internal/cache"
	"litepan/internal/core/driverexec"
	"litepan/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

type crosslayerAccountGate struct{ disabled bool }

func (g *crosslayerAccountGate) Check(context.Context, int64) error {
	if g.disabled {
		return domain.Errf(domain.CodePermissionDenied)
	}
	return nil
}
func (g *crosslayerAccountGate) HandlePassiveError(context.Context, int64) error {
	panic("unexpected refresh")
}
func TestCrosslayerDisabledAccountCannotUseWarmPlaybackCache(t *testing.T) {
	calls := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(200) }))
	defer up.Close()
	d := &referenceTraceDriver{endpoint: up.URL, client: up.Client()}
	gate := &crosslayerAccountGate{}
	c := cache.NewService(cache.Options{})
	defer c.Close()
	s := NewService(driverexec.New(referenceTraceProvider{d}, gate), c)
	r := httptest.NewRequest("GET", "/play", nil)
	r.Header.Set("User-Agent", "player")
	if err := s.ServeHTTP(httptest.NewRecorder(), r, Request{AccountID: 1, FileID: "/movie.mkv"}, Intent{}); err != nil {
		t.Fatal(err)
	}
	gate.disabled = true
	w := httptest.NewRecorder()
	if err := s.ServeHTTP(w, r, Request{AccountID: 1, FileID: "/movie.mkv"}, Intent{}); err == nil {
		t.Fatal("disabled cached account allowed")
	}
	if calls != 1 || w.Header().Get("Location") != "" {
		t.Fatalf("calls=%d location=%s", calls, w.Header().Get("Location"))
	}
}
