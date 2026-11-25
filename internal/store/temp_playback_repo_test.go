package store_test

import (
	"context"
	"litepan/internal/domain"
	"litepan/internal/store"
	"path/filepath"
	"testing"
	"time"
)

func TestTemporaryPlaybackOwnershipSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "copies.db")
	db, err := store.Open(ctx, store.Options{Path: file})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	original := domain.TemporaryPlaybackRecord{ID: "request-one", AccountID: 42, FileID: "owned-copy", Due: time.Now().Add(-time.Second), LastError: "retry cleanup"}
	if err = store.New(db).TemporaryPlayback.Save(ctx, original); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = store.Open(ctx, store.Options{Path: file})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := store.New(db).TemporaryPlayback
	records, err := repo.Pending(ctx)
	if err != nil || len(records) != 1 {
		t.Fatalf("records=%v err=%v", records, err)
	}
	if records[0].FileID != original.FileID || records[0].LastError != original.LastError {
		t.Fatalf("record=%+v", records[0])
	}
	if err = repo.Delete(ctx, original.ID); err != nil {
		t.Fatal(err)
	}
	records, err = repo.Pending(ctx)
	if err != nil || len(records) != 0 {
		t.Fatalf("records=%v err=%v", records, err)
	}
}
