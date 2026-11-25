package auth

import (
	"context"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"testing"
	"time"
)

func TestCapabilityFailureDoesNotRetryWholeAccount(t *testing.T) {
	calls := 0
	gate := &capabilityRetryGate{}
	cause := &driver.CapabilityAuthError{Capability: domain.AuthCapabilityCookieAPI, Cause: domain.Errf(domain.CodeAuthExpired)}
	err := WithRetry(context.Background(), gate, 1, func() error { calls++; return cause })
	if err != cause || calls != 1 || gate.refreshes != 0 {
		t.Fatal("capability failure retried")
	}
}

type capabilityRetryGate struct{ refreshes int }

func (g *capabilityRetryGate) Check(context.Context, int64) error { return nil }
func (g *capabilityRetryGate) HandlePassiveError(context.Context, int64) error {
	g.refreshes++
	return nil
}

func TestDegradedCapabilityGate(t *testing.T) {
	ctx := context.Background()
	svc, repo, drv, bus := newTestService(time.Now(), driver.RefreshSuccess)
	defer bus.Close(ctx)
	repo.states[1] = &domain.AuthState{AccountID: 1, Status: domain.AuthFailed, CookieAuth: domain.CredentialHealth{Status: domain.AuthTokenExpired}, OpenAuth: domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: "1"}}
	gate := &Gate{svc: svc}
	if err := gate.Check(ctx, 1); err != nil {
		t.Fatalf("degraded account blocked: %v", err)
	}
	if err := gate.CheckCapability(ctx, 1, domain.AuthCapabilityOpenAPI); err != nil {
		t.Fatal(err)
	}
	if err := gate.CheckCapability(ctx, 1, domain.AuthCapabilityCookieAPI); err == nil {
		t.Fatal("failed Cookie allowed")
	}
	if drv.calls != 0 {
		t.Fatal("gate performed an upstream refresh")
	}
}

func TestDisabledAccountBlocksDegradedGate(t *testing.T) {
	ctx := context.Background()
	svc, repo, _, bus := newTestService(time.Now(), driver.RefreshSuccess)
	defer bus.Close(ctx)
	svc.accounts.(*fakeAccountRepo).accounts[1].IsActive = false
	repo.states[1] = &domain.AuthState{AccountID: 1, Status: domain.AuthActive, OpenAuth: domain.CredentialHealth{Status: domain.AuthActive}}
	gate := &Gate{svc: svc}
	if err := gate.Check(ctx, 1); err == nil {
		t.Fatal("disabled account passed generic gate")
	}
	if err := gate.CheckCapability(ctx, 1, domain.AuthCapabilityOpenAPI); err == nil {
		t.Fatal("disabled account passed capability gate")
	}
}
func TestExpiryOnlySchedulePreservesRealExpiry(t *testing.T) {
	now := time.Now()
	expiry := now.Add(30 * time.Minute)
	cfg := driver.Config{AuthType: driver.AuthToken, RefreshAtExpiry: true, RefreshAdvance: 15 * time.Minute}
	state := &domain.AuthState{Status: domain.AuthActive, TokenExpires: expiry}
	if got := nextCheck(cfg, state, now, false); !got.Equal(expiry) {
		t.Fatalf("early refresh scheduled: %v", got)
	}
}
func TestCredentialPersistenceInvalidatesBeforeRefreshScopeReturns(t *testing.T) {
	ctx := context.Background()
	svc, _, _, bus := newTestService(time.Now(), driver.RefreshSuccess)
	defer bus.Close(ctx)
	calls := 0
	svc.SetCredentialInvalidator(func(id int64) {
		if id != 1 {
			t.Fatal("wrong account")
		}
		calls++
	})
	ctx, unlock, err := svc.lockAccount(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	svc.onCredentialsPersisted(ctx, 1)
	if calls != 1 {
		t.Fatal("scoped persistence did not synchronously invalidate playback")
	}
}

func TestTypedRefreshFailureOnlyMarksRequestedCapability(t *testing.T) {
	ctx := context.Background()
	svc, repo, _, bus := newTestService(time.Now(), driver.RefreshSuccess)
	defer bus.Close(ctx)
	repo.states[1] = &domain.AuthState{AccountID: 1, Status: domain.AuthActive, CookieAuth: domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: "1"}, OpenAuth: domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: "1"}}
	cause := &driver.CapabilityAuthError{Capability: domain.AuthCapabilityOpenAPI, Cause: domain.Errf(domain.CodeAuthExpired)}
	svc.recordRefreshFailure(ctx, 1, driver.RefreshFatal, driver.CallerActive, cause)
	got, err := repo.Get(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.AuthActive || got.CookieAuth.Status != domain.AuthActive || got.OpenAuth.Status != domain.AuthTokenExpired {
		t.Fatalf("cross-capability contamination: %+v", got)
	}
}

func TestCookieHealthFailurePreservesOpenPlaybackCache(t *testing.T) {
	ctx := context.Background()
	svc, repo, _, bus := newTestService(time.Now(), driver.RefreshSuccess)
	defer bus.Close(ctx)
	repo.states[1] = &domain.AuthState{AccountID: 1, Status: domain.AuthActive, CookieAuth: domain.CredentialHealth{Status: domain.AuthActive}, OpenAuth: domain.CredentialHealth{Status: domain.AuthActive}}
	invalidated := 0
	svc.SetCredentialInvalidator(func(int64) { invalidated++ })
	gate := &Gate{svc: svc}
	_ = gate.HandleCapabilityError(ctx, 1, domain.AuthCapabilityCookieAPI, domain.Errf(domain.CodeAuthExpired))
	if invalidated != 0 {
		t.Fatal("Cookie health failure invalidated Open playback")
	}
	if err := gate.CheckCapability(ctx, 1, domain.AuthCapabilityOpenAPI); err != nil {
		t.Fatal(err)
	}
	_ = gate.HandleCapabilityError(ctx, 1, domain.AuthCapabilityOpenAPI, domain.Errf(domain.CodeAuthExpired))
	if invalidated != 1 {
		t.Fatal("Open health failure did not invalidate playback")
	}
}
