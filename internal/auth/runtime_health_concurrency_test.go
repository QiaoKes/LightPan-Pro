package auth

import (
	"context"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"litepan/internal/store"
	"testing"
	"time"
)

type runtimeWriteBarrier struct {
	domain.AuthStateRepository
	runtime domain.AuthRuntimeUpdater
	ready   chan struct{}
	release chan struct{}
}

func (r *runtimeWriteBarrier) UpsertAuthRuntime(ctx context.Context, s *domain.AuthState) error {
	close(r.ready)
	select {
	case <-r.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	return r.runtime.UpsertAuthRuntime(ctx, s)
}
func TestRuntimeBookkeepingPreservesConcurrentHealthPatch(t *testing.T) {
	for _, operation := range []string{"success", "failure"} {
		t.Run(operation, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			db, err := store.Open(ctx, store.Options{Memory: true})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			st := store.New(db)
			id, err := st.Accounts.Create(ctx, &domain.Account{Name: "runtime", DriverType: "test", Config: "{}", IsActive: true})
			if err != nil {
				t.Fatal(err)
			}
			snapshot := &domain.AuthState{AccountID: id, Status: domain.AuthActive, AccessToken: "access", Cookie: "cookie", CredentialGeneration: 4, OpenAuth: domain.CredentialHealth{Status: domain.AuthActive}, CookieAuth: domain.CredentialHealth{Status: domain.AuthActive}}
			if err := st.AuthStates.Upsert(ctx, snapshot); err != nil {
				t.Fatal(err)
			}
			wrapper := &runtimeWriteBarrier{AuthStateRepository: st.AuthStates, runtime: st.AuthStates.(domain.AuthRuntimeUpdater), ready: make(chan struct{}), release: make(chan struct{})}
			svc := NewService(Options{AuthStates: wrapper, Now: time.Now})
			done := make(chan error, 1)
			go func() {
				if operation == "success" {
					done <- svc.markSuccess(ctx, id, snapshot, true)
				} else {
					svc.handleFailure(ctx, id, snapshot, driver.RefreshRetryable, driver.CallerActive, domain.Errorf(domain.CodeDriverError, "upstream unavailable"))
					done <- nil
				}
			}()
			select {
			case <-wrapper.ready:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			cookie := domain.CredentialHealth{Status: domain.AuthTokenExpired, LastError: "concurrent cookie failure"}
			if err := st.AuthStates.(domain.CredentialHealthPatcher).PatchCredentialHealth(ctx, id, 4, &cookie, nil); err != nil {
				t.Fatal(err)
			}
			close(wrapper.release)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			got, err := st.AuthStates.Get(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if got.CookieAuth != cookie || got.OpenAuth.Status != domain.AuthActive || got.AccessToken != "access" || got.Cookie != "cookie" {
				t.Fatalf("runtime bookkeeping overwrote concurrent observation: %+v", got)
			}
			if operation == "success" && got.LastRefreshAt.IsZero() {
				t.Fatal("success bookkeeping missing")
			}
			if operation == "failure" && got.LastError == "" {
				t.Fatal("failure bookkeeping missing")
			}
			// A reconnect after snapshot load must also reject stale runtime bookkeeping.
			newer := *got
			newer.CredentialGeneration = 5
			newer.AccessToken = "reconnected"
			if err := st.AuthStates.Upsert(ctx, &newer); err != nil {
				t.Fatal(err)
			}
			if err := st.AuthStates.(domain.AuthRuntimeUpdater).UpsertAuthRuntime(ctx, snapshot); err == nil {
				t.Fatal("stale runtime snapshot accepted")
			}
			got, err = st.AuthStates.Get(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if got.AccessToken != "reconnected" || got.CookieAuth != cookie {
				t.Fatal("stale runtime snapshot overwrote reconnect")
			}
		})
	}
}

func TestRuntimeBookkeepingPreservesExpiryOnlyCredentialPatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := store.Open(ctx, store.Options{Memory: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	id, err := st.Accounts.Create(ctx, &domain.Account{Name: "expiry", DriverType: "test", Config: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	oldExpiry := time.Now().UTC().Truncate(time.Second)
	snapshot := &domain.AuthState{AccountID: id, Status: domain.AuthActive, AccessToken: "unchanged", Cookie: "unchanged-cookie", TokenExpires: oldExpiry, CookieExpires: oldExpiry, CredentialGeneration: 4, OpenAuth: domain.CredentialHealth{Status: domain.AuthActive}, CookieAuth: domain.CredentialHealth{Status: domain.AuthActive}}
	if err := st.AuthStates.Upsert(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	wrapper := &runtimeWriteBarrier{AuthStateRepository: st.AuthStates, runtime: st.AuthStates.(domain.AuthRuntimeUpdater), ready: make(chan struct{}), release: make(chan struct{})}
	svc := NewService(Options{AuthStates: wrapper, Now: time.Now})
	done := make(chan error, 1)
	go func() { done <- svc.markSuccess(ctx, id, snapshot, true) }()
	select {
	case <-wrapper.ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	tokenExpiry, cookieExpiry := oldExpiry.Add(2*time.Hour), oldExpiry.Add(3*time.Hour)
	if err := st.AuthStates.(domain.AuthCredentialPatcher).PatchAuthCredentials(ctx, id, 4, domain.AuthCredentials{AccessToken: "unchanged", Cookie: "unchanged-cookie", TokenExpires: tokenExpiry, CookieExpires: cookieExpiry}); err != nil {
		t.Fatal(err)
	}
	close(wrapper.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got, err := st.AuthStates.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.TokenExpires.Equal(tokenExpiry) || !got.CookieExpires.Equal(cookieExpiry) || got.CredentialGeneration != 4 || got.LastRefreshAt.IsZero() {
		t.Fatalf("bookkeeping rolled back same-generation expiry patch: %+v", got)
	}
}
