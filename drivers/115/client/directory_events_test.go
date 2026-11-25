package pan115client

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	cipher115 "litepan/drivers/115/client/cipher"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func encryptedFixture(raw string) []byte {
	data := []byte(raw)
	block := []byte{0xf0}
	remaining := len(data) - 15
	for remaining >= 255 {
		block = append(block, 255)
		remaining -= 255
	}
	block = append(block, byte(remaining))
	block = append(block, data...)
	frame := binary.LittleEndian.AppendUint16(nil, uint16(len(block)))
	frame = append(frame, block...)
	return cipher115.AESEncrypt(frame)
}
func TestDirectoryEventTraversalUsesOSPagesAndBatchSkim(t *testing.T) {
	stablePoints.Delete("fixture115")
	var probes atomic.Int64
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body []byte
		switch r.URL.Path {
		case "/files":
			if r.URL.Query().Get("limit") != "1" {
				return nil, fmt.Errorf("unexpected stable point probe")
			}
			body = []byte(`{"state":true,"data":[{"pc":"a4h4t9"}]}`)
		case "/android/folder/update":
			probes.Add(1)
			body = []byte(`{"state":true,"data":{"file_id":"42","file_name":"Movies","paths":[{"file_id":0,"file_name":""}]}}`)
		case "/os_windows/2.0/ufile/downfolders":
			if r.URL.Query().Get("page") != "1" {
				body = encryptedFixture(`{"state":true,"data":{"list":[],"has_next_page":false}}`)
				break
			}
			if r.URL.Query().Get("pickcode") != "a4sh4t9" || r.URL.Query().Get("per_page") != "5000" || r.URL.Query().Get("k_ec") == "" {
				return nil, fmt.Errorf("wrong folder wire")
			}
			body = encryptedFixture(`{"state":true,"data":{"list":[{"fid":"100","fn":"Sub","pid":"42"}],"has_next_page":false}}`)
		case "/os_windows/2.0/ufile/downfiles":
			if r.URL.Query().Get("page") != "1" {
				body = encryptedFixture(`{"state":true,"data":{"list":[],"has_next_page":false}}`)
				break
			}
			body = encryptedFixture(`{"state":true,"data":{"list":[{"pc":"at6h4t9","pid":"100","fs":12,"sha1":"hash"}],"has_next_page":0}}`)
		case "/files/file":
			if r.Method != "POST" {
				return nil, fmt.Errorf("skim must batch POST")
			}
			if err := r.ParseForm(); err != nil {
				return nil, err
			}
			if r.PostForm.Get("file_id") != "101" {
				return nil, fmt.Errorf("wrong skim ids")
			}
			body = []byte(`{"state":true,"data":[{"file_id":"101","pick_code":"at6h4t9","file_name":"Film.srt"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})}, "UID=123")
	if _, err := c.FolderPath(context.Background(), "42", "fixture115"); err != nil {
		t.Fatal(err)
	}
	files, err := c.DirectoryEventFiles(context.Background(), "42", "fixture115")
	if probes.Load() != 1 {
		t.Fatalf("duplicate folder probes: %d", probes.Load())
	}
	if err != nil || len(files) != 1 || files[0].Path != "/Movies/Sub/Film.srt" || files[0].ID != "101" {
		t.Fatalf("files=%+v error=%v", files, err)
	}
}
func TestPickcodeCodecReferenceVectors(t *testing.T) {
	for id, pc := range map[string]string{"1": "a4h4t9", "42": "a4sh4t9", "100": "atah4t9", "101": "at6h4t9"} {
		got, err := encodePickcode(id, "0123")
		if err != nil || got != pc {
			t.Fatalf("%s =>%s %v", id, got, err)
		}
		decoded, stable, err := decodePickcode(pc)
		if err != nil || decoded != id || stable != "0123" {
			t.Fatal(decoded, stable, err)
		}
	}
}

func TestDownloadNodesMissingTerminalFieldFailsWithoutRetry(t *testing.T) {
	var firstPage atomic.Int64
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw := `{"state":true,"data":{"list":[],"has_next_page":false}}`
		if r.URL.Query().Get("page") == "1" && firstPage.Add(1) == 1 {
			raw = `{"state":true,"data":{"list":[]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(encryptedFixture(raw))))}, nil
	})}, "UID=1")
	if _, err := c.downloadNodes(context.Background(), "known", false); err == nil || firstPage.Load() != 1 {
		t.Fatal(firstPage.Load(), err)
	}
}

func TestDownloadNodesMissingTerminalFieldStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int64
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) >= 1 {
			cancel()
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(encryptedFixture(`{"state":true,"data":{"list":[]}}`))))}, nil
	})}, "UID=1")
	if _, err := c.downloadNodes(ctx, "known", false); err == nil {
		t.Fatal("missing terminal field advanced successfully")
	}
}
