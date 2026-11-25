package pan115client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAttachmentPathKeepsFullProbeThenParentAndList(t *testing.T) {
	attachmentOriginSequence.Store(0)
	var trace []string
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		trace = append(trace, r.URL.Host+r.URL.Path+" "+r.URL.Query().Get("path"))
		body := ""
		switch len(trace) {
		case 1:
			if r.URL.Query().Get("path") != "Movies/a.nfo" {
				return nil, fmt.Errorf("full path probe missing")
			}
			body = `{"state":true,"id":0}`
		case 2:
			if r.URL.Query().Get("path") != "Movies" {
				return nil, fmt.Errorf("parent probe missing")
			}
			body = `{"state":true,"id":"42"}`
		case 3:
			if r.URL.Query().Has("type") || r.URL.Query().Has("nf") || r.URL.Host != "webapi.115.com" || r.URL.Query().Get("limit") != "1150" || r.URL.Query().Get("cid") != "42" || r.URL.Query().Get("custom_order") != "2" {
				return nil, fmt.Errorf("bad list parameters")
			}
			body = `{"state":true,"count":1,"data":[{"n":"a.nfo","fid":"101"}]}`
		default:
			return nil, fmt.Errorf("extra request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}, "UID=1")
	id, err := c.AttachmentID(context.Background(), "/Movies/a.nfo")
	if err != nil || id != "101" || len(trace) != 3 {
		t.Fatal(id, trace, err)
	}
}
