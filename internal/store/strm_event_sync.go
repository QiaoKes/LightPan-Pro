package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"litepan/internal/domain"
)

// LoadEventSync returns an uninitialized state until a complete snapshot exists.
func (db *DB) LoadEventSync(ctx context.Context, taskID int64) (domain.EventSyncState, error) {
	var raw string
	err := db.read.QueryRowContext(ctx, `SELECT state_json FROM strm_event_sync WHERE task_id=?`, taskID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.EventSyncState{}, nil
	}
	if err != nil {
		return domain.EventSyncState{}, wrapDB(err)
	}
	var state domain.EventSyncState
	err = json.Unmarshal([]byte(raw), &state)
	return state, err
}
func (db *DB) SaveEventSync(ctx context.Context, taskID int64, state domain.EventSyncState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = db.write.ExecContext(ctx, `INSERT INTO strm_event_sync(task_id,state_json) VALUES(?,?) ON CONFLICT(task_id) DO UPDATE SET state_json=excluded.state_json`, taskID, string(raw))
	return wrapDB(err)
}
