package pan115

import (
	"context"
	"fmt"
	pan115client "litepan/drivers/115/client"
	"litepan/internal/domain"
	"litepan/internal/driver"
)

func (d *Driver) ExportTree(ctx context.Context, req driver.TreeRequest, emit func(driver.TreeEntry) error) (driver.TreeSnapshot, error) {
	snapshot := driver.TreeSnapshot{OperationID: req.OperationID, RootID: req.RootID, RootPath: req.RootPath}
	if err := d.requireCapability(domain.AuthCapabilityCookieAPI); err != nil {
		return snapshot, err
	}
	d.mu.Lock()
	cookie := d.cookie
	ch, oh := d.cookieHealth, d.openHealth
	d.mu.Unlock()
	if cookie == "" || ch.VerifiedUID == "" || oh.VerifiedUID == "" || ch.VerifiedUID != oh.VerifiedUID {
		return snapshot, fmt.Errorf("tree export requires verified matching Cookie/Open identity")
	}
	absolute, err := d.absoluteTreeRoot(ctx, req.RootPath)
	if err != nil {
		return snapshot, err
	}
	snapshot.RootPath = absolute
	handleCookieError := d.cookieErrorHandler()
	paths, complete, err := pan115client.New(d.client, cookie).ExportTree(ctx, absolute)
	if err != nil {
		return snapshot, handleCookieError(ctx, err)
	}
	for _, p := range paths {
		if err = ctx.Err(); err != nil {
			return snapshot, err
		}
		if err = emit(driver.TreeEntry{Path: p}); err != nil {
			return snapshot, err
		}
	}
	snapshot.Complete = complete
	if !complete {
		snapshot.Warnings = append(snapshot.Warnings, "root listing was truncated; missing-output reconciliation prohibited")
	}
	return snapshot, nil
}

var _ driver.TreeSnapshotProvider = (*Driver)(nil)
