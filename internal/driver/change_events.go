package driver

import "context"

// LifeEventProvider exposes verified remote mutations without coupling STRM to a provider.
type LifeEventProvider interface {
	ReadLifeEvents(context.Context, LifeRequest) (LifeBatch, error)
}
type LifeRequest struct {
	Cursor    string
	SinceUnix int64
}
type LifeBatch struct {
	ThroughUnix          int64
	Events               []LifeEvent
	NextCursor           string
	Complete, HistoryGap bool
}
type LifeEvent struct {
	ID, FileID, ParentID, Path, OldPath, Pickcode string
	Kind                                          string
	UpdateUnix                                    int64
	IsDir, Verified                               bool
}

// DirectoryEventProvider expands a verified directory creation into current files.
type DirectoryEventProvider interface {
	ExpandDirectoryEvents(context.Context, string) ([]LifeEvent, error)
}
