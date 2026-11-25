// Package pan115client implements the 115 wire transport. Workflow retries and
// Open business scheduling belong to callers.
package pan115client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	cipher115 "litepan/drivers/115/client/cipher"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Client retains only account Cookie state; per-call headers are never shared.
type Client struct {
	http   *http.Client
	mu     sync.RWMutex
	cookie string
}

func New(httpClient *http.Client, cookie string) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{http: httpClient, cookie: cookie}
}
func (c *Client) SetCookie(cookie string) { c.mu.Lock(); c.cookie = cookie; c.mu.Unlock() }

// Request preserves the application envelope, including state=false. It makes
// exactly one request and does not interpret application authentication errors.
func (c *Client) Request(ctx context.Context, method, rawURL string, query, form url.Values, headers http.Header) (json.RawMessage, error) {
	return c.request(ctx, method, rawURL, query, form, headers, true)
}

func (c *Client) request(ctx context.Context, method, rawURL string, query, form url.Values, headers http.Header, sdkDefaults bool) (json.RawMessage, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("115 request URL: %w", err)
	}
	q := u.Query()
	if sdkDefaults && query != nil && query.Get("app_ver") == "" {
		q.Set("app_ver", "36.2.28")
	}
	for k, vs := range query {
		q.Del(k)
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	for k, vs := range headers {
		req.Header[http.CanonicalHeaderKey(k)] = append([]string(nil), vs...)
	}
	if form != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if _, ok := req.Header["Cookie"]; !ok && !strings.HasPrefix(u.Path, "/open/") {
		c.mu.RLock()
		cookie := c.cookie
		c.mu.RUnlock()
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: resp.StatusCode}
	}
	data, err := ReadResponseBody(resp, 0)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("115 response is not valid JSON")
	}
	return json.RawMessage(data), nil
}

// ECDHRequest explicitly selects the SDK's fixed ECDH-derived AES transport.
// It does not enable encryption for ordinary Request calls or add a scheduler.
func (c *Client) ECDHRequest(ctx context.Context, method, rawURL string, query, form url.Values, headers http.Header) (json.RawMessage, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	if query != nil {
		for k, vs := range query {
			q[k] = append([]string(nil), vs...)
		}
		if _, ok := query["app_ver"]; !ok {
			q.Set("app_ver", "36.2.28")
		}
	}
	q.Set("k_ec", cipher115.Token())
	u.RawQuery = q.Encode()
	var body io.Reader
	if len(form) > 0 {
		body = bytes.NewReader(cipher115.AESEncrypt([]byte(form.Encode() + "&")))
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	for k, vs := range headers {
		req.Header[http.CanonicalHeaderKey(k)] = append([]string(nil), vs...)
	}
	if req.Header.Get("Referer") == "" {
		req.Header.Set("Referer", u.Scheme+"://"+u.Host)
	}
	if len(form) > 0 {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if _, ok := req.Header["Cookie"]; !ok {
		c.mu.RLock()
		req.Header.Set("Cookie", c.cookie)
		c.mu.RUnlock()
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: resp.StatusCode}
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	raw, err = cipher115.AESDecrypt(raw)
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("115 decrypted response is not valid JSON")
	}
	return raw, nil
}
