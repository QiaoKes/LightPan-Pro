package pan115client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestExportCancellationRetainsOnlyKnownTemporaryIdentity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls []string
	c := New(&http.Client{Transport: treeTransport(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		body := `{"id":"42"}`
		if r.URL.Path == "/files/export_dir" {
			if r.Method == "POST" {
				body = `{"data":"job"}`
			} else {
				body = `{"data":false}`
				cancel()
			}
		} else if r.URL.Path != "/files/getid" {
			t.Fatalf("unexpected request %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}, "cookie")
	_, complete, err := c.ExportTree(ctx, "/scope")
	var owned *TreeExportError
	if complete || !errors.Is(err, context.Canceled) || !errors.As(err, &owned) || owned.ExportID != "job" || owned.FileID != "" || len(calls) != 3 {
		t.Fatalf("complete=%v err=%v calls=%v", complete, err, calls)
	}
}

func TestExportTimeoutBoundaryDoesNotInventCleanup(t *testing.T) {
	clock := time.Unix(0, 0)
	polls, waits := 0, 0
	c := New(&http.Client{Transport: treeTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"id":"42"}`
		switch r.URL.Path {
		case "/files/getid":
		case "/files/export_dir":
			if r.Method == "POST" {
				body = `{"data":"job"}`
			} else {
				polls++
				body = `{"data":false}`
			}
		default:
			t.Fatalf("unexpected request %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}, "cookie")
	_, complete, err := c.exportTreeWithTiming(context.Background(), "/scope", func(context.Context, string, string) (*Download, error) {
		t.Fatal("unexpected resolution")
		return nil, nil
	}, func() time.Time { return clock }, func(context.Context) error {
		waits++
		if waits == 1 {
			clock = clock.Add(3000 * time.Second)
		} else {
			clock = clock.Add(time.Nanosecond)
		}
		return nil
	})
	var owned *TreeExportError
	if complete || !errors.As(err, &owned) || owned.ExportID != "job" || owned.FileID != "" || !strings.Contains(err.Error(), "tree export timeout") || polls != 2 || waits != 2 {
		t.Fatalf("complete=%v err=%v polls=%d waits=%d", complete, err, polls, waits)
	}
}
