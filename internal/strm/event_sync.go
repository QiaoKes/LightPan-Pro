package strm

import (
	"context"
	"errors"
	"strings"
	"sync"

	"litepan/internal/domain"
	"litepan/internal/driver"
)

var taskSyncLocks sync.Map

// WithTaskSync serializes an entire snapshot with event application for one task.
func WithTaskSync(ctx context.Context, taskID int64, fn func() error) error {
	value, _ := taskSyncLocks.LoadOrStore(taskID, make(chan struct{}, 1))
	lock := value.(chan struct{})
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return fn()
}

type EventSyncState = domain.EventSyncState

// EventSyncStore must atomically save cursor and receipts after output publication.
type EventSyncStore interface {
	LoadEventSync(context.Context, int64) (EventSyncState, error)
	SaveEventSync(context.Context, int64, EventSyncState) error
}

type EventSync struct {
	Store    EventSyncStore
	Provider driver.LifeEventProvider
	Apply    func(context.Context, driver.LifeEvent) error
}

// Poll never interprets an empty batch as deletion. The owner enables this only
// for opted-in tasks; Apply must be idempotent across a crash before SaveEventSync.
func (s *EventSync) Poll(ctx context.Context, taskID int64) error {
	return WithTaskSync(ctx, taskID, func() error {
		state, err := s.Store.LoadEventSync(ctx, taskID)
		if err != nil {
			return err
		}
		if !state.SnapshotComplete {
			return nil
		}
		if state.NeedsReconcile {
			return errors.New("event synchronization requires complete reconciliation outside task lock")
		}
		since := state.SinceUnix - 30
		if since < 0 {
			since = 0
		}
		batch, err := s.Provider.ReadLifeEvents(ctx, driver.LifeRequest{Cursor: state.Cursor, SinceUnix: since})
		if err != nil {
			state.NeedsReconcile = true
			_ = s.Store.SaveEventSync(ctx, taskID, state)
			return err
		}
		if batch.HistoryGap {
			state.NeedsReconcile = true
			return s.Store.SaveEventSync(ctx, taskID, state)
		}
		if !batch.Complete {
			state.NeedsReconcile = true
			if err = s.Store.SaveEventSync(ctx, taskID, state); err != nil {
				return err
			}
			return errors.New("incomplete life event batch")
		}
		receipts := make(map[string]int64, len(state.Receipts))
		for id, timestamp := range state.Receipts {
			receipts[id] = timestamp
		}
		state.Receipts = receipts
		paths := make(map[string]string, len(state.FilePaths))
		for id, p := range state.FilePaths {
			paths[id] = p
		}
		state.FilePaths = paths
		for _, event := range batch.Events {
			if event.UpdateUnix < since {
				continue
			}
			if _, done := state.Receipts[event.ID]; done {
				continue
			}
			if event.ID == "" || !event.Verified {
				state.NeedsReconcile = true
				if event.ID != "" {
					state.PendingReconcile = map[string]int64{event.ID: event.UpdateUnix}
				}
				if err = s.Store.SaveEventSync(ctx, taskID, state); err != nil {
					return err
				}
				return errors.New("unverified life event")
			}
			if event.FileID != "" && (event.Kind == "move" || event.Kind == "rename") {
				if old := state.FilePaths[event.FileID]; old != "" {
					event.OldPath = old
				}
			}
			if err = s.Apply(ctx, event); err != nil {
				state.NeedsReconcile = true
				state.PendingReconcile = map[string]int64{event.ID: event.UpdateUnix}
				if saveErr := s.Store.SaveEventSync(ctx, taskID, state); saveErr != nil {
					return saveErr
				}
				return err
			}
			if event.FileID != "" {
				if event.Kind == "delete" {
					delete(state.FilePaths, event.FileID)
				} else {
					base := strings.TrimRight(state.RootPath, "/")
					if base == "" || strings.HasPrefix(event.Path, base+"/") {
						state.FilePaths[event.FileID] = event.Path
					} else {
						delete(state.FilePaths, event.FileID)
					}
				}
			}
			state.Receipts[event.ID] = event.UpdateUnix
			if event.UpdateUnix > state.SinceUnix {
				state.SinceUnix = event.UpdateUnix
			}
		}
		if batch.ThroughUnix > state.SinceUnix {
			state.SinceUnix = batch.ThroughUnix
		}
		cutoff := state.SinceUnix - 30
		for id, ts := range state.Receipts {
			if ts < cutoff {
				delete(state.Receipts, id)
			}
		}
		state.Cursor = batch.NextCursor
		return s.Store.SaveEventSync(ctx, taskID, state)
	})
}
