package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"com.birdhalfbaked.aml-toolkit/internal/fieldschema"
)

func columnExists(db *sql.DB, table, col string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}

// RunMigrations adds field_schema_json / field_values_json and backfills legacy rows.
// Run explicitly via cmd/migrate (or call after Open); the HTTP server does not run this on startup.
func RunMigrations(db *sql.DB) error {
	hasCol, err := columnExists(db, "collections", "field_schema_json")
	if err != nil {
		return err
	}
	if !hasCol {
		if _, err := db.Exec(`ALTER TABLE collections ADD COLUMN field_schema_json TEXT`); err != nil {
			return fmt.Errorf("alter collections: %w", err)
		}
	}
	hasSeg, err := columnExists(db, "segments", "field_values_json")
	if err != nil {
		return err
	}
	if !hasSeg {
		if _, err := db.Exec(`ALTER TABLE segments ADD COLUMN field_values_json TEXT`); err != nil {
			return fmt.Errorf("alter segments: %w", err)
		}
	}
	hasAF, err := columnExists(db, "audio_files", "field_values_json")
	if err != nil {
		return err
	}
	if !hasAF {
		if _, err := db.Exec(`ALTER TABLE audio_files ADD COLUMN field_values_json TEXT DEFAULT '{}'`); err != nil {
			return fmt.Errorf("alter audio_files: %w", err)
		}
	}
	if _, err := db.Exec(`UPDATE audio_files SET field_values_json = '{}' WHERE field_values_json IS NULL OR TRIM(field_values_json) = ''`); err != nil {
		return err
	}

	def := fieldschema.DefaultSchemaJSON()
	if _, err := db.Exec(`UPDATE collections SET field_schema_json = ? WHERE field_schema_json IS NULL OR TRIM(field_schema_json) = ''`, def); err != nil {
		return err
	}

	rows, err := db.Query(`
		SELECT s.id, s.label_id, s.transcription, l.name, s.field_values_json
		FROM segments s
		LEFT JOIN labels l ON l.id = s.label_id
		WHERE s.field_values_json IS NULL OR TRIM(s.field_values_json) = '' OR s.field_values_json = '{}'`)
	if err != nil {
		return err
	}
	type segBackfill struct {
		id   int64
		json string
	}
	var pending []segBackfill
	for rows.Next() {
		var id int64
		var lid sql.NullInt64
		var tr sql.NullString
		var lname sql.NullString
		var fv sql.NullString
		if err := rows.Scan(&id, &lid, &tr, &lname, &fv); err != nil {
			_ = rows.Close()
			return err
		}
		m := map[string]string{}
		if fv.Valid && strings.TrimSpace(fv.String) != "" && fv.String != "{}" {
			_ = json.Unmarshal([]byte(fv.String), &m)
		}
		if lname.Valid && strings.TrimSpace(lname.String) != "" {
			m["label"] = strings.TrimSpace(lname.String)
		}
		if tr.Valid && strings.TrimSpace(tr.String) != "" {
			m["transcription"] = strings.TrimSpace(tr.String)
		}
		b, err := json.Marshal(m)
		if err != nil {
			continue
		}
		pending = append(pending, segBackfill{id: id, json: string(b)})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	// Cannot run db.Exec while rows are open: with SetMaxOpenConns(1) that deadlocks.
	for _, p := range pending {
		if _, err := db.Exec(`UPDATE segments SET field_values_json = ? WHERE id = ?`, p.json, p.id); err != nil {
			return err
		}
	}
	return nil
}
