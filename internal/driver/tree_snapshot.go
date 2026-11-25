package driver

import "context"

// TreeRequest identifies a scoped export. OperationID belongs to the caller.
type TreeRequest struct {
	OperationID, RootID, RootPath string
}

// TreeEntry describes a remote leaf. Missing IDs and unknown sizes stay absent;
// callers must use path references rather than manufacture stable identities.
type TreeEntry struct {
	Path, StableID, Pickcode string
	IsDir                    bool
	Size                     *int64
}

// TreeSnapshot grants missing-leaf reconciliation only when Complete is true.
// Export trees do not inventory empty directories. Callback or cleanup errors
// must return an incomplete snapshot and cannot authorize local deletion.
type TreeSnapshot struct {
	OperationID, RootID, RootPath string
	Complete                      bool
	Warnings                      []string
}

// TreeSnapshotProvider exports a scoped inventory. Consumers stage callback
// results until successful completion, serializing application with events.
type TreeSnapshotProvider interface {
	ExportTree(context.Context, TreeRequest, func(TreeEntry) error) (TreeSnapshot, error)
}
