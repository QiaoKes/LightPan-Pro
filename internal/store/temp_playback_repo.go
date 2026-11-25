package store

import (
	"context"
	"litepan/internal/domain"
	"time"
)

type temporaryPlaybackRepo struct{ db *DB }

func (r *temporaryPlaybackRepo) Save(ctx context.Context, v domain.TemporaryPlaybackRecord) error {
	_, err := r.db.write.ExecContext(ctx, `INSERT INTO temporary_playback_copies(id,account_id,file_id,pick_code,name,due,last_error) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET file_id=excluded.file_id,pick_code=excluded.pick_code,name=excluded.name,due=excluded.due,last_error=excluded.last_error`, v.ID, v.AccountID, v.FileID, v.PickCode, v.Name, v.Due.UnixMilli(), v.LastError)
	return wrapDB(err)
}
func (r *temporaryPlaybackRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.write.ExecContext(ctx, `DELETE FROM temporary_playback_copies WHERE id=?`, id)
	return wrapDB(err)
}
func (r *temporaryPlaybackRepo) Pending(ctx context.Context) ([]domain.TemporaryPlaybackRecord, error) {
	rows, err := r.db.read.QueryContext(ctx, `SELECT id,account_id,file_id,pick_code,name,due,last_error FROM temporary_playback_copies`)
	if err != nil {
		return nil, wrapDB(err)
	}
	defer rows.Close()
	var out []domain.TemporaryPlaybackRecord
	for rows.Next() {
		var v domain.TemporaryPlaybackRecord
		var due int64
		if err := rows.Scan(&v.ID, &v.AccountID, &v.FileID, &v.PickCode, &v.Name, &due, &v.LastError); err != nil {
			return nil, wrapDB(err)
		}
		v.Due = time.UnixMilli(due)
		out = append(out, v)
	}
	return out, wrapDB(rows.Err())
}
