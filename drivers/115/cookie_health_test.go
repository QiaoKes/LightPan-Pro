package pan115

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	pan115client "litepan/drivers/115/client"
	"litepan/internal/domain"
	"litepan/internal/driver"
)

func TestCookieAuthFailureIsolation(t *testing.T) {
	for _, kind := range []pan115client.ErrorKind{pan115client.AuthExpired, pan115client.VerificationRequired, pan115client.PermissionDenied, ""} {
		gen := uint64(5)
		d := &Driver{cookie: "cookie", generation: &gen, cookieHealth: domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: "123"}, openHealth: domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: "123"}}
		persisted := false
		d.persist = func(_ context.Context, c domain.AuthCredentials) error {
			persisted = true
			if c.OpenAuth != nil || c.CookieAuth == nil || *c.ExpectedGeneration != 5 {
				t.Fatal("wrong capability persistence")
			}
			return nil
		}
		err := d.cookieErrorHandler()(context.Background(), &pan115client.APIError{Kind: kind, Code: 99})
		var capability *driver.CapabilityAuthError
		want := kind == pan115client.AuthExpired || kind == pan115client.VerificationRequired
		if errors.As(err, &capability) != want || persisted != want {
			t.Fatalf("kind=%s err=%v persisted=%v", kind, err, persisted)
		}
		if d.openHealth.Status != domain.AuthActive {
			t.Fatal("Cookie failure poisoned Open")
		}
	}
}
func TestLateCookieFailureDoesNotInvalidateRelogin(t *testing.T) {
	gen := uint64(5)
	d := &Driver{cookie: "old", generation: &gen, cookieHealth: domain.CredentialHealth{Status: domain.AuthActive}}
	handle := d.cookieErrorHandler()
	next := uint64(6)
	d.SetAuthCredentials(domain.AuthCredentials{Cookie: "new", ExpectedGeneration: &next})
	_ = handle(context.Background(), &pan115client.APIError{Kind: pan115client.AuthExpired})
	if d.cookieHealth.Status != domain.AuthActive {
		t.Fatal("late failure poisoned new Cookie")
	}
}
func TestScheduledRefreshDoesNotRefreshEarly(t *testing.T) {
	d := &Driver{tokenExpires: time.Now().Add(time.Hour), client: &http.Client{Transport: fixtureTransport(func(*http.Request) (*http.Response, error) { t.Fatal("early refresh wire call"); return nil, nil })}}
	if _, err := d.RefreshAuth(context.Background(), driver.CallerActive); err != nil {
		t.Fatal(err)
	}
}
