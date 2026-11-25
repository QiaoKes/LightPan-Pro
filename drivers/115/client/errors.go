package pan115client

import (
	"encoding/json"
	"errors"
	"fmt"
)

// HTTPError deliberately excludes response bodies and URLs, which may contain
// credentials or signed download links.
type HTTPError struct{ StatusCode int }

func (e *HTTPError) Error() string { return fmt.Sprintf("115 HTTP status %d", e.StatusCode) }

// ErrorKind separates invalid credentials from verification and ordinary failures.
type ErrorKind string

const (
	AuthExpired          ErrorKind = "auth_expired"
	VerificationRequired ErrorKind = "verification_required"
	PermissionDenied     ErrorKind = "permission_denied"
	UpstreamRejected     ErrorKind = "upstream_rejected"
)

type APIError struct {
	Kind           ErrorKind
	Code           int64
	ResponseStatus int64
}

func (e *APIError) Error() string { return fmt.Sprintf("115 %s (code %d)", e.Kind, e.Code) }

// CheckResponse is opt-in; Request intentionally retains SDK check=false semantics.
func CheckResponse(raw json.RawMessage) error {
	var v map[string]json.RawMessage
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	state, ok := v["state"]
	if !ok || string(state) == "true" || string(state) == "1" {
		return nil
	}
	var code int64
	for _, key := range []string{"errno", "errNo", "errcode", "errCode", "code", "msg_code"} {
		if b := v[key]; len(b) > 0 {
			var n json.Number
			if json.Unmarshal(b, &n) == nil {
				code, _ = n.Int64()
			}
			if code != 0 {
				break
			}
		}
	}
	kind := UpstreamRejected
	switch code {
	case 99, 40101032:
		kind = AuthExpired
	case 911, 40101004, 40101009, 40101010, 40101017:
		kind = VerificationRequired
	}
	var responseStatus int64
	for _, key := range []string{"status", "code", "status_code"} {
		if string(v[key]) == "null" {
			continue
		}
		var value int64
		if json.Unmarshal(v[key], &value) == nil {
			responseStatus = value
			break
		}
		var boolean bool
		if json.Unmarshal(v[key], &boolean) == nil {
			if boolean {
				responseStatus = 1
			}
			break
		}
	}
	return &APIError{Kind: kind, Code: code, ResponseStatus: responseStatus}
}

// KindOf classifies HTTP failures without treating permission denial as expiry.
func KindOf(err error) ErrorKind {
	var api *APIError
	if errors.As(err, &api) {
		return api.Kind
	}
	var status *HTTPError
	if errors.As(err, &status) {
		switch status.StatusCode {
		case 401:
			return AuthExpired
		case 403:
			return PermissionDenied
		}
	}
	return ""
}
