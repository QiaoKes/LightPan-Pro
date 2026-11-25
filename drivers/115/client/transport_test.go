package pan115client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestRequestPreservesEnvelopeAndHeaders(t *testing.T) {
	calls := 0
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Cookie") != "UID=1" {
			t.Fatal("missing Cookie")
		}
		if r.Header.Get("User-Agent") != "" {
			t.Fatal("empty UA lost")
		}
		if r.URL.Query().Get("path") != "/中文 +" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"state":false,"errno":99}`)), Header: make(http.Header)}, nil
	})}, "UID=1")
	data, err := c.Request(context.Background(), "GET", "https://webapi.115.com/files", url.Values{"path": {"/中文 +"}}, nil, http.Header{"User-Agent": {""}})
	if err != nil || string(data) != `{"state":false,"errno":99}` || calls != 1 {
		t.Fatalf("%s %v calls=%d", data, err, calls)
	}
}
func TestCookieExchangeMultipartAndResponseCookies(t *testing.T) {
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Host != "passportapi.115.com" || !strings.Contains(string(body), "Content-Disposition:  form-data;  name=\"app\"\r\n\r\nweb") {
			t.Fatalf("unexpected exchange: %s %s", r.URL, body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"state":true}`)), Header: http.Header{"Set-Cookie": {"UID=42; Path=/", "CID=secret; HttpOnly"}}}, nil
	})}, "")
	cookie, _, err := c.CookieQRExchange(context.Background(), "web", "opaque")
	if err != nil || cookie != "UID=42; CID=secret" {
		t.Fatalf("exchange %q %v", cookie, err)
	}
}
func TestOpenQRPKCEAndNoCookie(t *testing.T) {
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Cookie") != "" {
			t.Fatal("Cookie leaked to Open auth")
		}
		_ = r.ParseForm()
		if r.Form.Get("code_challenge") != "ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0=" || r.Form.Get("code_challenge_method") != "sha256" {
			t.Fatal(r.Form)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":0}`)), Header: make(http.Header)}, nil
	})}, "UID=1")
	if _, err := c.OpenQRToken(context.Background(), "123", "abc"); err != nil {
		t.Fatal(err)
	}
}
func TestRequestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := New(nil, "")
	if _, err := c.Request(ctx, "GET", "https://webapi.115.com/files", nil, nil, nil); err == nil {
		t.Fatal("cancel ignored")
	}
}

func TestCookieErrorClassification(t *testing.T) {
	for _, x := range []struct {
		raw  string
		want ErrorKind
	}{{`{"state":false,"errno":99}`, AuthExpired}, {`{"state":false,"errno":911}`, VerificationRequired}, {`{"state":false,"errno":1001}`, UpstreamRejected}, {`{"state":true}`, ""}} {
		if got := KindOf(CheckResponse([]byte(x.raw))); got != x.want {
			t.Fatalf("%s got %s", x.raw, got)
		}
	}
	if KindOf(&HTTPError{403}) != PermissionDenied {
		t.Fatal("403 mislabeled")
	}
}
func TestNormalizeDirectoryPickcode(t *testing.T) {
	if got, err := normalizePickcode("fa44444"); err != nil || got != "" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestQRStatusExactQuery(t *testing.T) {
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.RawQuery; got != "sign=s&time=123&uid=u" {
			t.Fatalf("business QR query contaminated: %s", got)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"state":true}`))}, nil
	})}, "")
	if _, err := c.QRStatus(context.Background(), "u", "123", "s"); err != nil {
		t.Fatal(err)
	}
}

func TestCookieDownloadExactQueryAndBody(t *testing.T) {
	raw, err := os.ReadFile("cipher/test-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Request struct {
			Token string `json:"k_ec"`
			Body  string `json:"wire_body_hex"`
		} `json:"cookie_download_request"`
	}
	if err = json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		want := url.Values{"k_ec": {vector.Request.Token}}.Encode()
		if r.URL.RawQuery != want {
			t.Fatalf("Cookie query %s want %s", r.URL.RawQuery, want)
		}
		b, _ := io.ReadAll(r.Body)
		if hex.EncodeToString(b) != vector.Request.Body {
			t.Fatal("bundled wire body mismatch")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"state":false,"errno":99}`))}, nil
	})}, "UID=42_abc")
	if _, err = c.CookieDownload(context.Background(), "abc", ""); KindOf(err) != AuthExpired {
		t.Fatal(err)
	}
}

func TestECDHRequestBundledWire(t *testing.T) {
	raw, err := os.ReadFile("cipher/test-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Response struct {
			Cipher string `json:"ciphertext_hex"`
			Plain  string `json:"decoded_json_utf8"`
		} `json:"aes_response"`
		Request struct {
			Token string `json:"k_ec"`
			Form  string `json:"form_before_aes_utf8"`
			Body  string `json:"wire_body_hex"`
		} `json:"cookie_download_request"`
	}
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	form, err := url.ParseQuery(strings.TrimSuffix(v.Request.Form, "&"))
	if err != nil {
		t.Fatal(err)
	}
	c := New(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.RawQuery != (url.Values{"k_ec": {v.Request.Token}}).Encode() {
			t.Fatal(r.URL.RawQuery)
		}
		body, _ := io.ReadAll(r.Body)
		if hex.EncodeToString(body) != v.Request.Body {
			t.Fatal("encrypted form mismatch")
		}
		if r.Header.Get("Cookie") != "UID=42" {
			t.Fatal("missing Cookie")
		}
		response, _ := hex.DecodeString(v.Response.Cipher)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(response)))}, nil
	})}, "UID=42")
	out, err := c.ECDHRequest(context.Background(), "POST", "https://proapi.115.com/os_windows/2.0/ufile/downfiles", nil, form, nil)
	if err != nil || string(out) != v.Response.Plain {
		t.Fatalf("%s %v", out, err)
	}
}
