package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// configureSQLite sets pragmas and pool limits for SQLite.
// Without busy_timeout (and a single pooled connection), concurrent handlers can hit SQLITE_BUSY (5) easily.
func configureSQLite(d *sql.DB) error {
	// Single connection avoids writer lock contention across pooled connections.
	d.SetMaxOpenConns(1)
	d.SetMaxIdleConns(1)
	d.SetConnMaxLifetime(0)

	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 10000`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	return nil
}

const schema = `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS projects (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS collections (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  field_schema_json TEXT,
  UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS labels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS audio_files (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  stored_filename TEXT NOT NULL,
  original_name TEXT NOT NULL,
  format TEXT NOT NULL,
  duration_ms INTEGER,
  uploaded_at TEXT NOT NULL,
  field_values_json TEXT DEFAULT '{}',
  UNIQUE(collection_id, stored_filename)
);

CREATE TABLE IF NOT EXISTS segments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  audio_file_id INTEGER NOT NULL REFERENCES audio_files(id) ON DELETE CASCADE,
  start_ms INTEGER NOT NULL,
  end_ms INTEGER NOT NULL,
  label_id INTEGER REFERENCES labels(id),
  transcription TEXT,
  field_values_json TEXT
);

CREATE TABLE IF NOT EXISTS datasets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  options_json TEXT NOT NULL,
  storage_root TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS dataset_samples (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  dataset_id INTEGER NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
  split TEXT NOT NULL,
  filename TEXT NOT NULL,
  rel_path TEXT NOT NULL,
  label TEXT NOT NULL,
  transcription TEXT,
  source_segment_id INTEGER NOT NULL,
  augmentation_json TEXT
);

CREATE INDEX IF NOT EXISTS idx_collections_project ON collections(project_id);
CREATE INDEX IF NOT EXISTS idx_audio_collection ON audio_files(collection_id);
CREATE INDEX IF NOT EXISTS idx_segments_audio ON segments(audio_file_id);
CREATE INDEX IF NOT EXISTS idx_labels_project ON labels(project_id);
CREATE INDEX IF NOT EXISTS idx_datasets_project ON datasets(project_id);
CREATE INDEX IF NOT EXISTS idx_dataset_samples_dataset ON dataset_samples(dataset_id);
`

func Open(path string) (*sql.DB, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := configureSQLite(d); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("sqlite config: %w", err)
	}
	if err := d.Ping(); err != nil {
		_ = d.Close()
		return nil, err
	}
	if _, err := d.Exec(schema); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	return d, nil
}
