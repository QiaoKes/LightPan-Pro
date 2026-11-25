ALTER TABLE strm_tasks ADD COLUMN monitoring_enabled INTEGER NOT NULL DEFAULT 0;
CREATE TABLE strm_event_sync (
 task_id INTEGER PRIMARY KEY REFERENCES strm_tasks(id) ON DELETE CASCADE,
 state_json TEXT NOT NULL
);
