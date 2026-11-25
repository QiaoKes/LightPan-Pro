package account

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	_ "litepan/drivers/LocalFs"
	"litepan/internal/auth"
	"litepan/internal/core/driverexec"
	"litepan/internal/domain"
	"litepan/internal/driver"
	"litepan/internal/store"
)

// Exercise the registered credential-free provider through the same account,
// persistence, auth guards and executor boundaries used by production wiring.
func TestLocalFsMountCreateAndListingWithoutDualCredentials(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	const filename = "local movie.mkv"
	const content = "local mount regression"
	if err := os.WriteFile(filepath.Join(root, filename), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(ctx, store.Options{Path: filepath.Join(t.TempDir(), "accounts.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	st := store.New(db)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := driver.NewManager(st.Accounts, st.AuthStates, st.Configs, log)
	t.Cleanup(func() { manager.Close(ctx) })
	authSvc := auth.NewService(auth.Options{Accounts: st.Accounts, AuthStates: st.AuthStates, Drivers: manager, Log: log})
	svc := NewService(Options{Accounts: st.Accounts, AuthStates: st.AuthStates, Drivers: manager, Auth: authSvc})
	config, err := json.Marshal(map[string]string{"root_path": root})
	if err != nil {
		t.Fatal(err)
	}
	view, err := svc.Create(ctx, Input{Name: "local mount", DriverType: "localfs", Config: string(config), IsActive: true})
	if err != nil {
		t.Fatalf("credential-free registered LocalFs mount rejected: %v", err)
	}
	if view.Account.ID == 0 || view.Account.DriverType != "localfs" || !view.Account.IsActive || view.CredentialHealth != "" || len(view.CredentialStatuses) != 0 {
		t.Fatalf("LocalFs treated as a dual-credential account: %+v", view)
	}
	var saved map[string]string
	if err := json.Unmarshal([]byte(view.Config), &saved); err != nil {
		t.Fatal(err)
	}
	if saved["root_path"] != root {
		t.Fatal("configured local root was not persisted")
	}
	exec := driverexec.New(manager, authSvc.Gate())
	checkListing := func(exec *driverexec.Executor) {
		t.Helper()
		var items []domain.FileItem
		if err := exec.Run(ctx, view.Account.ID, func(d driver.Driver) error { var err error; items, err = d.ListFiles(ctx, "/"); return err }); err != nil {
			t.Fatalf("LocalFs initialization/listing blocked by auth: %v", err)
		}
		if len(items) != 1 || items[0].Name != filename || items[0].IsDir || items[0].Size != int64(len(content)) || items[0].IDKind != domain.IDPath {
			t.Fatalf("wrong configured-root listing: %+v", items)
		}
	}
	checkListing(exec)
	// Rebuild from persisted configuration, without adding credentials or health.
	manager.Close(ctx)
	reloaded := driver.NewManager(st.Accounts, st.AuthStates, st.Configs, log)
	t.Cleanup(func() { reloaded.Close(ctx) })
	reloadedAuth := auth.NewService(auth.Options{Accounts: st.Accounts, AuthStates: st.AuthStates, Drivers: reloaded, Log: log})
	checkListing(driverexec.New(reloaded, reloadedAuth.Gate()))
	if _, err := st.AuthStates.Get(ctx, view.Account.ID); err == nil {
		t.Fatal("credential-free mount unexpectedly created auth credentials")
	} else if ae, ok := domain.AsAppError(err); !ok || ae.Code != domain.CodeNotFound {
		t.Fatal(err)
	}
	// A real local initialization failure must still reject the mount before save.
	invalidConfig, err := json.Marshal(map[string]string{"root_path": filepath.Join(root, filename)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, Input{Name: "invalid local mount", DriverType: "localfs", Config: string(invalidConfig), IsActive: true}); err == nil {
		t.Fatal("file path accepted as a local mount directory")
	}
	accounts, err := st.Accounts.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("failed local initialization persisted an account: %+v", accounts)
	}
}
