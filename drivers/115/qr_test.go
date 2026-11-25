package pan115

import (
	"context"
	"fmt"
	"io"
	"litepan/internal/driver"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type qrRoundTrip func(*http.Request) (*http.Response, error)

func (f qrRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestQRSurvivesDriverRecreationAndConsumesExchange(t *testing.T) {
	var exchanges atomic.Int32
	hc := &http.Client{Transport: qrRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"state":true,"data":{"uid":"server-uid","time":123,"sign":"secret"}}`
		switch {
		case strings.Contains(r.URL.Path, "get/status"):
			if r.URL.Query().Get("uid") != "server-uid" {
				t.Error("lost server identity")
			}
			body = `{"state":true,"data":{"status":2}}`
		case strings.Contains(r.URL.Path, "deviceCodeToToken"):
			exchanges.Add(1)
			_ = r.ParseForm()
			if len(r.Form.Get("code_verifier")) != 128 {
				t.Error("incorrect verifier")
			}
			body = `{"state":true,"data":{"access_token":"access","refresh_token":"refresh","expires_in":60}}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	d := &Driver{client: hc}
	start, err := d.StartQRLoginFor(context.Background(), driver.QRLoginOptions{Purpose: "open"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.PollQRLoginFor(context.Background(), start.Token, "cookie"); err == nil {
		t.Fatal("accepted wrong purpose")
	}
	other := &Driver{client: hc}
	got, err := other.PollQRLoginFor(context.Background(), start.Token, "open")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != driver.QRSuccess || got.Credentials.AccessToken != "access" || got.Credentials.TokenExpires.IsZero() {
		t.Fatalf("bad result: %#v", got)
	}
	if _, err = other.PollQRLoginFor(context.Background(), start.Token, "open"); err == nil {
		t.Fatal("reused consumed session")
	}
	if exchanges.Load() != 1 {
		t.Fatal("exchange repeated")
	}
}
func TestQRCancelRemovesSession(t *testing.T) {
	s := &qrSession{purpose: "cookie"}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	qrSessions.Lock()
	qrSessions.entries["cancel-test"] = s
	qrSessions.Unlock()
	d := &Driver{}
	_ = d.CancelQRLogin(context.Background(), "cancel-test", "open")
	if s.ctx.Err() != nil {
		t.Fatal("wrong purpose cancelled session")
	}
	_ = d.CancelQRLogin(context.Background(), "cancel-test", "cookie")
	if s.ctx.Err() == nil {
		t.Fatal("session not cancelled")
	}
	if _, err := d.PollQRLoginFor(context.Background(), "cancel-test", "cookie"); err == nil {
		t.Fatal("cancelled session retained")
	}
}

func TestQRUpstreamStatusesAndTransientFailure(t *testing.T) {
	for _, tc := range []struct {
		code, want string
		terminal   bool
	}{{"1", "scanned", false}, {"-1", "expired", true}, {"-2", "cancelled", true}} {
		t.Run(tc.want, func(t *testing.T) {
			var calls int
			hc := &http.Client{Transport: qrRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return nil, context.DeadlineExceeded
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"state":true,"data":{"status":` + tc.code + `}}`))}, nil
			})}
			s := &qrSession{purpose: "open", uid: "u", stamp: "1", sign: "s"}
			s.ctx, s.cancel = context.WithCancel(context.Background())
			token := "status-" + tc.want
			qrSessions.Lock()
			qrSessions.entries[token] = s
			qrSessions.Unlock()
			defer removeQR(token)
			d := &Driver{client: hc}
			if _, err := d.PollQRLoginFor(context.Background(), token, "open"); err == nil {
				t.Fatal("expected transient error")
			}
			got, err := d.PollQRLoginFor(context.Background(), token, "open")
			if err != nil {
				t.Fatal(err)
			}
			if string(got.Status) != tc.want {
				t.Fatalf("got %s", got.Status)
			}
			qrSessions.Lock()
			_, exists := qrSessions.entries[token]
			qrSessions.Unlock()
			if exists == tc.terminal {
				t.Fatal("incorrect session retention")
			}
		})
	}
}

func TestQRCapacityEvictsOldest(t *testing.T) {
	qrSessions.Lock()
	saved := qrSessions.entries
	qrSessions.entries = make(map[string]*qrSession)
	qrSessions.Unlock()
	defer func() {
		qrSessions.Lock()
		for _, s := range qrSessions.entries {
			s.cancel()
		}
		qrSessions.entries = saved
		qrSessions.Unlock()
	}()
	var first *qrSession
	for i := 0; i < 1025; i++ {
		s := &qrSession{}
		s.ctx, s.cancel = context.WithCancel(context.Background())
		if i == 0 {
			first = s
		}
		storeQR(fmt.Sprint(i), s)
	}
	if first.ctx.Err() == nil {
		t.Fatal("oldest session not cancelled")
	}
	qrSessions.Lock()
	defer qrSessions.Unlock()
	if len(qrSessions.entries) != 1024 {
		t.Fatal("capacity exceeded")
	}
	if _, ok := qrSessions.entries["1024"]; !ok {
		t.Fatal("newest session evicted")
	}
}
