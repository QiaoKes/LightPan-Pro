package driver

import (
	"context"
	"litepan/internal/domain"
	"litepan/internal/store"
	"sync/atomic"
	"testing"
	"time"
)

type healthReadBarrier struct {
	domain.AuthStateRepository
	domain.CredentialHealthPatcher
	domain.AuthCredentialPatcher
	ready   chan struct{}
	release chan struct{}
	reads   atomic.Int32
}

func (r *healthReadBarrier) Get(ctx context.Context, id int64) (*domain.AuthState, error) {
	st, err := r.AuthStateRepository.Get(ctx, id)
	if err == nil && r.reads.Add(1) <= 2 {
		r.ready <- struct{}{}
		select {
		case <-r.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return st, err
}
func TestManagerConcurrentHealthPatchesPreserveBothSides(t *testing.T) {
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
	id, err := st.Accounts.Create(ctx, &domain.Account{Name: "dual", DriverType: "test", Config: "{}", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	initial := &domain.AuthState{AccountID: id, Status: domain.AuthActive, AccessToken: "access", RefreshToken: "refresh", Cookie: "cookie", CredentialGeneration: 7, CookieAuth: domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: "1"}, OpenAuth: domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: "1"}}
	if err := st.AuthStates.Upsert(ctx, initial); err != nil {
		t.Fatal(err)
	}
	barrier := &healthReadBarrier{AuthStateRepository: st.AuthStates, CredentialHealthPatcher: st.AuthStates.(domain.CredentialHealthPatcher), ready: make(chan struct{}, 2), release: make(chan struct{})}
	mgr := NewManager(st.Accounts, barrier, nil, nil)
	var invalidations atomic.Int32
	mgr.SetAuthPersistHook(func(context.Context, int64) { invalidations.Add(1) })
	generation := uint64(7)
	cookie := domain.CredentialHealth{Status: domain.AuthTokenExpired, VerifiedUID: "1", LastError: "cookie failure"}
	open := domain.CredentialHealth{Status: domain.AuthFailed, VerifiedUID: "1", LastError: "open failure"}
	results := make(chan error, 2)
	go func() {
		results <- mgr.persistAuth(ctx, id, domain.AuthCredentials{CookieAuth: &cookie, ExpectedGeneration: &generation})
	}()
	go func() {
		results <- mgr.persistAuth(ctx, id, domain.AuthCredentials{OpenAuth: &open, ExpectedGeneration: &generation})
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-barrier.ready:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	close(barrier.release)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.AuthStates.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.CookieAuth != cookie || got.OpenAuth != open || got.Cookie != "cookie" || got.AccessToken != "access" || got.RefreshToken != "refresh" || got.CredentialGeneration != 7 || invalidations.Load() != 0 {
		t.Fatalf("health patch lost state: %+v invalidations=%d", got, invalidations.Load())
	}
	got.AccessToken = "relogin"
	got.CredentialGeneration = 8
	if err := st.AuthStates.Upsert(ctx, got); err != nil {
		t.Fatal(err)
	}
	stale := domain.CredentialHealth{Status: domain.AuthActive}
	if err := mgr.persistAuth(ctx, id, domain.AuthCredentials{CookieAuth: &stale, ExpectedGeneration: &generation}); err == nil {
		t.Fatal("stale health patch accepted")
	}
	got, err = st.AuthStates.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.CookieAuth != cookie || got.AccessToken != "relogin" {
		t.Fatal("stale health patch altered newer credentials")
	}
}

func TestOpenCredentialPatchPreservesCookieHealthAfterSnapshot(t *testing.T) {
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
	id, err := st.Accounts.Create(ctx, &domain.Account{Name: "refresh", DriverType: "test", Config: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	initial := &domain.AuthState{AccountID: id, Status: domain.AuthActive, AccessToken: "old-access", RefreshToken: "old-refresh", Cookie: "unchanged-cookie", CredentialGeneration: 4, CookieAuth: domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: "1"}, OpenAuth: domain.CredentialHealth{Status: domain.AuthTokenExpired, VerifiedUID: "1"}}
	if err := st.AuthStates.Upsert(ctx, initial); err != nil {
		t.Fatal(err)
	}
	barrier := &healthReadBarrier{AuthStateRepository: st.AuthStates, CredentialHealthPatcher: st.AuthStates.(domain.CredentialHealthPatcher), AuthCredentialPatcher: st.AuthStates.(domain.AuthCredentialPatcher), ready: make(chan struct{}, 2), release: make(chan struct{})}
	mgr := NewManager(st.Accounts, barrier, nil, nil)
	generation := uint64(4)
	open := domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: "1"}
	expiry := time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	done := make(chan error, 1)
	go func() {
		done <- mgr.persistAuth(ctx, id, domain.AuthCredentials{AccessToken: "new-access", RefreshToken: "new-refresh", TokenExpires: expiry, OpenAuth: &open, ExpectedGeneration: &generation})
	}()
	select {
	case <-barrier.ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	cookie := domain.CredentialHealth{Status: domain.AuthTokenExpired, VerifiedUID: "1", LastError: "Cookie failed after refresh snapshot"}
	if err := st.AuthStates.(domain.CredentialHealthPatcher).PatchCredentialHealth(ctx, id, 4, &cookie, nil); err != nil {
		t.Fatal(err)
	}
	close(barrier.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	got, err := st.AuthStates.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.CookieAuth != cookie || got.OpenAuth != open || got.Cookie != "unchanged-cookie" || got.AccessToken != "new-access" || got.RefreshToken != "new-refresh" || !got.TokenExpires.Equal(expiry) || got.CredentialGeneration != 5 {
		t.Fatalf("Open refresh lost concurrent Cookie health: %+v", got)
	}
	if err := st.AuthStates.(domain.AuthCredentialPatcher).PatchAuthCredentials(ctx, id, 4, domain.AuthCredentials{AccessToken: "stale"}); err == nil {
		t.Fatal("stale credential patch accepted")
	}
}
