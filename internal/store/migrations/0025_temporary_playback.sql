CREATE TABLE IF NOT EXISTS temporary_playback_copies (
 id TEXT PRIMARY KEY,
 account_id INTEGER NOT NULL,
 file_id TEXT NOT NULL,
 pick_code TEXT NOT NULL DEFAULT '',
 name TEXT NOT NULL DEFAULT '',
 due INTEGER NOT NULL,
 last_error TEXT NOT NULL DEFAULT ''
);
