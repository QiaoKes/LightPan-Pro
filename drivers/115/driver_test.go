package pan115

import (
	"context"
	"io"
	"litepan/internal/domain"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPingRequiresLiveCookieAndMatchingOpenIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, cookieResponse, openUID string
		wantErr                       bool
	}{
		{"matching", `{"state":true}`, "123", false},
		{"expired", `{"state":false}`, "123", true},
		{"mismatch", `{"state":true}`, "456", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			d := &Driver{token: "access", refresh: "refresh", cookie: "UID=123_A1; CID=c; SEID=s"}
			d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				body := tc.cookieResponse
				if r.URL.Host == "my.115.com" {
					if r.Header.Get("Cookie") == "" {
						t.Fatal("missing Cookie")
					}
					if r.URL.Query().Get("ac") != "status" {
						t.Fatal("wrong live validation")
					}
				} else {
					body = `{"state":true,"data":{"user_id":"` + tc.openUID + `"}}`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			openBucket.Lock()
			openBucket.last = time.Time{}
			openBucket.Unlock()
			err := d.Ping(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v", err)
			}
			if tc.name == "expired" && calls != 1 {
				t.Fatalf("invalid Cookie continued to Open: %d", calls)
			}
			if !tc.wantErr && d.CredentialHealth()[domain.AuthCapabilityOpenAPI].VerifiedUID != "123" {
				t.Fatal("identity not verified")
			}
		})
	}
}
func TestOpenAdmissionHonorsCancellation(t *testing.T) {
	openBucket.Lock()
	openBucket.last = time.Now()
	openBucket.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := acquireOpen(ctx); err != context.Canceled {
		t.Fatalf("error=%v", err)
	}
}
func TestCookieFailureDoesNotBlockOpenCapability(t *testing.T) {
	d := &Driver{cookieHealth: domain.CredentialHealth{Status: domain.AuthFailed}, openHealth: domain.CredentialHealth{Status: domain.AuthActive}}
	if err := d.requireCapability(domain.AuthCapabilityOpenAPI); err != nil {
		t.Fatal(err)
	}
	if err := d.requireCapability(domain.AuthCapabilityCookieAPI); err == nil {
		t.Fatal("Cookie admitted")
	}
}

func TestReconnectProbesOnlyChangedSide(t *testing.T) {
	for _, cookieChange := range []bool{true, false} {
		ch := domain.CredentialHealth{Status: domain.AuthTokenExpired, VerifiedUID: "123"}
		oh := ch
		previous := domain.AuthCredentials{Cookie: "UID=123_old", AccessToken: "old", RefreshToken: "refresh", CookieAuth: &ch, OpenAuth: &oh}
		d := &Driver{cookie: previous.Cookie, token: previous.AccessToken, refresh: previous.RefreshToken}
		if cookieChange {
			d.cookie = "UID=123_new"
		} else {
			d.token = "new"
		}
		calls := 0
		d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			body := `{"state":true}`
			if cookieChange {
				if r.URL.Host != "my.115.com" {
					t.Fatal("probed retained expired Open")
				}
			} else {
				if r.URL.Host != "proapi.115.com" {
					t.Fatal("probed retained expired Cookie")
				}
				body = `{"state":true,"data":{"user_id":"123"}}`
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		if err := d.ValidateCredentialUpdate(context.Background(), previous); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("calls=%d", calls)
		}
		health := d.CredentialHealth()
		if cookieChange && health[domain.AuthCapabilityOpenAPI].Status != domain.AuthTokenExpired {
			t.Fatal("retained Open health reset")
		}
		if !cookieChange && health[domain.AuthCapabilityCookieAPI].Status != domain.AuthTokenExpired {
			t.Fatal("retained Cookie health reset")
		}
	}
}
func TestRefreshDiscardsChangedGenerationWithSameRefreshToken(t *testing.T) {
	gen := uint64(3)
	d := &Driver{token: "old", refresh: "same", generation: &gen}
	entered, release := make(chan struct{}), make(chan struct{})
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"state":1,"data":{"access_token":"stale","refresh_token":"same"}}`))}, nil
	})}
	persisted := false
	d.persist = func(context.Context, domain.AuthCredentials) error { persisted = true; return nil }
	done := make(chan error, 1)
	go func() { _, err := d.exchangeToken(context.Background()); done <- err }()
	<-entered
	newer := uint64(4)
	d.SetAuthCredentials(domain.AuthCredentials{AccessToken: "relogin", RefreshToken: "same", ExpectedGeneration: &newer})
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if persisted || d.currentToken() != "relogin" {
		t.Fatal("stale refresh overwrote relogin")
	}
}
func TestRefreshPersistenceFailureKeepsLocalCredentials(t *testing.T) {
	d := &Driver{token: "old", refresh: "old-refresh"}
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"state":1,"data":{"access_token":"new","refresh_token":"new-refresh"}}`))}, nil
	})}
	d.persist = func(context.Context, domain.AuthCredentials) error { return context.Canceled }
	if _, err := d.exchangeToken(context.Background()); err == nil {
		t.Fatal("expected persistence error")
	}
	if d.currentToken() != "old" {
		t.Fatal("unpersisted refresh became active locally")
	}
}

func TestRefreshAdvancesGenerationForNextPersistence(t *testing.T) {
	gen := uint64(8)
	d := &Driver{token: "old", refresh: "old-refresh", generation: &gen}
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"state":1,"data":{"access_token":"new","refresh_token":"new-refresh","expires_in":3600}}`))}, nil
	})}
	calls := 0
	d.persist = func(_ context.Context, c domain.AuthCredentials) error {
		expected := uint64(8)
		if calls > 0 {
			expected = 9
		}
		if c.ExpectedGeneration == nil || *c.ExpectedGeneration != expected {
			t.Fatalf("unexpected generation: %v", c.ExpectedGeneration)
		}
		calls++
		return nil
	}
	for i := 0; i < 2; i++ {
		if _, err := d.exchangeToken(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if *d.generation != 9 {
		t.Fatalf("generation=%d", *d.generation)
	}
}
func TestReconnectRejectsOtherIdentity(t *testing.T) {
	previousHealth := domain.CredentialHealth{Status: domain.AuthTokenExpired, VerifiedUID: "123"}
	d := &Driver{cookie: "UID=456_new", token: "old", refresh: "refresh"}
	d.client = &http.Client{Transport: fixtureTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"state":true}`))}, nil
	})}
	if err := d.ValidateCredentialUpdate(context.Background(), domain.AuthCredentials{Cookie: "UID=123_old", AccessToken: "old", RefreshToken: "refresh", CookieAuth: &previousHealth, OpenAuth: &previousHealth}); err == nil {
		t.Fatal("accepted mismatched reconnect")
	}
}
