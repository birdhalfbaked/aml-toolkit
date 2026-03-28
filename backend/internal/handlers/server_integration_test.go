package handlers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"com.birdhalfbaked.aml-toolkit/internal/db"
	"com.birdhalfbaked.aml-toolkit/internal/models"
	"com.birdhalfbaked.aml-toolkit/internal/repo"
	"com.birdhalfbaked.aml-toolkit/internal/store"

	"github.com/julienschmidt/httprouter"
)

type testEnv struct {
	repo   *repo.Repo
	router *httprouter.Router
	layout *store.Layout
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	root := t.TempDir()
	sqldb, err := db.Open(filepath.Join(root, "app.db"))
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })

	layout, err := store.NewLayout(filepath.Join(root, "data"))
	if err != nil {
		t.Fatalf("store.NewLayout() error = %v", err)
	}

	rp := &repo.Repo{DB: sqldb}
	srv := NewServer(rp, layout)
	r := httprouter.New()
	srv.Register(r)

	return &testEnv{repo: rp, router: r, layout: layout}
}

func requestJSON(t *testing.T, r http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		reqBody = b
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func decodeJSON[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; body=%s", err, rr.Body.String())
	}
	return out
}

func TestHTTP_MultiTaxonomyFlow_CreatesLabelsAndQueueIsComplete(t *testing.T) {
	env := newTestEnv(t)

	rr := requestJSON(t, env.router, http.MethodPost, "/api/projects", map[string]string{"name": "ATC"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create project status = %d; want 201; body=%s", rr.Code, rr.Body.String())
	}
	project := decodeJSON[models.Project](t, rr)

	rr = requestJSON(t, env.router, http.MethodPost, fmt.Sprintf("/api/projects/%d/collections", project.ID), map[string]string{"name": "radio"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create collection status = %d; want 201; body=%s", rr.Code, rr.Body.String())
	}
	collection := decodeJSON[models.Collection](t, rr)

	schema := `{"version":1,"fields":[` +
		`{"id":"speaker","type":"taxonomy","title":"Speaker","required":true,"scope":"segment"},` +
		`{"id":"phrase_type","type":"taxonomy","title":"Phrase Type","required":true,"scope":"segment"},` +
		`{"id":"transcription","type":"textarea","title":"Transcript","required":false,"scope":"segment"}` +
		`]}`
	rr = requestJSON(t, env.router, http.MethodPatch, fmt.Sprintf("/api/collections/%d", collection.ID), map[string]string{
		"fieldSchemaJson": schema,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("patch schema status = %d; want 200; body=%s", rr.Code, rr.Body.String())
	}

	f, err := env.repo.InsertAudioFile(context.Background(), collection.ID, "f1.wav", "f1.wav", "wav", nil)
	if err != nil {
		t.Fatalf("InsertAudioFile() error = %v", err)
	}

	rr = requestJSON(t, env.router, http.MethodPost, fmt.Sprintf("/api/files/%d/segments", f.ID), map[string]interface{}{
		"startMs": 0,
		"endMs":   1200,
		"fieldValues": map[string]string{
			"speaker":     "pilot",
			"phrase_type": "command",
		},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create segment status = %d; want 201; body=%s", rr.Code, rr.Body.String())
	}

	rr = requestJSON(t, env.router, http.MethodGet, fmt.Sprintf("/api/projects/%d/labels", project.ID), nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list labels status = %d; want 200; body=%s", rr.Code, rr.Body.String())
	}
	labels := decodeJSON[[]models.Label](t, rr)
	if len(labels) != 2 {
		t.Fatalf("labels len = %d; want 2; body=%s", len(labels), rr.Body.String())
	}
	var hasPilot, hasCommand bool
	for _, l := range labels {
		if l.Name == "pilot" {
			hasPilot = true
		}
		if l.Name == "command" {
			hasCommand = true
		}
	}
	if !hasPilot || !hasCommand {
		t.Fatalf("labels = %#v; want names pilot + command", labels)
	}

	rr = requestJSON(t, env.router, http.MethodGet, fmt.Sprintf("/api/collections/%d/labeling-queue", collection.ID), nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("labeling queue status = %d; want 200; body=%s", rr.Code, rr.Body.String())
	}
	queue := decodeJSON[[]models.LabelingQueueItem](t, rr)
	if len(queue) != 0 {
		t.Fatalf("queue = %#v; want empty queue", queue)
	}
}

func TestHTTP_MultiTaxonomyFlow_MissingSecondaryTaxonomyIsIncomplete(t *testing.T) {
	env := newTestEnv(t)

	rr := requestJSON(t, env.router, http.MethodPost, "/api/projects", map[string]string{"name": "ATC"})
	project := decodeJSON[models.Project](t, rr)

	rr = requestJSON(t, env.router, http.MethodPost, fmt.Sprintf("/api/projects/%d/collections", project.ID), map[string]string{"name": "radio"})
	collection := decodeJSON[models.Collection](t, rr)

	schema := `{"version":1,"fields":[` +
		`{"id":"speaker","type":"taxonomy","title":"Speaker","required":true,"scope":"segment"},` +
		`{"id":"phrase_type","type":"taxonomy","title":"Phrase Type","required":true,"scope":"segment"}` +
		`]}`
	rr = requestJSON(t, env.router, http.MethodPatch, fmt.Sprintf("/api/collections/%d", collection.ID), map[string]string{
		"fieldSchemaJson": schema,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("patch schema status = %d; want 200; body=%s", rr.Code, rr.Body.String())
	}

	f, err := env.repo.InsertAudioFile(context.Background(), collection.ID, "f2.wav", "f2.wav", "wav", nil)
	if err != nil {
		t.Fatalf("InsertAudioFile() error = %v", err)
	}

	rr = requestJSON(t, env.router, http.MethodPost, fmt.Sprintf("/api/files/%d/segments", f.ID), map[string]interface{}{
		"startMs": 0,
		"endMs":   900,
		"fieldValues": map[string]string{
			"speaker": "controller",
		},
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create segment status = %d; want 201; body=%s", rr.Code, rr.Body.String())
	}

	rr = requestJSON(t, env.router, http.MethodGet, fmt.Sprintf("/api/collections/%d/labeling-queue", collection.ID), nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("labeling queue status = %d; want 200; body=%s", rr.Code, rr.Body.String())
	}
	queue := decodeJSON[[]models.LabelingQueueItem](t, rr)
	if len(queue) != 1 {
		t.Fatalf("queue len = %d; want 1; queue=%#v", len(queue), queue)
	}
	if queue[0].AudioFileID != f.ID || queue[0].Reason != "incomplete_fields" {
		t.Fatalf("queue[0] = %#v; want audioFileId=%d reason=incomplete_fields", queue[0], f.ID)
	}
}

func writeMinimalWav(t *testing.T, path string, sampleRate int, durationMs int64) {
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

// TestHTTP_CreateDataset_PersistsManifestAndZip exercises POST /api/projects/:id/datasets and
// GET /api/datasets/:id/download so export storage behavior stays stable.
func TestHTTP_CreateDataset_PersistsManifestAndZip(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	rr := requestJSON(t, env.router, http.MethodPost, "/api/projects", map[string]string{"name": "P"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", rr.Code, rr.Body.String())
	}
	project := decodeJSON[models.Project](t, rr)

	rr = requestJSON(t, env.router, http.MethodPost, fmt.Sprintf("/api/projects/%d/collections", project.ID), map[string]string{"name": "C"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create collection: %d %s", rr.Code, rr.Body.String())
	}
	collection := decodeJSON[models.Collection](t, rr)

	schema := `{"version":1,"fields":[` +
		`{"id":"speaker","type":"taxonomy","title":"Speaker","required":true,"scope":"segment"},` +
		`{"id":"phrase_type","type":"taxonomy","title":"Phrase","required":true,"scope":"segment"},` +
		`{"id":"source","type":"text","title":"Source","required":false,"scope":"file"}` +
		`]}`
	rr = requestJSON(t, env.router, http.MethodPatch, fmt.Sprintf("/api/collections/%d", collection.ID), map[string]string{
		"fieldSchemaJson": schema,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("patch schema: %d %s", rr.Code, rr.Body.String())
	}

	if err := env.layout.EnsureCollectionDir(project.ID, collection.ID); err != nil {
		t.Fatal(err)
	}
	stored := "a.wav"
	writeMinimalWav(t, filepath.Join(env.layout.CollectionDir(project.ID, collection.ID), stored), 8000, 2000)

	af, err := env.repo.InsertAudioFile(ctx, collection.ID, stored, stored, "wav", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.repo.UpdateAudioFileFieldValues(ctx, af.ID, `{"source":"tower"}`); err != nil {
		t.Fatal(err)
	}

	pilot, err := env.repo.UpsertLabel(ctx, project.ID, "pilot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.repo.UpsertLabel(ctx, project.ID, "command"); err != nil {
		t.Fatal(err)
	}

	fv := `{"speaker":"pilot","phrase_type":"command"}`
	lid := pilot.ID
	if _, err := env.repo.CreateSegment(ctx, af.ID, 0, 400, &lid, nil, fv); err != nil {
		t.Fatal(err)
	}

	rr = requestJSON(t, env.router, http.MethodPost, fmt.Sprintf("/api/projects/%d/datasets", project.ID), map[string]interface{}{
		"name":                   "export1",
		"trainRatio":             1.0,
		"validationRatio":        0.0,
		"evaluationRatio":        0.0,
		"requireTranscription":   false,
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create dataset: %d %s", rr.Code, rr.Body.String())
	}
	ds := decodeJSON[models.Dataset](t, rr)
	if ds.StorageRoot == "" {
		t.Fatal("dataset storageRoot empty")
	}

	manifestPath := filepath.Join(ds.StorageRoot, "manifest.json")
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest on disk: %v", err)
	}
	var man struct {
		Samples []struct {
			Label    string            `json:"label"`
			Fields   map[string]string `json:"fields"`
			Filename string            `json:"filename"`
		} `json:"samples"`
	}
	if err := json.Unmarshal(b, &man); err != nil {
		t.Fatalf("manifest json: %v", err)
	}
	if len(man.Samples) != 1 {
		t.Fatalf("manifest samples: %d", len(man.Samples))
	}
	if man.Samples[0].Label != "pilot" {
		t.Fatalf("manifest label = %q; want pilot", man.Samples[0].Label)
	}
	if man.Samples[0].Fields["phrase_type"] != "command" || man.Samples[0].Fields["source"] != "tower" {
		t.Fatalf("manifest fields = %#v", man.Samples[0].Fields)
	}
	outWav := filepath.Join(ds.StorageRoot, man.Samples[0].Filename)
	if st, err := os.Stat(outWav); err != nil || st.Size() == 0 {
		t.Fatalf("exported wav: %v size=%v", err, st)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/datasets/%d/download", ds.ID), nil)
	rrZip := httptest.NewRecorder()
	env.router.ServeHTTP(rrZip, req)
	if rrZip.Code != http.StatusOK {
		t.Fatalf("download zip: %d %s", rrZip.Code, rrZip.Body.String())
	}
	zbody := rrZip.Body.Bytes()
	if len(zbody) < 4 || zbody[0] != 'P' || zbody[1] != 'K' {
		t.Fatalf("response is not a zip (missing PK header)")
	}
}

