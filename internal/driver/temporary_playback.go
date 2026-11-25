package driver

import "context"

// TemporaryPlaybackCopy identifies only the remote object created for playback.
// A nonzero handle must be retained even when creation returns a later error.
type TemporaryPlaybackCopy struct{ FileID, PickCode, Name string }
type TemporaryPlaybackCopier interface {
	CreateTemporaryPlaybackCopy(ctx context.Context, source, name string) (TemporaryPlaybackCopy, error)
	DeleteTemporaryPlaybackCopy(ctx context.Context, copy TemporaryPlaybackCopy) error
}

// TrackedTemporaryPlaybackCopier persists the discovered copy before rename.
// The callback must succeed before any further operation on the copy.
type TrackedTemporaryPlaybackCopier interface {
	CreateTrackedTemporaryPlaybackCopy(context.Context, string, string, func(TemporaryPlaybackCopy) error) (TemporaryPlaybackCopy, error)
}
