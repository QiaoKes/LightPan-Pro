package pan115client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type treeTransport func(*http.Request) (*http.Response, error)

func (f treeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestExportTreeDownloadAttemptsAndCleanupOrder(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			var trace []string
			attempts := 0
			c := New(&http.Client{Transport: treeTransport(func(r *http.Request) (*http.Response, error) {
				trace = append(trace, r.Method+" "+r.URL.Path)
				body := ""
				switch r.URL.Path {
				case "/files/getid":
					body = `{"id":"42"}`
				case "/files/export_dir":
					if r.Method == "POST" {
						r.ParseForm()
						if r.Form.Get("file_ids") != "42" || r.Form.Get("target") != "U_1_0" {
							t.Fatal(r.Form)
						}
						body = `{"data":"job"}`
					} else {
						body = `{"data":{"pick_code":"pc","file_id":"temporary"}}`
					}
				case "/tree":
					attempts++
					if r.Header.Get("User-Agent") != "" {
						t.Fatal("download UA")
					}
					if failed || attempts < 6 {
						return nil, fmt.Errorf("transfer failed")
					}
					body = "|——root\n| |-scope\n| | |-movie.mkv"
				case "/rb/delete":
					r.ParseForm()
					if r.Form.Get("fid") != "temporary" {
						t.Fatal(r.Form)
					}
					body = `{"state":true}`
				default:
					t.Fatal(r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}, "cookie")
			paths, complete, err := c.exportTree(context.Background(), "/scope", func(_ context.Context, pc, ua string) (*Download, error) {
				if pc != "pc" || ua != "" {
					t.Fatal(pc, ua)
				}
				trace = append(trace, "resolve")
				return &Download{URL: "https://fixture/tree"}, nil
			})
			if attempts != 6 {
				t.Fatal(attempts)
			}
			last := trace[len(trace)-1]
			if failed {
				if err == nil || complete || last == "POST /rb/delete" {
					t.Fatal(trace, err)
				}
			} else {
				if err != nil || !complete || len(paths) == 0 || last != "POST /rb/delete" {
					t.Fatal(trace, paths, err)
				}
			}
		})
	}
}

func TestExportCleanupFailurePreventsParsing(t *testing.T) {
	calls := []string{}
	c := New(&http.Client{Transport: treeTransport(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.Path)
		body := ""
		switch r.URL.Path {
		case "/files/getid":
			body = `{"id":"42"}`
		case "/files/export_dir":
			if r.Method == "POST" {
				body = `{"data":"job"}`
			} else {
				body = `{"data":{"pick_code":"pc","file_id":"temporary"}}`
			}
		case "/tree":
			body = "not a valid tree"
		case "/rb/delete":
			body = `{"state":false}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}, "cookie")
	paths, complete, err := c.exportTree(context.Background(), "/scope", func(context.Context, string, string) (*Download, error) {
		return &Download{URL: "https://fixture/tree"}, nil
	})
	var traceErr *TreeExportError
	if !errors.As(err, &traceErr) || traceErr.ExportID != "job" || traceErr.FileID != "temporary" {
		t.Fatal("missing export trace", err)
	}
	if err == nil || complete || len(paths) > 0 || calls[len(calls)-1] != "/rb/delete" {
		t.Fatal(paths, complete, err, calls)
	}
}

func TestExportRejectedRootListingDoesNotStartExport(t *testing.T) {
	calls := 0
	c := New(&http.Client{Transport: treeTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/files" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"state":false}`)), Header: make(http.Header)}, nil
	})}, "cookie")
	_, complete, err := c.ExportTree(context.Background(), "/")
	if err == nil || complete || calls != 1 {
		t.Fatal(complete, err, calls)
	}
}

func TestRootExportKeepsSinglePageOrderButRejectsCompleteness(t *testing.T) {
	var forms []string
	lists := 0
	c := New(&http.Client{Transport: treeTransport(func(r *http.Request) (*http.Response, error) {
		body := ""
		switch r.URL.Path {
		case "/files":
			lists++
			if r.URL.Query().Get("limit") != "32" || r.URL.Query().Get("offset") != "0" {
				t.Fatal(r.URL)
			}
			body = `{"data":[{"fid":"file"},{"cid":"dir"}],"count":3}`
		case "/files/export_dir":
			if r.Method == "POST" {
				r.ParseForm()
				forms = append(forms, r.Form.Get("file_ids"))
				body = `{"data":"job"}`
			} else {
				body = `{"data":{"pick_code":"pc","file_id":"temp"}}`
			}
		case "/tree":
			body = "|——root\n| |-movie.mkv"
		case "/rb/delete":
			body = `{"state":true}`
		default:
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}, "cookie")
	_, complete, err := c.exportTree(context.Background(), "/", func(context.Context, string, string) (*Download, error) {
		return &Download{URL: "https://fixture/tree"}, nil
	})
	if err != nil || complete || lists != 1 || len(forms) != 1 || forms[0] != "file,dir" {
		t.Fatal(complete, lists, forms, err)
	}
}
