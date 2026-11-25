package pan115client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Wire mapping: restored-python/util/wangpan/util_115.py:306-355.
func (c *Client) CookieQRToken(ctx context.Context, app string) (json.RawMessage, error) {
	if app == "" || strings.ContainsAny(app, "/?#\r\n") {
		return nil, fmt.Errorf("invalid QR app")
	}
	return c.Request(ctx, "GET", "https://qrcodeapi.115.com/api/1.0/"+app+"/1.0/token/", nil, nil, nil)
}
func (c *Client) QRStatus(ctx context.Context, uid, time, sign string) (json.RawMessage, error) {
	return c.request(ctx, "GET", "https://qrcodeapi.115.com/get/status/", url.Values{"uid": {uid}, "time": {time}, "sign": {sign}}, nil, nil, false)
}

// CookieQRExchange reads response Set-Cookie fields, as the reference does;
// JSON cookie-shaped data is not accepted as a substitute.
func (c *Client) CookieQRExchange(ctx context.Context, app, uid string) (string, json.RawMessage, error) {
	if app == "" || strings.ContainsAny(app+uid, "\r\n") || strings.ContainsAny(app, "/?#") {
		return "", nil, fmt.Errorf("invalid QR exchange arguments")
	}
	const boundary = "----WebKitFormBoundaryCfAXUMKv8XAmw2EU"
	body := "--" + boundary + "\r\nContent-Disposition:  form-data;  name=\"app\"\r\n\r\n" + app + "\r\n--" + boundary + "\r\nContent-Disposition:  form-data;  name=\"account\"\r\n\r\n" + uid + "\r\n--" + boundary + "--\r\n"
	req, err := http.NewRequestWithContext(ctx, "POST", "https://passportapi.115.com/app/1.0/"+app+"/1.0/login/qrcode/", strings.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", nil, &HTTPError{StatusCode: resp.StatusCode}
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	if !json.Valid(data) {
		return "", nil, fmt.Errorf("115 response is not valid JSON")
	}
	var cookies []string
	for _, cookie := range resp.Cookies() {
		cookies = append(cookies, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(cookies, "; "), data, nil
}
