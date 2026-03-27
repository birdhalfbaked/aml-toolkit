// Export permutation tests: synthetic repo.SegmentExportRow + WAV on disk exercise dataset.Build
// without requiring segments in SQLite. Covers:
//   - invalid split ratios
//   - ExportReady filtering (requireTranscription, incomplete multi-taxonomy)
//   - train/validation/evaluation counts (deterministic seed)
//   - manifest label, transcription, merged file+segment fields
//   - augmentation (noise, time-shift) and silence-trim (collapse vs non-silent success)
package dataset

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"com.birdhalfbaked.aml-toolkit/internal/db"
	"com.birdhalfbaked.aml-toolkit/internal/models"
	"com.birdhalfbaked.aml-toolkit/internal/repo"
	"com.birdhalfbaked.aml-toolkit/internal/store"
)

// Synthetic SegmentExportRow + on-disk WAV exercises dataset.Build without a full segment graph in SQLite.

func newExportFixture(t *testing.T) (ctx context.Context, rp *repo.Repo, ly *store.Layout, projectID, collectionID int64, stored string) {
	t.Helper()
	ctx = context.Background()
	root := t.TempDir()
	sqldb, err := db.Open(filepath.Join(root, "app.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })
	ly, err = store.NewLayout(filepath.Join(root, "data"))
	if err != nil {
		t.Fatalf("store.NewLayout: %v", err)
	}
	rp = &repo.Repo{DB: sqldb}
	p, err := rp.CreateProject(ctx, "perm-proj")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	projectID = p.ID
	collectionID = 1
	if err := ly.EnsureCollectionDir(projectID, collectionID); err != nil {
		t.Fatalf("EnsureCollectionDir: %v", err)
	}
	stored = "clip.wav"
	writeTestWav(t, filepath.Join(ly.CollectionDir(projectID, collectionID), stored), 8000, 2000)
	return ctx, rp, ly, projectID, collectionID, stored
}

func exportRow(
	projectID, collectionID int64,
	segmentID, audioFileID int64,
	stored, schema, fvJSON, fileFVJSON string,
	labelID *int64,
	labelName string,
	transcription *string,
) repo.SegmentExportRow {
	return repo.SegmentExportRow{
		SegmentID:           segmentID,
		AudioFileID:         audioFileID,
		CollectionID:        collectionID,
		ProjectID:           projectID,
		StartMs:             0,
		EndMs:               400,
		LabelID:             labelID,
		LabelName:           labelName,
		Transcription:       transcription,
		StoredFilename:      stored,
		Format:              "wav",
		FieldValuesJSON:     fvJSON,
		FileFieldValuesJSON: fileFVJSON,
		FieldSchemaJSON:     schema,
	}
}

const schemaSpeakerTextarea = `{"version":1,"fields":[` +
	`{"id":"speaker","type":"taxonomy","title":"Speaker","required":true,"scope":"segment"},` +
	`{"id":"transcription","type":"textarea","title":"T","required":false,"scope":"segment"}` +
	`]}`

const schemaMultiTaxFile = `{"version":1,"fields":[` +
	`{"id":"speaker","type":"taxonomy","title":"Speaker","required":true,"scope":"segment"},` +
	`{"id":"phrase_type","type":"taxonomy","title":"Phrase","required":true,"scope":"segment"},` +
	`{"id":"transcription","type":"textarea","title":"T","required":false,"scope":"segment"},` +
	`{"id":"source","type":"text","title":"Source","required":false,"scope":"file"}` +
	`]}`

func TestBuild_InvalidRatios_ReturnsError(t *testing.T) {
	ctx, rp, ly, pid, _, _ := newExportFixture(t)
	req := models.CreateDatasetRequest{
		Name:              "bad",
		TrainRatio:        0.4,
		ValidationRatio:   0.4,
		EvaluationRatio:   0.1,
		RequireTranscription: false,
	}
	_, err := Build(ctx, rp, ly, pid, req, nil)
	if err == nil || !strings.Contains(err.Error(), "split ratios") {
		t.Fatalf("Build() error = %v; want split ratios error", err)
	}
}

func TestBuild_AllRowsFiltered_EmptySamples(t *testing.T) {
	ctx, rp, ly, pid, cid, stored := newExportFixture(t)
	lid := int64(42)
	rows := []repo.SegmentExportRow{
		exportRow(pid, cid, 1, 101, stored, schemaSpeakerTextarea,
			`{"speaker":"pilot","transcription":""}`,
			`{}`, &lid, "pilot", nil),
	}
	req := models.CreateDatasetRequest{
		Name:              "empty",
		TrainRatio:        1,
		ValidationRatio:   0,
		EvaluationRatio:   0,
		RequireTranscription: true,
	}
	ds, err := Build(ctx, rp, ly, pid, req, rows)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(ds.StorageRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man Manifest
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	if len(man.Samples) != 0 {
		t.Fatalf("samples = %d; want 0 filtered by requireTranscription", len(man.Samples))
	}
}

func TestBuild_SplitAssignment_ThreeSegments_Deterministic(t *testing.T) {
	ctx, rp, ly, pid, cid, stored := newExportFixture(t)
	lid := int64(7)
	rows := []repo.SegmentExportRow{
		exportRow(pid, cid, 1, 101, stored, schemaSpeakerTextarea,
			`{"speaker":"a","transcription":"t1"}`, `{}`, &lid, "a", nil),
		exportRow(pid, cid, 2, 102, stored, schemaSpeakerTextarea,
			`{"speaker":"b","transcription":"t2"}`, `{}`, &lid, "b", nil),
		exportRow(pid, cid, 3, 103, stored, schemaSpeakerTextarea,
			`{"speaker":"c","transcription":"t3"}`, `{}`, &lid, "c", nil),
	}
	req := models.CreateDatasetRequest{
		Name:              "splits",
		TrainRatio:        1.0 / 3.0,
		ValidationRatio:   1.0 / 3.0,
		EvaluationRatio:   1.0 / 3.0,
		RequireTranscription: false,
		Seed:              int64Ptr(42),
	}
	ds, err := Build(ctx, rp, ly, pid, req, rows)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(ds.StorageRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man Manifest
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	if len(man.Samples) != 3 {
		t.Fatalf("len = %d; want 3", len(man.Samples))
	}
	counts := map[string]int{}
	for _, s := range man.Samples {
		counts[s.Split]++
	}
	if counts["train"] != 1 || counts["validation"] != 1 || counts["evaluation"] != 1 {
		t.Fatalf("split counts = %#v; want 1 each (seed 42)", counts)
	}
}

func TestBuild_Augmentation_Noise_AddsVariantSamples(t *testing.T) {
	ctx, rp, ly, pid, cid, stored := newExportFixture(t)
	lid := int64(7)
	rows := []repo.SegmentExportRow{
		exportRow(pid, cid, 1, 101, stored, schemaSpeakerTextarea,
			`{"speaker":"pilot","transcription":"x"}`, `{}`, &lid, "pilot", nil),
	}
	n := 50.0
	req := models.CreateDatasetRequest{
		Name:              "aug",
		TrainRatio:        1,
		ValidationRatio:   0,
		EvaluationRatio:   0,
		RequireTranscription: false,
		AugmentVariants:   2,
		AugmentNoiseDB:    &n,
	}
	ds, err := Build(ctx, rp, ly, pid, req, rows)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(ds.StorageRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man Manifest
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	if len(man.Samples) != 3 {
		t.Fatalf("samples = %d; want 1 base + 2 aug", len(man.Samples))
	}
	var aug int
	for _, s := range man.Samples {
		if s.Augmentation != nil {
			aug++
		}
	}
	if aug != 2 {
		t.Fatalf("augmented samples = %d; want 2", aug)
	}
	_, err = rp.ListDatasetSamples(ctx, ds.ID, nil, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestBuild_Augmentation_TimeShiftOnly_AddsVariantSamples(t *testing.T) {
	ctx, rp, ly, pid, cid, stored := newExportFixture(t)
	lid := int64(7)
	rows := []repo.SegmentExportRow{
		exportRow(pid, cid, 1, 101, stored, schemaSpeakerTextarea,
			`{"speaker":"pilot","transcription":"x"}`, `{}`, &lid, "pilot", nil),
	}
	ms := int64(80)
	req := models.CreateDatasetRequest{
		Name:                 "aug-shift",
		TrainRatio:           1,
		ValidationRatio:      0,
		EvaluationRatio:      0,
		RequireTranscription: false,
		AugmentVariants:      1,
		AugmentMaxShiftMs:    &ms,
	}
	ds, err := Build(ctx, rp, ly, pid, req, rows)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(ds.StorageRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man Manifest
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	if len(man.Samples) != 2 {
		t.Fatalf("samples = %d; want 1 base + 1 shift aug", len(man.Samples))
	}
	var aug *string
	for i := range man.Samples {
		if man.Samples[i].Augmentation != nil {
			aug = man.Samples[i].Augmentation
			break
		}
	}
	if aug == nil || !strings.Contains(*aug, "shift") {
		t.Fatalf("want shift augmentation metadata, got %#v", man.Samples)
	}
	if ds.StorageRoot == "" {
		t.Fatal("empty storage root")
	}
}

func TestBuild_MultiTaxonomyAndFileFields_ManifestFields(t *testing.T) {
	ctx, rp, ly, pid, cid, stored := newExportFixture(t)
	lid := int64(99)
	rows := []repo.SegmentExportRow{
		exportRow(pid, cid, 1, 101, stored, schemaMultiTaxFile,
			`{"speaker":"pilot","phrase_type":"command","transcription":"hi"}`,
			`{"source":"tower"}`, &lid, "pilot", nil),
	}
	req := models.CreateDatasetRequest{
		Name:              "mt",
		TrainRatio:        1,
		ValidationRatio:   0,
		EvaluationRatio:   0,
		RequireTranscription: false,
	}
	ds, err := Build(ctx, rp, ly, pid, req, rows)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(ds.StorageRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man Manifest
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	if len(man.Samples) != 1 {
		t.Fatalf("samples = %d", len(man.Samples))
	}
	f := man.Samples[0].Fields
	if f["speaker"] != "pilot" || f["phrase_type"] != "command" || f["source"] != "tower" {
		t.Fatalf("fields = %#v", f)
	}
	if man.Samples[0].Label != "pilot" {
		t.Fatalf("label = %q", man.Samples[0].Label)
	}
	if man.Samples[0].Transcription == nil || *man.Samples[0].Transcription != "hi" {
		t.Fatalf("transcription = %v", man.Samples[0].Transcription)
	}
}

func TestBuild_IncompleteSegment_FilteredOut(t *testing.T) {
	ctx, rp, ly, pid, cid, stored := newExportFixture(t)
	lid := int64(1)
	rows := []repo.SegmentExportRow{
		exportRow(pid, cid, 1, 101, stored, schemaMultiTaxFile,
			`{"speaker":"pilot","phrase_type":"command"}`,
			`{}`, &lid, "pilot", nil),
		exportRow(pid, cid, 2, 102, stored, schemaMultiTaxFile,
			`{"speaker":"pilot"}`,
			`{}`, &lid, "pilot", nil),
	}
	req := models.CreateDatasetRequest{
		Name:              "mix",
		TrainRatio:        1,
		ValidationRatio:   0,
		EvaluationRatio:   0,
		RequireTranscription: false,
	}
	ds, err := Build(ctx, rp, ly, pid, req, rows)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(ds.StorageRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man Manifest
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	if len(man.Samples) != 1 {
		t.Fatalf("samples = %d; want 1 (incomplete row excluded)", len(man.Samples))
	}
}

// writeTestWavNonSilent writes mono 16-bit PCM with non-zero samples so silence-trim does not collapse the whole segment.
func writeTestWavNonSilent(t *testing.T, path string, sampleRate int, durationMs int64) {
	t.Helper()
	frames := int(int64(sampleRate) * durationMs / 1000)
	if frames < 1 {
		frames = 1
	}
	pcm := make([]byte, frames*2)
	for i := range pcm {
		pcm[i] = byte((i*17 + 33) % 255)
		if pcm[i] == 0 {
			pcm[i] = 1
		}
	}
	if err := writePCMToWavFile(path, sampleRate, pcm); err != nil {
		t.Fatal(err)
	}
}

func writePCMToWavFile(path string, sampleRate int, pcm []byte) error {
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
	return os.WriteFile(path, b.Bytes(), 0o644)
}

func TestBuild_SilenceTrimRMS_AllSilenceClip_ReturnsCollapseError(t *testing.T) {
	ctx, rp, ly, pid, cid, stored := newExportFixture(t)
	lid := int64(3)
	rows := []repo.SegmentExportRow{
		exportRow(pid, cid, 1, 101, stored, schemaSpeakerTextarea,
			`{"speaker":"p","transcription":"t"}`, `{}`, &lid, "p", nil),
	}
	rms := 0.02
	req := models.CreateDatasetRequest{
		Name:              "trim-fail",
		TrainRatio:        1,
		ValidationRatio:   0,
		EvaluationRatio:   0,
		RequireTranscription: false,
		SilenceTrimRMS:    &rms,
	}
	_, err := Build(ctx, rp, ly, pid, req, rows)
	if err == nil || !strings.Contains(err.Error(), "collapsed") {
		t.Fatalf("Build() error = %v; want collapse after trim on silence-only source", err)
	}
}

func TestBuild_SilenceTrimRMS_NonSilentSource_Succeeds(t *testing.T) {
	ctx, rp, ly, pid, cid, stored := newExportFixture(t)
	writeTestWavNonSilent(t, filepath.Join(ly.CollectionDir(pid, cid), stored), 8000, 2000)
	lid := int64(3)
	rows := []repo.SegmentExportRow{
		exportRow(pid, cid, 1, 101, stored, schemaSpeakerTextarea,
			`{"speaker":"p","transcription":"t"}`, `{}`, &lid, "p", nil),
	}
	rms := 0.02
	req := models.CreateDatasetRequest{
		Name:              "trim-ok",
		TrainRatio:        1,
		ValidationRatio:   0,
		EvaluationRatio:   0,
		RequireTranscription: false,
		SilenceTrimRMS:    &rms,
	}
	ds, err := Build(ctx, rp, ly, pid, req, rows)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(ds.StorageRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man Manifest
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatal(err)
	}
	if len(man.Samples) != 1 {
		t.Fatalf("samples = %d; want 1", len(man.Samples))
	}
}
