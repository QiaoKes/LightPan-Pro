package pan115client

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"testing"
)

func TestResponseBodyCompressionBoundsAndTransportDecode(t *testing.T) {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, _ = w.Write(bytes.Repeat([]byte("a"), 1024))
	_ = w.Close()
	response := func() *http.Response {
		return &http.Response{Header: http.Header{"Content-Encoding": {"gzip"}}, Body: io.NopCloser(bytes.NewReader(b.Bytes()))}
	}
	if _, err := ReadResponseBody(response(), 100); err == nil {
		t.Fatal("expanded limit not enforced")
	}
	if raw, err := ReadResponseBody(response(), 0); err != nil || len(raw) != 1024 {
		t.Fatalf("unbounded decode len=%d err=%v", len(raw), err)
	}
	r := response()
	r.Uncompressed = true
	r.Body = io.NopCloser(bytes.NewBufferString("decoded"))
	if raw, err := ReadResponseBody(r, 100); err != nil || string(raw) != "decoded" {
		t.Fatalf("transport decode=%s err=%v", raw, err)
	}
}
