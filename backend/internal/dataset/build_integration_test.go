package dataset

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"com.birdhalfbaked.aml-toolkit/internal/db"
	"com.birdhalfbaked.aml-toolkit/internal/models"
	"com.birdhalfbaked.aml-toolkit/internal/repo"
	"com.birdhalfbaked.aml-toolkit/internal/store"
)

// writeTestWav writes a minimal mono 16-bit PCM WAV long enough for segment extraction tests.
func writeTestWav(t *testing.T, path string, sampleRate int, durationMs int64) {
	t.Helper()
	frames := int(int64(sampleRate) * durationMs / 1000)
	if frames < 1 {
		frames = 1
	}
	pcm := make([]byte, frames*2)
	var b bytes.Buffer
	dataSize := len(pcm)
	chunkSize := 36 + dataSize
	_, _ = b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(chunkSize))
	_, _ = b.WriteString("WAVE")
	_, _ = b.WriteString("fmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint32(sampleRate))
	byteRate := uint32(sampleRate * 2)
	_ = binary.Write(&b, binary.LittleEndian, byteRate)
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	_, _ = b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(dataSize))
	b.Write(pcm)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// TestBuild_MultiTaxonomyAndFileFields_StorageAndManifest verifies:
// - segment field_values_json and label_id align with multi-taxonomy rules
// - file-level field_values_json merges into dataset manifest fields
// - export primary label is the first taxonomy value; WAV + manifest land on disk
func TestBuild_MultiTaxonomyAndFileFields_StorageAndManifest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(root, "app.db")
	sqldb, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })

	layout, err := store.NewLayout(filepath.Join(root, "data"))
	if err != nil {
		t.Fatalf("store.NewLayout: %v", err)
	}
	rp := &repo.Repo{DB: sqldb}

	p, err := rp.CreateProject(ctx, "export-proj")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	c, err := rp.CreateCollection(ctx, p.ID, "coll1")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	schema := `{"version":1,"fields":[` +
		`{"id":"speaker","type":"taxonomy","title":"Speaker","required":true,"scope":"segment"},` +
		`{"id":"phrase_type","type":"taxonomy","title":"Phrase","required":true,"scope":"segment"},` +
		`{"id":"transcription","type":"textarea","title":"Transcript","required":false,"scope":"segment"},` +
		`{"id":"source","type":"text","title":"Source","required":false,"scope":"file"}` +
		`]}`
	if err := rp.UpdateCollectionFieldSchema(ctx, c.ID, schema); err != nil {
		t.Fatalf("UpdateCollectionFieldSchema: %v", err)
	}
	if err := layout.EnsureCollectionDir(p.ID, c.ID); err != nil {
		t.Fatalf("EnsureCollectionDir: %v", err)
	}

	storedName := "clip.wav"
	srcWav := filepath.Join(layout.CollectionDir(p.ID, c.ID), storedName)
	writeTestWav(t, srcWav, 8000, 2000)

	af, err := rp.InsertAudioFile(ctx, c.ID, storedName, "clip.wav", "wav", int64Ptr(2000))
	if err != nil {
		t.Fatalf("InsertAudioFile: %v", err)
	}
	fileFV := map[string]string{"source": "tower"}
	b, _ := json.Marshal(fileFV)
	if err := rp.UpdateAudioFileFieldValues(ctx, af.ID, string(b)); err != nil {
		t.Fatalf("UpdateAudioFileFieldValues: %v", err)
	}

	pilot, err := rp.UpsertLabel(ctx, p.ID, "pilot")
	if err != nil {
		t.Fatalf("UpsertLabel pilot: %v", err)
	}
	_, err = rp.UpsertLabel(ctx, p.ID, "command")
	if err != nil {
		t.Fatalf("UpsertLabel command: %v", err)
	}

	segFV := map[string]string{
		"speaker":       "pilot",
		"phrase_type":   "command",
		"transcription": "cleared",
	}
	fvJSON, err := json.Marshal(segFV)
	if err != nil {
		t.Fatal(err)
	}
	lid := pilot.ID
	seg, err := rp.CreateSegment(ctx, af.ID, 0, 500, &lid, nil, string(fvJSON))
	if err != nil {
		t.Fatalf("CreateSegment: %v", err)
	}
	if seg.LabelID == nil || *seg.LabelID != pilot.ID {
		t.Fatalf("segment labelId = %v; want pilot id %d", seg.LabelID, pilot.ID)
	}
	if seg.FieldValues["speaker"] != "pilot" || seg.FieldValues["phrase_type"] != "command" {
		t.Fatalf("segment fieldValues = %#v", seg.FieldValues)
	}

	got, err := rp.GetSegment(ctx, seg.ID)
	if err != nil || got == nil {
		t.Fatalf("GetSegment: %v", err)
	}
	if got.FieldValues["phrase_type"] != "command" {
		t.Fatalf("stored field_values_json missing phrase_type: %#v", got.FieldValues)
	}

	rows, err := rp.ListSegmentsForExport(ctx, p.ID, []int64{c.ID})
	if err != nil {
		t.Fatalf("ListSegmentsForExport: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("export rows len = %d; want 1", len(rows))
	}
	if rows[0].LabelName != "pilot" {
		t.Fatalf("export row LabelName = %q; want pilot", rows[0].LabelName)
	}

	req := models.CreateDatasetRequest{
		Name:              "ds1",
		TrainRatio:        1,
		ValidationRatio:   0,
		EvaluationRatio:   0,
		RequireTranscription: false,
	}
	ds, err := Build(ctx, rp, layout, p.ID, req, rows)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if ds.StorageRoot == "" {
		t.Fatal("dataset StorageRoot empty")
	}

	manifestPath := filepath.Join(ds.StorageRoot, "manifest.json")
	mf, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var man Manifest
	if err := json.Unmarshal(mf, &man); err != nil {
		t.Fatalf("manifest json: %v", err)
	}
	if len(man.Samples) != 1 {
		t.Fatalf("manifest samples = %d; want 1", len(man.Samples))
	}
	s0 := man.Samples[0]
	if s0.Label != "pilot" {
		t.Fatalf("manifest Label = %q; want primary taxonomy pilot", s0.Label)
	}
	if s0.Transcription == nil || *s0.Transcription != "cleared" {
		t.Fatalf("manifest Transcription = %v; want cleared", s0.Transcription)
	}
	if s0.Fields == nil {
		t.Fatal("manifest Fields nil")
	}
	if s0.Fields["speaker"] != "pilot" || s0.Fields["phrase_type"] != "command" {
		t.Fatalf("manifest Fields taxonomies = %#v", s0.Fields)
	}
	if s0.Fields["source"] != "tower" {
		t.Fatalf("manifest Fields[source] = %q; want merged file field tower", s0.Fields["source"])
	}

	outWav := filepath.Join(ds.StorageRoot, s0.Filename)
	if st, err := os.Stat(outWav); err != nil || st.Size() == 0 {
		t.Fatalf("exported wav missing or empty: %v size=%v", err, st)
	}

	samples, err := rp.ListDatasetSamples(ctx, ds.ID, nil, 10, 0)
	if err != nil {
		t.Fatalf("ListDatasetSamples: %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("db samples len = %d; want 1", len(samples))
	}
	if samples[0].Label != "pilot" {
		t.Fatalf("dataset sample label = %q; want pilot", samples[0].Label)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
