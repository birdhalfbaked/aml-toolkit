package handlers

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"com.birdhalfbaked.aml-toolkit/internal/db"
	"com.birdhalfbaked.aml-toolkit/internal/models"
	"com.birdhalfbaked.aml-toolkit/internal/repo"
)

func TestMergeSegmentPayload_MultiTaxonomy_UpsertsAllAndSetsPrimaryLabelID(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqldb, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })

	rp := &repo.Repo{DB: sqldb}

	pr, err := rp.CreateProject(ctx, "p1")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// Two taxonomy fields: speaker (primary) + phrase type (secondary).
	col := &models.Collection{
		ID:            1,
		ProjectID:     pr.ID,
		Name:          "c1",
		FieldSchemaJSON: `{"version":1,"fields":[` +
			`{"id":"speaker","type":"taxonomy","title":"Speaker","required":true,"scope":"segment"},` +
			`{"id":"phrase_type","type":"taxonomy","title":"Phrase type","required":true,"scope":"segment"}` +
			`]}`,
	}

	body := &segmentBody{
		StartMs: 0,
		EndMs:   1000,
		FieldValues: map[string]string{
			"speaker":     "pilot",
			"phrase_type": "command",
		},
	}

	labelID, tr, fvJSON, err := mergeSegmentPayload(ctx, rp, pr.ID, col, body, nil)
	if err != nil {
		t.Fatalf("mergeSegmentPayload() error = %v", err)
	}
	if tr != nil {
		t.Fatalf("transcription = %v; want nil", *tr)
	}

	// Field values should include both taxonomy values.
	var fv map[string]string
	if err := json.Unmarshal([]byte(fvJSON), &fv); err != nil {
		t.Fatalf("unmarshal fieldValuesJSON error = %v", err)
	}
	if fv["speaker"] != "pilot" || fv["phrase_type"] != "command" {
		t.Fatalf("field values = %#v; want speaker=pilot and phrase_type=command", fv)
	}

	// Both values should be upserted as labels.
	labels, err := rp.ListLabels(ctx, pr.ID)
	if err != nil {
		t.Fatalf("ListLabels() error = %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("labels len = %d; want 2 (pilot, command)", len(labels))
	}
	var pilotID, commandID int64
	for _, l := range labels {
		switch l.Name {
		case "pilot":
			pilotID = l.ID
		case "command":
			commandID = l.ID
		}
	}
	if pilotID == 0 || commandID == 0 {
		t.Fatalf("label IDs: pilot=%d command=%d; want both non-zero", pilotID, commandID)
	}

	// Primary taxonomy should map to returned labelID.
	if labelID == nil || *labelID != pilotID {
		got := int64(0)
		if labelID != nil {
			got = *labelID
		}
		t.Fatalf("labelID = %d; want %d (pilot)", got, pilotID)
	}
}

