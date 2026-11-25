package domain

type EventSyncState struct {
	FilePaths        map[string]string `json:",omitempty"`
	RootPath         string
	PendingReconcile map[string]int64
	Cursor           string
	SinceUnix        int64
	SnapshotComplete bool
	NeedsReconcile   bool
	Receipts         map[string]int64
}
