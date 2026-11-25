package pan115

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"io"
	"litepan/internal/driver"
	"net/http"
	"testing"
	"time"
)

func TestOpenReferenceStickySessionHeadersAndCompression(t *testing.T) {
	for _, encoding := range []string{"gzip", "deflate", "raw-deflate"} {
		t.Run(encoding, func(t *testing.T) {
			d := &Driver{token: "old", refresh: "refresh"}
			expectedUA := "EmbyNginx"
			d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
				if r.UserAgent() != expectedUA || r.Header.Get("Accept") != "*/*" || r.Header.Get("Accept-Encoding") != "gzip, deflate" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.Header.Get("Accept-Language") != "" || r.Header.Get("Cookie") != "" {
					t.Fatalf("headers: %v", r.Header)
				}
				body := `{"state":true,"data":{}}`
				if r.URL.Path == pathDownload {
					body = referenceLink
				}
				if r.URL.Path == "/open/refreshToken" {
					body = `{"state":1,"data":{"access_token":"new","refresh_token":"next","expires_in":3600}}`
				}
				var buf bytes.Buffer
				var w io.WriteCloser
				switch encoding {
				case "gzip":
					w = gzip.NewWriter(&buf)
				case "deflate":
					w = zlib.NewWriter(&buf)
				default:
					w, _ = flate.NewWriter(&buf, flate.DefaultCompression)
				}
				_, _ = w.Write([]byte(body))
				_ = w.Close()
				enc := encoding
				if enc == "raw-deflate" {
					enc = "deflate"
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Encoding": {enc}}, Body: io.NopCloser(&buf)}, nil
			})}
			if err := d.rawRequest(context.Background(), "GET", pathUserInfo, "old", nil, nil, nil, nil, false); err != nil {
				t.Fatal(err)
			}
			expectedUA = "player-UA"
			d.rememberPickCode(fileEntry{FileID: "10", PickCode: "pc"})
			if _, err := d.ResolveDownload(context.Background(), driver.DownloadRequest{FileID: "10", UA: expectedUA}); err != nil {
				t.Fatal(err)
			}
			if err := d.rawRequest(context.Background(), "GET", pathUserInfo, "old", nil, nil, nil, nil, false); err != nil {
				t.Fatal(err)
			}
			if _, err := d.exchangeToken(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOpen429RechecksExpiryBeforeRetry(t *testing.T) {
	d := &Driver{token: "old", refresh: "refresh", tokenExpires: time.Now().Add(time.Hour)}
	var paths []string
	d.openWait = func(ctx context.Context, delay time.Duration) error {
		if delay != 5*time.Second {
			t.Fatalf("wait=%v", delay)
		}
		d.mu.Lock()
		d.tokenExpires = time.Now().Add(-time.Second)
		d.mu.Unlock()
		openBucket.Lock()
		openBucket.last = time.Time{}
		openBucket.Unlock()
		return nil
	}
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		status := 200
		body := `{"state":true,"data":{}}`
		h := http.Header{}
		if len(paths) == 1 {
			status = 429
			h.Set("X-RateLimit-Reset", "0")
		} else if r.URL.Path == "/open/refreshToken" {
			body = `{"state":1,"data":{"access_token":"new","refresh_token":"next","expires_in":3600}}`
		} else if r.Header.Get("Authorization") != "Bearer new" {
			t.Fatalf("stale token: %v", r.Header)
		}
		return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	openBucket.Lock()
	openBucket.last = time.Time{}
	openBucket.Unlock()
	if err := d.apiCall(context.Background(), "GET", pathUserInfo, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 || paths[0] != pathUserInfo || paths[1] != "/open/refreshToken" || paths[2] != pathUserInfo {
		t.Fatalf("trace=%v", paths)
	}
}

func TestOpenDownloadUpdatesSessionBeforeExpiryRefresh(t *testing.T) {
	d := &Driver{token: "old", refresh: "refresh", tokenExpires: time.Now().Add(-time.Second)}
	d.rememberPickCode(fileEntry{FileID: "10", PickCode: "pc"})
	var trace []string
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		trace = append(trace, r.URL.Path)
		if r.UserAgent() != "expired-player" {
			t.Fatalf("UA=%q", r.UserAgent())
		}
		body := referenceLink
		if r.URL.Path == "/open/refreshToken" {
			body = `{"state":1,"data":{"access_token":"new","refresh_token":"next","expires_in":3600}}`
		} else if r.Header.Get("Authorization") != "Bearer new" {
			t.Fatal("stale token")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	openBucket.Lock()
	openBucket.last = time.Time{}
	openBucket.Unlock()
	if _, err := d.ResolveDownload(context.Background(), driver.DownloadRequest{FileID: "10", UA: "expired-player"}); err != nil {
		t.Fatal(err)
	}
	if len(trace) != 2 || trace[0] != "/open/refreshToken" || trace[1] != pathDownload {
		t.Fatalf("trace=%v", trace)
	}
}

func TestOpenRefreshReferenceCodeEnvelope(t *testing.T) {
	for _, tc := range []struct {
		body string
		fail bool
	}{
		{`{"code":0,"data":{"access_token":"new","refresh_token":"next"}}`, false},
		{`{"code":0,"state":true,"data":{"access_token":"new","refresh_token":"next"}}`, false},
		{`{"code":0,"state":false,"data":{"access_token":"new","refresh_token":"next"}}`, false},
		{`{"code":40140115,"data":{"access_token":"new"}}`, false},
		{`{"code":0,"data":null}`, true},
	} {
		d := &Driver{token: "old", refresh: "refresh"}
		d.client = &http.Client{Transport: fixtureTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(tc.body))}, nil
		})}
		_, err := d.exchangeToken(context.Background())
		if (err != nil) != tc.fail {
			t.Fatalf("body=%s err=%v", tc.body, err)
		}
		if tc.fail && d.token != "old" {
			t.Fatal("failed response changed token")
		}
		if !tc.fail && d.token != "new" {
			t.Fatal("success did not change token")
		}
	}
}

func TestOpenAdmissionPrecedesExpiryRefresh(t *testing.T) {
	d := &Driver{token: "old", refresh: "refresh", tokenExpires: time.Now().Add(-time.Second)}
	var trace []string
	d.openAdmission = func(context.Context) error { trace = append(trace, "admission"); return nil }
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		trace = append(trace, r.URL.Path)
		body := `{"data":{}}`
		if r.URL.Path == "/open/refreshToken" {
			body = `{"data":{"access_token":"new","expires_in":3600}}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	if err := d.apiCall(context.Background(), "GET", pathUserInfo, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(trace) != 3 || trace[0] != "admission" || trace[1] != "/open/refreshToken" || trace[2] != pathUserInfo {
		t.Fatalf("trace=%v", trace)
	}
}

func TestOpenReferenceEndpointEnvelopePolicies(t *testing.T) {
	for _, p := range []string{pathFileInfo, pathDownload, pathUserInfo, pathList, pathDelete, pathCopy, pathRename, pathMkdir} {
		d := &Driver{token: "token"}
		calls := 0
		d.client = &http.Client{Transport: fixtureTransport(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(`{"state":false,"code":99,"data":{"file_id":"10"}}`))}, nil
		})}
		var data map[string]any
		err := d.rawRequest(context.Background(), "POST", p, "token", nil, nil, nil, &data, false)
		mutation := p == pathCopy || p == pathRename || p == pathMkdir
		if (err != nil) != mutation || calls != 1 {
			t.Fatalf("path=%s err=%v calls=%d", p, err, calls)
		}
		if !mutation && data["file_id"] != "10" {
			t.Fatal("data lost")
		}
	}
}

func TestOpenPathEmptyFalseStateRetriesAfterReferenceWait(t *testing.T) {
	d := &Driver{token: "token"}
	calls := 0
	waits := 0
	d.openAdmission = func(context.Context) error { return nil }
	d.openWait = func(_ context.Context, delay time.Duration) error {
		waits++
		if delay != 30*time.Second {
			t.Fatal(delay)
		}
		return nil
	}
	d.client = &http.Client{Transport: fixtureTransport(func(*http.Request) (*http.Response, error) {
		calls++
		body := `{"state":false,"data":{}}`
		if calls == 2 {
			body = referenceEntry
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(body))}, nil
	})}
	entry, err := d.playbackPathInfo(context.Background(), "/movie")
	if err != nil || entry.entryID() != "10" || calls != 2 || waits != 1 {
		t.Fatalf("err=%v calls=%d waits=%d", err, calls, waits)
	}
}

func TestOpenDeleteFalseEnvelopeIsOneHTTPCall(t *testing.T) {
	d := &Driver{token: "token"}
	calls := 0
	d.openAdmission = func(context.Context) error { return nil }
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		_ = r.ParseForm()
		if r.URL.Path != pathDelete || r.Form.Get("file_ids") != "10" {
			t.Fatalf("request=%v %v", r.URL, r.Form)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(`{"state":false,"code":99}`))}, nil
	})}
	if err := d.DeleteFiles(context.Background(), []string{"10"}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
