package pan115client

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
)

// Wire mapping: restored-python/util/wangpan/util_115_open.py:54-76,116-132.
func openAuthHeaders() http.Header {
	return http.Header{"User-Agent": {"EmbyNginx"}, "Content-Type": {"application/x-www-form-urlencoded"}, "Accept": {"*/*"}, "Accept-Encoding": {"gzip, deflate"}, "Connection": {"keep-alive"}, "Cookie": {""}}
}
func (c *Client) OpenQRToken(ctx context.Context, clientID, verifier string) (json.RawMessage, error) {
	digest := sha256.Sum256([]byte(verifier))
	return c.Request(ctx, "POST", "https://passportapi.115.com/open/authDeviceCode", nil, url.Values{"client_id": {clientID}, "code_challenge": {base64.StdEncoding.EncodeToString(digest[:])}, "code_challenge_method": {"sha256"}}, openAuthHeaders())
}
func (c *Client) OpenQRExchange(ctx context.Context, uid, verifier string) (json.RawMessage, error) {
	return c.Request(ctx, "POST", "https://passportapi.115.com/open/deviceCodeToToken", nil, url.Values{"uid": {uid}, "code_verifier": {verifier}}, openAuthHeaders())
}
func (c *Client) OpenRefresh(ctx context.Context, refreshToken string) (json.RawMessage, error) {
	return c.Request(ctx, "POST", "https://passportapi.115.com/open/refreshToken", nil, url.Values{"refresh_token": {refreshToken}}, openAuthHeaders())
}

// OpenQRStatus uses the fresh Open session defaults without SDK query additions.
func (c *Client) OpenQRStatus(ctx context.Context, uid, stamp, sign string) (json.RawMessage, error) {
	return c.request(ctx, "GET", "https://qrcodeapi.115.com/get/status/", url.Values{"uid": {uid}, "time": {stamp}, "sign": {sign}}, nil, openAuthHeaders(), false)
}
