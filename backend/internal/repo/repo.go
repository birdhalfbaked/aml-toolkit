package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"com.birdhalfbaked.aml-toolkit/internal/fieldschema"
	"com.birdhalfbaked.aml-toolkit/internal/models"
)

type Repo struct {
	DB *sql.DB
}

func (r *Repo) ListProjects(ctx context.Context) ([]models.Project, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, name, created_at FROM projects ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Project, 0)
	for rows.Next() {
		var p models.Project
		var ts string
		if err := rows.Scan(&p.ID, &p.Name, &ts); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repo) CreateProject(ctx context.Context, name string) (*models.Project, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := r.DB.ExecContext(ctx, `INSERT INTO projects (name, created_at) VALUES (?, ?)`, name, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	t, _ := time.Parse(time.RFC3339Nano, now)
	return &models.Project{ID: id, Name: name, CreatedAt: t}, nil
}

func (r *Repo) GetProject(ctx context.Context, id int64) (*models.Project, error) {
	var p models.Project
	var ts string
	err := r.DB.QueryRowContext(ctx, `SELECT id, name, created_at FROM projects WHERE id = ?`, id).Scan(&p.ID, &p.Name, &ts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
	return &p, nil
}

func (r *Repo) ListCollections(ctx context.Context, projectID int64) ([]models.Collection, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, project_id, name, created_at, field_schema_json FROM collections WHERE project_id = ? ORDER BY id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Collection, 0)
	for rows.Next() {
		var c models.Collection
		var ts string
		var fs sql.NullString
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Name, &ts, &fs); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
		if fs.Valid {
			c.FieldSchemaJSON = fs.String
		} else {
			c.FieldSchemaJSON = fieldschema.DefaultSchemaJSON()
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repo) CreateCollection(ctx context.Context, projectID int64, name string) (*models.Collection, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	fs := fieldschema.DefaultSchemaJSON()
	res, err := r.DB.ExecContext(ctx, `INSERT INTO collections (project_id, name, created_at, field_schema_json) VALUES (?, ?, ?, ?)`, projectID, name, now, fs)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	t, _ := time.Parse(time.RFC3339Nano, now)
	return &models.Collection{ID: id, ProjectID: projectID, Name: name, CreatedAt: t, FieldSchemaJSON: fs}, nil
}

func (r *Repo) GetCollection(ctx context.Context, id int64) (*models.Collection, error) {
	var c models.Collection
	var ts string
	var fs sql.NullString
	err := r.DB.QueryRowContext(ctx, `SELECT id, project_id, name, created_at, field_schema_json FROM collections WHERE id = ?`, id).Scan(&c.ID, &c.ProjectID, &c.Name, &ts, &fs)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
	if fs.Valid {
		c.FieldSchemaJSON = fs.String
	} else {
		c.FieldSchemaJSON = fieldschema.DefaultSchemaJSON()
	}
	return &c, nil
}

func (r *Repo) UpdateCollectionFieldSchema(ctx context.Context, id int64, jsonStr string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE collections SET field_schema_json = ? WHERE id = ?`, jsonStr, id)
	return err
}

func (r *Repo) ListAudioFiles(ctx context.Context, collectionID int64) ([]models.AudioFile, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, collection_id, stored_filename, original_name, format, duration_ms, uploaded_at, COALESCE(field_values_json, '{}') FROM audio_files WHERE collection_id = ? ORDER BY id`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AudioFile, 0)
	for rows.Next() {
		var a models.AudioFile
		var dur sql.NullInt64
		var ts string
		var fv sql.NullString
		if err := rows.Scan(&a.ID, &a.CollectionID, &a.StoredFilename, &a.OriginalName, &a.Format, &dur, &ts, &fv); err != nil {
			return nil, err
		}
		if dur.Valid {
			v := dur.Int64
			a.DurationMs = &v
		}
		a.UploadedAt, _ = time.Parse(time.RFC3339Nano, ts)
		a.FieldValues = parseFieldValuesJSON(fv)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repo) InsertAudioFile(ctx context.Context, collectionID int64, stored, original, format string, durationMs *int64) (*models.AudioFile, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var dur interface{}
	if durationMs != nil {
		dur = *durationMs
	}
	res, err := r.DB.ExecContext(ctx, `INSERT INTO audio_files (collection_id, stored_filename, original_name, format, duration_ms, uploaded_at) VALUES (?, ?, ?, ?, ?, ?)`,
		collectionID, stored, original, format, dur, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	t, _ := time.Parse(time.RFC3339Nano, now)
	return &models.AudioFile{ID: id, CollectionID: collectionID, StoredFilename: stored, OriginalName: original, Format: format, DurationMs: durationMs, UploadedAt: t}, nil
}

func (r *Repo) GetAudioFile(ctx context.Context, id int64) (*models.AudioFile, error) {
	var a models.AudioFile
	var dur sql.NullInt64
	var ts string
	var fv sql.NullString
	err := r.DB.QueryRowContext(ctx, `SELECT id, collection_id, stored_filename, original_name, format, duration_ms, uploaded_at, COALESCE(field_values_json, '{}') FROM audio_files WHERE id = ?`, id).Scan(
		&a.ID, &a.CollectionID, &a.StoredFilename, &a.OriginalName, &a.Format, &dur, &ts, &fv)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if dur.Valid {
		v := dur.Int64
		a.DurationMs = &v
	}
	a.UploadedAt, _ = time.Parse(time.RFC3339Nano, ts)
	a.FieldValues = parseFieldValuesJSON(fv)
	return &a, nil
}

func (r *Repo) UpdateAudioFileFieldValues(ctx context.Context, id int64, jsonStr string) error {
	if strings.TrimSpace(jsonStr) == "" {
		jsonStr = "{}"
	}
	_, err := r.DB.ExecContext(ctx, `UPDATE audio_files SET field_values_json = ? WHERE id = ?`, jsonStr, id)
	return err
}

func (r *Repo) DeleteAudioFile(ctx context.Context, id int64) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM audio_files WHERE id = ?`, id)
	return err
}

func (r *Repo) ListLabels(ctx context.Context, projectID int64) ([]models.Label, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, project_id, name FROM labels WHERE project_id = ? ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Label, 0)
	for rows.Next() {
		var l models.Label
		if err := rows.Scan(&l.ID, &l.ProjectID, &l.Name); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *Repo) UpsertLabel(ctx context.Context, projectID int64, name string) (*models.Label, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("empty label")
	}
	_, err := r.DB.ExecContext(ctx, `INSERT INTO labels (project_id, name) VALUES (?, ?) ON CONFLICT(project_id, name) DO NOTHING`, projectID, name)
	if err != nil {
		return nil, err
	}
	var l models.Label
	err = r.DB.QueryRowContext(ctx, `SELECT id, project_id, name FROM labels WHERE project_id = ? AND name = ?`, projectID, name).Scan(&l.ID, &l.ProjectID, &l.Name)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func parseFieldValuesJSON(ns sql.NullString) map[string]string {
	if !ns.Valid || strings.TrimSpace(ns.String) == "" || ns.String == "{}" {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(ns.String), &m); err != nil || m == nil {
		return map[string]string{}
	}
	return m
}

func (r *Repo) ListSegments(ctx context.Context, audioFileID int64) ([]models.Segment, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT s.id, s.audio_file_id, s.start_ms, s.end_ms, s.label_id, s.transcription, l.name, s.field_values_json
		FROM segments s
		LEFT JOIN labels l ON l.id = s.label_id
		WHERE s.audio_file_id = ?
		ORDER BY s.start_ms`, audioFileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Segment, 0)
	for rows.Next() {
		var s models.Segment
		var lid sql.NullInt64
		var tr sql.NullString
		var ln sql.NullString
		var fv sql.NullString
		if err := rows.Scan(&s.ID, &s.AudioFileID, &s.StartMs, &s.EndMs, &lid, &tr, &ln, &fv); err != nil {
			return nil, err
		}
		if lid.Valid {
			v := lid.Int64
			s.LabelID = &v
		}
		if tr.Valid {
			t := tr.String
			s.Transcription = &t
		}
		if ln.Valid {
			n := ln.String
			s.LabelName = &n
		}
		s.FieldValues = parseFieldValuesJSON(fv)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repo) CreateSegment(ctx context.Context, audioFileID, startMs, endMs int64, labelID *int64, transcription *string, fieldValuesJSON string) (*models.Segment, error) {
	if strings.TrimSpace(fieldValuesJSON) == "" {
		fieldValuesJSON = "{}"
	}
	res, err := r.DB.ExecContext(ctx, `INSERT INTO segments (audio_file_id, start_ms, end_ms, label_id, transcription, field_values_json) VALUES (?, ?, ?, ?, ?, ?)`,
		audioFileID, startMs, endMs, nullInt64(labelID), nullString(transcription), fieldValuesJSON)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	var fv map[string]string
	_ = json.Unmarshal([]byte(fieldValuesJSON), &fv)
	if fv == nil {
		fv = map[string]string{}
	}
	return &models.Segment{ID: id, AudioFileID: audioFileID, StartMs: startMs, EndMs: endMs, LabelID: labelID, Transcription: transcription, FieldValues: fv}, nil
}

func (r *Repo) UpdateSegment(ctx context.Context, id int64, startMs, endMs int64, labelID *int64, transcription *string, fieldValuesJSON *string) error {
	if fieldValuesJSON != nil {
		fv := *fieldValuesJSON
		if strings.TrimSpace(fv) == "" {
			fv = "{}"
		}
		_, err := r.DB.ExecContext(ctx, `UPDATE segments SET start_ms = ?, end_ms = ?, label_id = ?, transcription = ?, field_values_json = ? WHERE id = ?`,
			startMs, endMs, nullInt64(labelID), nullString(transcription), fv, id)
		return err
	}
	_, err := r.DB.ExecContext(ctx, `UPDATE segments SET start_ms = ?, end_ms = ?, label_id = ?, transcription = ? WHERE id = ?`,
		startMs, endMs, nullInt64(labelID), nullString(transcription), id)
	return err
}

func (r *Repo) DeleteSegment(ctx context.Context, id int64) error {
	_, err := r.DB.ExecContext(ctx, `DELETE FROM segments WHERE id = ?`, id)
	return err
}

func (r *Repo) GetSegment(ctx context.Context, id int64) (*models.Segment, error) {
	var s models.Segment
	var lid sql.NullInt64
	var tr sql.NullString
	var fv sql.NullString
	err := r.DB.QueryRowContext(ctx, `SELECT id, audio_file_id, start_ms, end_ms, label_id, transcription, field_values_json FROM segments WHERE id = ?`, id).Scan(
		&s.ID, &s.AudioFileID, &s.StartMs, &s.EndMs, &lid, &tr, &fv)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lid.Valid {
		v := lid.Int64
		s.LabelID = &v
	}
	if tr.Valid {
		t := tr.String
		s.Transcription = &t
	}
	s.FieldValues = parseFieldValuesJSON(fv)
	return &s, nil
}

func nullInt64(p *int64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

func nullString(p *string) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// LabelingQueue returns audio file ids that need work: no segments or any segment missing required fields per collection schema.
func (r *Repo) LabelingQueue(ctx context.Context, collectionID int64) ([]models.LabelingQueueItem, error) {
	col, err := r.GetCollection(ctx, collectionID)
	if err != nil || col == nil {
		return nil, err
	}
	schema, err := fieldschema.Parse(col.FieldSchemaJSON)
	if err != nil {
		schema, _ = fieldschema.Parse(fieldschema.DefaultSchemaJSON())
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT id FROM audio_files WHERE collection_id = ? ORDER BY id`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var fileIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		fileIDs = append(fileIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]models.LabelingQueueItem, 0)
	for _, fid := range fileIDs {
		segs, err := r.ListSegments(ctx, fid)
		if err != nil {
			return nil, err
		}
		if len(segs) == 0 {
			out = append(out, models.LabelingQueueItem{AudioFileID: fid, Reason: "no_segments"})
			continue
		}
		incomplete := false
		for _, seg := range segs {
			if !fieldschema.SegmentCompleteEx(schema, seg.FieldValues, seg.LabelID, seg.LabelName, seg.Transcription) {
				incomplete = true
				break
			}
		}
		if !incomplete {
			af, err := r.GetAudioFile(ctx, fid)
			if err != nil {
				return nil, err
			}
			if af != nil && !fieldschema.FileCompleteEx(schema, af.FieldValues) {
				incomplete = true
			}
		}
		if incomplete {
			out = append(out, models.LabelingQueueItem{AudioFileID: fid, Reason: "incomplete_fields"})
		}
	}
	return out, nil
}

// SegmentsForExport returns segments with collection/project for materialization (filter export-ready in dataset.Build).
type SegmentExportRow struct {
	SegmentID       int64
	AudioFileID     int64
	CollectionID    int64
	ProjectID       int64
	StartMs         int64
	EndMs           int64
	LabelID         *int64
	LabelName       string
	Transcription   *string
	StoredFilename  string
	Format          string
	FieldValuesJSON     string
	FileFieldValuesJSON string
	FieldSchemaJSON     string
}

func (r *Repo) ListSegmentsForExport(ctx context.Context, projectID int64, collectionIDs []int64) ([]SegmentExportRow, error) {
	var q string
	var args []interface{}
	if len(collectionIDs) == 0 {
		q = `
			SELECT s.id, s.audio_file_id, c.id, c.project_id, s.start_ms, s.end_ms, s.label_id, l.name, s.transcription, af.stored_filename, af.format,
				COALESCE(s.field_values_json, '{}'), COALESCE(af.field_values_json, '{}'), COALESCE(c.field_schema_json, '')
			FROM segments s
			JOIN audio_files af ON af.id = s.audio_file_id
			JOIN collections c ON c.id = af.collection_id
			LEFT JOIN labels l ON l.id = s.label_id
			WHERE c.project_id = ?`
		args = append(args, projectID)
	} else {
		placeholders := strings.Repeat("?,", len(collectionIDs))
		placeholders = placeholders[:len(placeholders)-1]
		q = fmt.Sprintf(`
			SELECT s.id, s.audio_file_id, c.id, c.project_id, s.start_ms, s.end_ms, s.label_id, l.name, s.transcription, af.stored_filename, af.format,
				COALESCE(s.field_values_json, '{}'), COALESCE(af.field_values_json, '{}'), COALESCE(c.field_schema_json, '')
			FROM segments s
			JOIN audio_files af ON af.id = s.audio_file_id
			JOIN collections c ON c.id = af.collection_id
			LEFT JOIN labels l ON l.id = s.label_id
			WHERE c.project_id = ? AND c.id IN (%s)`, placeholders)
		args = append(args, projectID)
		for _, id := range collectionIDs {
			args = append(args, id)
		}
	}
	q += ` ORDER BY s.id ASC`
	rows, err := r.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SegmentExportRow, 0)
	for rows.Next() {
		var row SegmentExportRow
		var tr sql.NullString
		var ln sql.NullString
		var lid sql.NullInt64
		if err := rows.Scan(&row.SegmentID, &row.AudioFileID, &row.CollectionID, &row.ProjectID, &row.StartMs, &row.EndMs, &lid, &ln, &tr, &row.StoredFilename, &row.Format, &row.FieldValuesJSON, &row.FileFieldValuesJSON, &row.FieldSchemaJSON); err != nil {
			return nil, err
		}
		if lid.Valid {
			v := lid.Int64
			row.LabelID = &v
		}
		if ln.Valid {
			row.LabelName = ln.String
		}
		if tr.Valid {
			t := tr.String
			row.Transcription = &t
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repo) GetLabelByID(ctx context.Context, id int64) (*models.Label, error) {
	var l models.Label
	err := r.DB.QueryRowContext(ctx, `SELECT id, project_id, name FROM labels WHERE id = ?`, id).Scan(&l.ID, &l.ProjectID, &l.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *Repo) InsertDataset(ctx context.Context, projectID int64, name, optionsJSON, storageRoot string) (*models.Dataset, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := r.DB.ExecContext(ctx, `INSERT INTO datasets (project_id, name, created_at, options_json, storage_root) VALUES (?, ?, ?, ?, ?)`,
		projectID, name, now, optionsJSON, storageRoot)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	t, _ := time.Parse(time.RFC3339Nano, now)
	return &models.Dataset{ID: id, ProjectID: projectID, Name: name, CreatedAt: t, OptionsJSON: optionsJSON, StorageRoot: storageRoot}, nil
}

func (r *Repo) UpdateDatasetStorageRoot(ctx context.Context, id int64, root string) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE datasets SET storage_root = ? WHERE id = ?`, root, id)
	return err
}

func (r *Repo) InsertDatasetSample(ctx context.Context, datasetID int64, split, filename, relPath, label string, transcription *string, sourceSegmentID int64, augJSON *string) (int64, error) {
	res, err := r.DB.ExecContext(ctx, `INSERT INTO dataset_samples (dataset_id, split, filename, rel_path, label, transcription, source_segment_id, augmentation_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		datasetID, split, filename, relPath, label, nullString(transcription), sourceSegmentID, nullString(augJSON))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func (r *Repo) ListDatasets(ctx context.Context, projectID int64) ([]models.Dataset, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id, project_id, name, created_at, options_json, storage_root FROM datasets WHERE project_id = ? ORDER BY id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.Dataset, 0)
	for rows.Next() {
		var d models.Dataset
		var ts string
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Name, &ts, &d.OptionsJSON, &d.StorageRoot); err != nil {
			return nil, err
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repo) GetDataset(ctx context.Context, id int64) (*models.Dataset, error) {
	var d models.Dataset
	var ts string
	err := r.DB.QueryRowContext(ctx, `SELECT id, project_id, name, created_at, options_json, storage_root FROM datasets WHERE id = ?`, id).Scan(
		&d.ID, &d.ProjectID, &d.Name, &ts, &d.OptionsJSON, &d.StorageRoot)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339Nano, ts)
	return &d, nil
}

func (r *Repo) ListDatasetSamples(ctx context.Context, datasetID int64, split *string, limit, offset int) ([]models.DatasetSample, error) {
	q := `SELECT id, dataset_id, split, filename, rel_path, label, transcription, source_segment_id, augmentation_json FROM dataset_samples WHERE dataset_id = ?`
	args := []interface{}{datasetID}
	if split != nil && *split != "" {
		q += ` AND split = ?`
		args = append(args, *split)
	}
	q += ` ORDER BY id LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := r.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.DatasetSample, 0)
	for rows.Next() {
		var s models.DatasetSample
		var tr, aug sql.NullString
		if err := rows.Scan(&s.ID, &s.DatasetID, &s.Split, &s.Filename, &s.RelPath, &s.Label, &tr, &s.SourceSegmentID, &aug); err != nil {
			return nil, err
		}
		if tr.Valid {
			t := tr.String
			s.Transcription = &t
		}
		if aug.Valid {
			a := aug.String
			s.AugmentationJSON = &a
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repo) GetDatasetSample(ctx context.Context, datasetID, sampleID int64) (*models.DatasetSample, error) {
	var s models.DatasetSample
	var tr, aug sql.NullString
	err := r.DB.QueryRowContext(ctx, `SELECT id, dataset_id, split, filename, rel_path, label, transcription, source_segment_id, augmentation_json FROM dataset_samples WHERE dataset_id = ? AND id = ?`,
		datasetID, sampleID).Scan(&s.ID, &s.DatasetID, &s.Split, &s.Filename, &s.RelPath, &s.Label, &tr, &s.SourceSegmentID, &aug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if tr.Valid {
		t := tr.String
		s.Transcription = &t
	}
	if aug.Valid {
		a := aug.String
		s.AugmentationJSON = &a
	}
	return &s, nil
}
