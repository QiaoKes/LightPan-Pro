package pan115client

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ReadResponseBody bounds both compressed input and decoded JSON payload.
func ReadResponseBody(resp *http.Response, limit int64) ([]byte, error) {
	var input io.Reader = resp.Body
	if limit > 0 {
		input = io.LimitReader(input, limit+1)
	}
	raw, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	if limit > 0 && int64(len(raw)) > limit {
		return nil, fmt.Errorf("115 response exceeds limit")
	}
	if resp.Uncompressed {
		return raw, nil
	}
	var reader io.ReadCloser
	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "gzip":
		reader, err = gzip.NewReader(bytes.NewReader(raw))
	case "deflate":
		reader, err = zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			reader = flate.NewReader(bytes.NewReader(raw))
			err = nil
		}
	default:
		return raw, nil
	}
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var expanded io.Reader = reader
	if limit > 0 {
		expanded = io.LimitReader(expanded, limit+1)
	}
	decoded, err := io.ReadAll(expanded)
	if err == nil && limit > 0 && int64(len(decoded)) > limit {
		err = fmt.Errorf("115 decoded response exceeds limit")
	}
	return decoded, err
}
