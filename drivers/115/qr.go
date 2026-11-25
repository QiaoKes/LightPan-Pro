package pan115

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	pan115client "litepan/drivers/115/client"
	"litepan/internal/domain"
	"litepan/internal/driver"
)

func qrData(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var envelope struct {
		Data  map[string]json.RawMessage `json:"data"`
		State *bool                      `json:"state"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if envelope.State != nil && !*envelope.State {
		return nil, errors.New("115 QR request rejected")
	}
	return envelope.Data, nil
}
func qrString(m map[string]json.RawMessage, k string) string {
	var s string
	if json.Unmarshal(m[k], &s) == nil {
		return s
	}
	return string(m[k])
}
func (d *Driver) StartQRLogin(ctx context.Context) (*driver.QRStartResult, error) {
	return d.StartQRLoginFor(ctx, driver.QRLoginOptions{Purpose: "open"})
}
func (d *Driver) PollQRLogin(ctx context.Context, token string) (*driver.QRPollResult, error) {
	return d.PollQRLoginFor(ctx, token, "open")
}
func (d *Driver) StartQRLoginFor(ctx context.Context, o driver.QRLoginOptions) (*driver.QRStartResult, error) {
	if o.Purpose != "cookie" && o.Purpose != "open" {
		return nil, errors.New("QR purpose must be cookie or open")
	}
	if o.App == "" {
		o.App = "qandroid"
	}
	if o.ClientID == "" {
		o.ClientID = "100197641"
	}
	s := &qrSession{purpose: o.Purpose, app: o.App, clientID: o.ClientID}
	c := pan115client.New(d.client, "")
	var raw json.RawMessage
	var err error
	if o.Purpose == "cookie" {
		raw, err = c.CookieQRToken(ctx, o.App)
	} else {
		s.verifier, err = qrRandom(96)
		if err == nil {
			raw, err = c.OpenQRToken(ctx, o.ClientID, s.verifier)
		}
	}
	if err != nil {
		return nil, err
	}
	data, err := qrData(raw)
	if err != nil {
		return nil, err
	}
	s.uid = qrString(data, "uid")
	s.stamp = qrString(data, "time")
	s.sign = qrString(data, "sign")
	if s.uid == "" || s.sign == "" || s.stamp == "" {
		return nil, errors.New("115 QR response missing session fields")
	}
	token, err := qrRandom(32)
	if err != nil {
		return nil, err
	}
	// The HTTP start request ends before subsequent polls. Session cancellation is explicit.
	s.ctx, s.cancel = context.WithCancel(context.Background())
	storeQR(token, s)
	return &driver.QRStartResult{Token: token, QRURL: "https://qrcodeapi.115.com/api/1.0/web/1.0/qrcode?uid=" + url.QueryEscape(s.uid), Title: "115 授权", Hint: "请扫码并确认授权"}, nil
}
func (d *Driver) CancelQRLogin(ctx context.Context, token, purpose string) error {
	qrSessions.Lock()
	s := qrSessions.entries[token]
	if s != nil && s.purpose == purpose {
		delete(qrSessions.entries, token)
	}
	qrSessions.Unlock()
	if s != nil && s.purpose == purpose {
		s.cancel()
	}
	return nil
}
func (d *Driver) PollQRLoginFor(ctx context.Context, token, purpose string) (*driver.QRPollResult, error) {
	qrSessions.Lock()
	s := qrSessions.entries[token]
	qrSessions.Unlock()
	if s == nil || s.purpose != purpose {
		return nil, errors.New("unknown QR session")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if s.purpose == "cookie" {
		if s.attempts >= 5 {
			removeQR(token)
			return &driver.QRPollResult{Status: driver.QRFailed, Message: "QR polling attempts exhausted"}, nil
		}
		if wait := time.Until(s.next); wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-call.Done():
				removeQR(token)
				return nil, call.Err()
			case <-timer.C:
			}
		}
	}
	c := pan115client.New(d.client, "")
	var raw json.RawMessage
	var err error
	if s.purpose == "open" {
		raw, err = c.OpenQRStatus(call, s.uid, s.stamp, s.sign)
	} else {
		raw, err = c.QRStatus(call, s.uid, s.stamp, s.sign)
	}
	if err != nil {
		// A transient polling failure does not invalidate the upstream QR token.
		return nil, err
	}
	s.attempts++
	data, err := qrData(raw)
	if err != nil {
		removeQR(token)
		return nil, err
	}
	if len(data) == 0 {
		removeQR(token)
		return &driver.QRPollResult{Status: driver.QRFailed, Message: "QR status unavailable"}, nil
	}
	status := qrString(data, "status")
	if status == "-1" || status == "-2" {
		removeQR(token)
		if status == "-1" {
			return &driver.QRPollResult{Status: driver.QRExpired, Message: "二维码已过期"}, nil
		}
		return &driver.QRPollResult{Status: driver.QRStatus("cancelled"), Message: "授权已取消"}, nil
	}
	if status != "2" {
		s.next = time.Now().Add(7 * time.Second)
		if status == "1" {
			return &driver.QRPollResult{Status: driver.QRStatus("scanned"), Message: "已扫码，请确认授权"}, nil
		}
		return &driver.QRPollResult{Status: driver.QRWaiting}, nil
	}
	defer removeQR(token)
	creds := domain.AuthCredentials{}
	if purpose == "cookie" {
		creds.Cookie, _, err = c.CookieQRExchange(call, s.app, s.uid)
	} else {
		raw, err = c.OpenQRExchange(call, s.uid, s.verifier)
		if err == nil {
			data, err = qrData(raw)
			if err == nil {
				creds.AccessToken = qrString(data, "access_token")
				creds.RefreshToken = qrString(data, "refresh_token")
				var seconds int64
				_ = json.Unmarshal(data["expires_in"], &seconds)
				if seconds > 0 {
					creds.TokenExpires = time.Now().Add(time.Duration(seconds) * time.Second)
				}
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if creds.Cookie == "" && creds.AccessToken == "" {
		return nil, errors.New("115 QR exchange returned no credentials")
	}
	return &driver.QRPollResult{Status: driver.QRSuccess, Credentials: creds}, nil
}

var _ driver.QRLoginProvider = (*Driver)(nil)
var _ driver.PurposeQRLoginProvider = (*Driver)(nil)
