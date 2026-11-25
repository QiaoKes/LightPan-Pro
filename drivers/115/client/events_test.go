package pan115client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLifeBehaviorUsesActiveIteratorAndLatestPerFile(t *testing.T) {
	calls := 0
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://proapi.115.com/android/behavior/detail?app_ver=36.2.28&date=2026-11-25&limit=1000&offset=0&type=" {
			t.Fatalf("unexpected wire %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"state":true,"data":{"count":4,"list":[{"id":4,"file_id":"a","type":3,"update_time":100},{"id":3,"file_id":"a","type":2,"update_time":99},{"id":2,"file_id":"b","type":2,"update_time":98},{"id":1,"file_id":"c","type":2,"update_time":50}]}}`)), Header: make(http.Header)}, nil
	})}, "UID=1")
	events, err := c.LifeBehaviorOnce(context.Background(), "2026-11-25", 70)
	if err != nil || len(events) != 1 || string(events[0]["id"]) != "2" || calls != 1 {
		t.Fatalf("events=%v error=%v calls=%d", events, err, calls)
	}
}
func TestSubtitleWirePreservesEnvelope(t *testing.T) {
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://webapi.115.com/movies/subtitle?app_ver=36.2.28&pickcode=code" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"state":true,"data":{"list":[]}}`)), Header: make(http.Header)}, nil
	})}, "UID=1")
	if _, err := c.VideoSubtitles(context.Background(), "code"); err != nil {
		t.Fatal(err)
	}
}

func TestAttachmentKnownPickcodeSkipsSkim(t *testing.T) {
	calls := 0
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if strings.Contains(r.URL.Path, "/files/file") {
			t.Error("known pickcode repeated skim")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"state":false,"errno":99}`))}, nil
	})}, "UID=1")
	if _, err := c.DownloadAttachmentPickcode(context.Background(), "abc"); KindOf(err) != AuthExpired || calls != 1 {
		t.Fatal(calls, err)
	}
}
