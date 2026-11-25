package domain

import (
	"context"
	"time"
)

type TemporaryPlaybackRecord struct {
	ID        string
	AccountID int64
	FileID    string
	PickCode  string
	Name      string
	Due       time.Time
	LastError string
}
type TemporaryPlaybackRepository interface {
	Save(context.Context, TemporaryPlaybackRecord) error
	Pending(context.Context) ([]TemporaryPlaybackRecord, error)
	Delete(context.Context, string) error
}
