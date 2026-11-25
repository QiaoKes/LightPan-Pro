package api

import (
	"context"
	"encoding/json"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type purposeFixture struct {
	driver.Driver
	sessions *sync.Map
}

func (p *purposeFixture) Config() driver.Config { return driver.Config{Name: "qr_purpose_fixture"} }
func (p *purposeFixture) GetAddition() any {
	return &struct {
		Required string `json:"required"`
	}{}
}
func (p *purposeFixture) Init(context.Context) error { panic("QR must not initialize an account") }
func (p *purposeFixture) Ping(context.Context) error { panic("QR must not ping an account") }
func (p *purposeFixture) Drop(context.Context) error { return nil }
func (p *purposeFixture) StartQRLoginFor(_ context.Context, o driver.QRLoginOptions) (*driver.QRStartResult, error) {
	if o.Purpose != "cookie" && o.Purpose != "open" {
		return nil, domain.Errorf(domain.CodeValidation, "invalid purpose")
	}
	if o.App != "fixture-app" || o.ClientID != "fixture-client" {
		return nil, domain.Errorf(domain.CodeValidation, "missing options")
	}
	token := "opaque-" + o.Purpose
	p.sessions.Store(token, o.Purpose)
	return &driver.QRStartResult{Token: token, QRURL: "https://example.invalid/qr"}, nil
}
func (p *purposeFixture) PollQRLoginFor(_ context.Context, token, purpose string) (*driver.QRPollResult, error) {
	got, ok := p.sessions.Load(token)
	if !ok || got != purpose {
		return nil, domain.Errorf(domain.CodeValidation, "session purpose mismatch")
	}
	expires := time.Date(2026, 11, 25, 9, 0, 0, 0, time.FixedZone("fixture", 8*3600))
	creds := domain.AuthCredentials{}
	if purpose == "cookie" {
		creds.Cookie = "fixture-cookie"
		creds.CookieExpires = expires
	} else {
		creds.AccessToken = "fixture-access"
		creds.RefreshToken = "fixture-refresh"
		creds.TokenExpires = expires
	}
	return &driver.QRPollResult{Status: driver.QRSuccess, Credentials: creds}, nil
}
func (p *purposeFixture) CancelQRLogin(_ context.Context, token, purpose string) error {
	got, ok := p.sessions.Load(token)
	if !ok || got != purpose {
		return domain.Errorf(domain.CodeValidation, "session purpose mismatch")
	}
	p.sessions.Delete(token)
	return nil
}
func TestQRPurposeHandlersWithoutAccountInitialization(t *testing.T) {
	sessions := &sync.Map{}
	driver.Register(func() driver.Driver { return &purposeFixture{sessions: sessions} })
	h := &Handler{}
	request := func(handler http.HandlerFunc, body string) (int, map[string]json.RawMessage) {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/qr", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		handler(w, r)
		var envelope struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return w.Code, envelope.Data
	}
	for _, purpose := range []string{"cookie", "open"} {
		body := `{"driver_type":"qr_purpose_fixture","purpose":"` + purpose + `","app":"fixture-app","client_id":"fixture-client"}`
		code, data := request(h.startQRLogin, body)
		if code != http.StatusOK {
			t.Fatalf("start: %d", code)
		}
		var token string
		if err := json.Unmarshal(data["token"], &token); err != nil {
			t.Fatal(err)
		}
		if string(data["purpose"]) != `"`+purpose+`"` {
			t.Fatal("purpose not echoed")
		}
		poll := `{"driver_type":"qr_purpose_fixture","purpose":"` + purpose + `","token":"` + token + `"}`
		code, data = request(h.pollQRLogin, poll)
		if code != http.StatusOK {
			t.Fatalf("poll: %d", code)
		}
		field := "token_expires"
		if purpose == "cookie" {
			field = "cookie_expires"
		}
		if string(data[field]) != `"2026-11-25T01:00:00Z"` {
			t.Fatalf("RFC3339 expiry: %s", data[field])
		}
		wrong := strings.Replace(poll, `"purpose":"`+purpose+`"`, `"purpose":"other"`, 1)
		code, _ = request(h.pollQRLogin, wrong)
		if code == http.StatusOK {
			t.Fatal("cross-purpose poll accepted")
		}
		code, _ = request(h.cancelQRLogin, wrong)
		if code == http.StatusOK {
			t.Fatal("cross-purpose cancel accepted")
		}
		code, _ = request(h.cancelQRLogin, poll)
		if code != http.StatusOK {
			t.Fatalf("cancel: %d", code)
		}
		code, _ = request(h.pollQRLogin, poll)
		if code == http.StatusOK {
			t.Fatal("cancelled session polled")
		}
	}
}
