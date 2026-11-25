package pan115client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestSubtitleSuffixOrderAndDirectTransfer(t *testing.T) {
	stablePoints.Store("subtitle-fixture", "0123")
	var mu sync.Mutex
	var suffixes []string
	direct := 0
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body []byte
		switch r.URL.Path {
		case "/os_windows/2.0/ufile/downfolders":
			body = encryptedFixture(`{"state":true,"data":{"list":[],"has_next_page":false}}`)
		case "/android/2.0/ufile/files":
			suffix := r.URL.Query().Get("suffix")
			mu.Lock()
			suffixes = append(suffixes, suffix)
			mu.Unlock()
			if r.URL.Query().Has("type") || r.URL.Query().Get("limit") != "7000" || r.URL.Query().Get("cur") != "0" || r.URL.Query().Get("count_folders") != "0" {
				return nil, fmt.Errorf("bad subtitle list")
			}
			body = []byte(`{"state":true,"count":0,"data":[]}`)
			if suffix == "srt" {
				body = []byte(`{"state":true,"count":1,"data":[{"cid":"42","n":"Film.srt","pc":"at6h4t9"}]}`)
			}
		case "/movies/subtitle":
			body = []byte(`{"state":true,"data":{"list":[{"file_name":"Film.srt","url":"https://fixture.invalid/subtitle-bytes"}]}}`)
		case "/subtitle-bytes":
			if r.Header.Get("Cookie") != "" {
				return nil, fmt.Errorf("Cookie leaked to subtitle URL")
			}
			mu.Lock()
			direct++
			mu.Unlock()
			body = []byte("subtitle")
		default:
			return nil, fmt.Errorf("extra %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})}, "UID=1")
	paths := []string{}
	err := c.SubtitleAttachments(context.Background(), "42", "subtitle-fixture", "/Movies", nil, func(path string, body []byte) error {
		paths = append(paths, path)
		if string(body) != "subtitle" {
			return fmt.Errorf("wrong bytes")
		}
		return nil
	})
	if err != nil || strings.Join(suffixes, ",") != "srt,ass,ssa" || direct != 1 || len(paths) != 1 || paths[0] != "/Movies/Film.srt" {
		t.Fatalf("suffix=%v direct=%d paths=%v error=%v", suffixes, direct, paths, err)
	}
}

func TestSubtitleFailureCancelsAndDrainsTransfers(t *testing.T) {
	stablePoints.Store("subtitle-cancel-fixture", "0123")
	started := make(chan struct{})
	stopped := make(chan struct{})
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"state":true,"count":0,"data":[]}`
		switch r.URL.Path {
		case "/os_windows/2.0/ufile/downfolders":
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(encryptedFixture(`{"state":true,"data":{"list":[],"has_next_page":false}}`))))}, nil
		case "/android/2.0/ufile/files":
			if r.URL.Query().Get("suffix") == "srt" {
				body = `{"state":true,"count":1,"data":[{"cid":"42","n":"Film.srt","pc":"at6h4t9"}]}`
			} else {
				<-started
				return nil, &HTTPError{StatusCode: 500}
			}
		case "/movies/subtitle":
			body = `{"state":true,"data":{"list":[{"file_name":"Film.srt","url":"https://fixture.invalid/block"}]}}`
		case "/block":
			close(started)
			<-r.Context().Done()
			close(stopped)
			return nil, r.Context().Err()
		default:
			return nil, fmt.Errorf("extra %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}, "UID=1")
	if err := c.SubtitleAttachments(context.Background(), "42", "subtitle-cancel-fixture", "/Movies", nil, func(string, []byte) error { return nil }); err == nil {
		t.Fatal("expected listing error")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("returned before active transfer stopped")
	}
}

func TestRootSubtitleHierarchyDirectoryOnlyWire(t *testing.T) {
	calls := 0
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		q := r.URL.Query()
		if r.URL.Path != "/android/2.0/ufile/files" || q.Get("cid") != "0" || q.Get("nf") != "1" || q.Get("show_dir") != "1" || q.Get("cur") != "1" || q.Get("count_folders") != "1" || q.Get("limit") != "7000" || q.Has("type") {
			return nil, fmt.Errorf("wrong root query %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"state":true,"count":0,"data":[]}`))}, nil
	})}, "UID=1")
	paths, err := c.subtitleHierarchy(context.Background(), "0", "root-fixture", "/")
	if err != nil || calls != 1 || paths["0"] != "" {
		t.Fatal(paths, calls, err)
	}
}

func TestRootSubtitleHierarchyStreamsChildrenAndUsesKnownPickcodes(t *testing.T) {
	stablePoints.Delete("root-stream-fixture")
	var mu sync.Mutex
	var trace []string
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body []byte
		q := r.URL.Query()
		switch r.URL.Path {
		case "/android/2.0/ufile/files":
			offset := q.Get("offset")
			mu.Lock()
			trace = append(trace, "root:"+offset)
			mu.Unlock()
			switch offset {
			case "0":
				body = []byte(`{"state":true,"count":7001,"data":[{"cid":"42","n":"First","pc":"a4sh4t9"}]}`)
			case "7000":
				body = []byte(`{"state":true,"count":7001,"data":[{"cid":"100","n":"Second","pc":"atah4t9"}]}`)
			default:
				return nil, &HTTPError{StatusCode: 400}
			}
		case "/os_windows/2.0/ufile/downfolders":
			mu.Lock()
			trace = append(trace, "folders:"+q.Get("pickcode"))
			mu.Unlock()
			body = encryptedFixture(`{"state":true,"data":{"list":[],"has_next_page":false}}`)
		default:
			t.Errorf("extra probe %s", r.URL)
			return nil, &HTTPError{StatusCode: 400}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})}, "UID=1")
	paths, err := c.subtitleHierarchy(context.Background(), "0", "root-stream-fixture", "/")
	if err != nil || paths["42"] != "/First" || paths["100"] != "/Second" {
		t.Fatal(paths, err)
	}
	mu.Lock()
	defer mu.Unlock()
	firstFolders, secondList := -1, -1
	for i, step := range trace {
		if step == "folders:a4sh4t9" && firstFolders < 0 {
			firstFolders = i
		}
		if step == "root:7000" {
			secondList = i
		}
	}
	if firstFolders < 0 || secondList < 0 || firstFolders > secondList {
		t.Fatalf("root hierarchy not streamed: %v", trace)
	}
	if _, ok := stablePoints.Load("root-stream-fixture"); ok {
		t.Fatal("known pickcodes unexpectedly probed stable point")
	}
}
