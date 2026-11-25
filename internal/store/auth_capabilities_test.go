package store_test

import (
	"context"
	"litepan/internal/domain"
	"testing"
	"time"
)

func TestCredentialHealthRoundTripAndCAS(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id, err := s.Accounts.Create(ctx, &domain.Account{Name: "dual", DriverType: "test", Config: "{}", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	state := &domain.AuthState{AccountID: id, Status: domain.AuthActive, AccessToken: "open", Cookie: "cookie", CredentialGeneration: 2, CookieAuth: domain.CredentialHealth{Status: domain.AuthFailed, VerifiedUID: "1"}, OpenAuth: domain.CredentialHealth{Status: domain.AuthActive, VerifiedUID: "1"}}
	if err := s.AuthStates.Upsert(ctx, state); err != nil {
		t.Fatal(err)
	}
	got, err := s.AuthStates.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.CookieAuth != state.CookieAuth || got.OpenAuth != state.OpenAuth || got.CredentialGeneration != 2 {
		t.Fatalf("roundtrip failed: %+v", got)
	}
	repo := s.AuthStates.(interface {
		UpsertGeneration(context.Context, *domain.AuthState, uint64) error
	})
	got.Cookie = "reconnected"
	got.CredentialGeneration = 3
	if err := repo.UpsertGeneration(ctx, got, 2); err != nil {
		t.Fatal(err)
	}
	state.AccessToken = "stale"
	if err := repo.UpsertGeneration(ctx, state, 2); err == nil {
		t.Fatal("stale CAS accepted")
	}
	latest, err := s.AuthStates.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Cookie != "reconnected" || latest.AccessToken != "open" {
		t.Fatal("stale write overwrote credentials")
	}
}

func TestTaskMetadataModeStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	aid, err := s.Accounts.Create(ctx, &domain.Account{Name: "metadata", DriverType: "115", Config: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.StrmTasks.Create(ctx, &domain.StrmTask{Name: "metadata", AccountID: aid, MetadataSyncMode: "cloud_primary"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.StrmTasks.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if task.MetadataSyncMode != "cloud_primary" {
		t.Fatal("mode not persisted")
	}
	task.MetadataSyncMode = "bidirectional"
	if err := s.StrmTasks.Update(ctx, task); err != nil {
		t.Fatal(err)
	}
	tasks, err := s.StrmTasks.ListByAccount(ctx, aid)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].MetadataSyncMode != "bidirectional" {
		t.Fatal("updated mode not persisted")
	}
}

func TestLegacyRuntimeStillUpdatesScheduledExpiry(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	id, err := s.Accounts.Create(ctx, &domain.Account{Name: "legacy-expiry", DriverType: "legacy", Config: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	state := &domain.AuthState{AccountID: id, Status: domain.AuthActive, AccessToken: "legacy"}
	if err := s.AuthStates.Upsert(ctx, state); err != nil {
		t.Fatal(err)
	}
	state.TokenExpires = time.Now().UTC().Truncate(time.Second).Add(time.Hour)
	if err := s.AuthStates.(domain.AuthRuntimeUpdater).UpsertAuthRuntime(ctx, state); err != nil {
		t.Fatal(err)
	}
	got, err := s.AuthStates.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.TokenExpires.Equal(state.TokenExpires) {
		t.Fatal("legacy inferred schedule no longer persists")
	}
}
