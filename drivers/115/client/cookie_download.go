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
	"strconv"
	"strings"
)

type Download struct {
	URL                    string
	Headers                http.Header
	FileID, PickCode, Name string
	Size                   int64
}

// CookieDownload follows download_url -> download_url_app's os_windows branch.
func (c *Client) CookieDownload(ctx context.Context, pickcode, ua string) (*Download, error) {
	c.mu.RLock()
	cookie := c.cookie
	c.mu.RUnlock()
	var uid string
	var err error
	for _, part := range strings.Split(cookie, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && k == "UID" {
			uid = strings.SplitN(v, "_", 2)[0]
		}
	}
	if uid == "" {
		return nil, fmt.Errorf("Cookie UID missing")
	}
	if strings.HasPrefix(pickcode, "f") {
		pickcode, err = normalizePickcode(pickcode)
		if err != nil {
			return nil, err
		}
	}
	uidNum, err := strconv.ParseUint(uid, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid Cookie UID")
	}
	// Preserve insertion order and compact JSON used by the SDK serializer.
	payload := []byte(fmt.Sprintf(`{"pickcode":%s,"user_id":%d}`, mustJSONString(pickcode), uidNum))
	body := cipher115.AESEncrypt([]byte(url.Values{"data": {cipher115.RSAEncrypt(payload)}}.Encode() + "&"))
	endpoint := "https://proapi.115.com/os_windows/2.0/ufile/downurl?k_ec=" + url.QueryEscape(cipher115.Token())
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Referer", "https://proapi.115.com")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, &HTTPError{StatusCode: resp.StatusCode}
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		raw, err = cipher115.AESDecrypt(raw)
		if err != nil {
			return nil, err
		}
	}
	var envelope struct {
		State bool            `json:"state"`
		Data  json.RawMessage `json:"data"`
	}
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if err = CheckResponse(raw); err != nil {
		return nil, err
	}
	var encrypted string
	if err = json.Unmarshal(envelope.Data, &encrypted); err != nil {
		return nil, err
	}
	decoded, err := cipher115.RSADecrypt(encrypted)
	if err != nil {
		return nil, err
	}
	var direct struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(decoded, &direct) == nil && direct.URL != "" {
		u, _ := url.Parse(direct.URL)
		name, _ := url.PathUnescape(u.Path[strings.LastIndex(u.Path, "/")+1:])
		return &Download{URL: direct.URL, PickCode: pickcode, Name: name, Headers: http.Header{"User-Agent": {ua}}}, nil
	}
	var entries map[string]struct {
		URL struct {
			URL string `json:"url"`
		} `json:"url"`
		PickCode string      `json:"pick_code"`
		Name     string      `json:"file_name"`
		Size     json.Number `json:"file_size"`
	}
	if err = json.Unmarshal(decoded, &entries); err != nil {
		return nil, err
	}
	for fid, v := range entries {
		if v.URL.URL == "" {
			return nil, fmt.Errorf("download target is directory")
		}
		size, _ := v.Size.Int64()
		return &Download{URL: v.URL.URL, Headers: http.Header{"User-Agent": {ua}}, FileID: fid, PickCode: v.PickCode, Name: v.Name, Size: size}, nil
	}
	return nil, fmt.Errorf("empty download response")
}
func mustJSONString(s string) string { b, _ := json.Marshal(s); return string(b) }

// normalizePickcode follows p115pickcode's reverse substitution, base36 ID,
// then default 'a' substitution with the original account stable suffix.
func normalizePickcode(p string) (string, error) {
	tables := map[string]string{"fa": "fumk0ytpj3sng8d5a194qh7cxlbi62zvewro", "fb": "sk732o9a1enmfpcrzbqdw4gjh6ty5xui08vl", "fc": "ywcz6hite9f4j3gup2kvdb5osal0qr1nm8x7", "fd": "on6vl0r2wpkeq9u3ts8jg7zbyc1xmhifd45a", "fe": "ljm0es2cfhwakqv6x4dgp8r1by9u7znt5io3"}
	if len(p) < 7 {
		return "", fmt.Errorf("invalid pickcode")
	}
	table, ok := tables[p[:2]]
	if !ok {
		return "", fmt.Errorf("invalid pickcode prefix")
	}
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	const target = "fuln1ytpj3smg8d5a094qh7cxkbi62zvewro"
	decoded := make([]byte, len(p)-2)
	for i, ch := range []byte(p[2:]) {
		idx := strings.IndexByte(alphabet, ch)
		if idx < 0 {
			return "", fmt.Errorf("invalid pickcode character")
		}
		decoded[i] = table[idx]
	}
	id, err := strconv.ParseUint(string(decoded[:len(decoded)-4]), 36, 64)
	if err != nil {
		return "", err
	}
	if id == 0 {
		return "", nil
	}
	plain := strconv.FormatUint(id, 36) + string(decoded[len(decoded)-4:])
	out := []byte("a")
	for _, ch := range []byte(plain) {
		idx := strings.IndexByte(target, ch)
		if idx < 0 {
			return "", fmt.Errorf("invalid pickcode substitution")
		}
		out = append(out, alphabet[idx])
	}
	return string(out), nil
}
