package handlers

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"com.birdhalfbaked.aml-toolkit/internal/audio"
	"com.birdhalfbaked.aml-toolkit/internal/dataset"
	"com.birdhalfbaked.aml-toolkit/internal/models"
	"com.birdhalfbaked.aml-toolkit/internal/repo"
	"com.birdhalfbaked.aml-toolkit/internal/store"

	"github.com/julienschmidt/httprouter"

	"com.birdhalfbaked.aml-toolkit/internal/fieldschema"
)

type Server struct {
	Repo   *repo.Repo
	Layout *store.Layout
}

func (s *Server) Register(r *httprouter.Router) {
	r.GET("/api/projects", s.cors(s.handleListProjects))
	r.POST("/api/projects", s.cors(s.handleCreateProject))
	r.GET("/api/projects/:id", s.cors(s.handleGetProject))
	r.GET("/api/projects/:id/collections", s.cors(s.handleListCollections))
	r.POST("/api/projects/:id/collections", s.cors(s.handleCreateCollection))
	r.GET("/api/collections/:id", s.cors(s.handleGetCollection))
	r.PATCH("/api/collections/:id", s.cors(s.handlePatchCollection))
	r.GET("/api/projects/:id/labels", s.cors(s.handleListLabels))
	r.POST("/api/projects/:id/labels", s.cors(s.handleCreateLabel))
	r.POST("/api/projects/:id/datasets", s.cors(s.handleCreateDataset))
	r.GET("/api/projects/:id/datasets", s.cors(s.handleListDatasets))

	r.POST("/api/collections/:id/upload", s.cors(s.handleUpload))
	r.GET("/api/collections/:id/files", s.cors(s.handleListFiles))
	r.GET("/api/collections/:id/labeling-queue", s.cors(s.handleLabelingQueue))

	r.GET("/api/files/:id/audio", s.cors(s.handleAudioFile))
	r.GET("/api/files/:id/segments", s.cors(s.handleListSegments))
	r.POST("/api/files/:id/segments", s.cors(s.handleCreateSegment))
	r.GET("/api/files/:id", s.cors(s.handleGetAudioFileRecord))
	r.PATCH("/api/files/:id", s.cors(s.handlePatchAudioFileRecord))
	r.DELETE("/api/files/:id", s.cors(s.handleDeleteAudioFile))
	r.PATCH("/api/segments/:id", s.cors(s.handleUpdateSegment))
	r.DELETE("/api/segments/:id", s.cors(s.handleDeleteSegment))
	r.POST("/api/segments/:id/trim-silence", s.cors(s.handleTrimSilence))

	r.GET("/api/datasets/:id", s.cors(s.handleGetDataset))
	r.GET("/api/datasets/:id/samples", s.cors(s.handleListDatasetSamples))
	r.GET("/api/datasets/:id/samples/:sampleId/audio", s.cors(s.handleDatasetSampleAudio))
	r.GET("/api/datasets/:id/download", s.cors(s.handleDatasetDownload))
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	p, err := s.Repo.GetProject(r.Context(), id)
	if err != nil || p == nil {
		WriteError(w, 404, "not found")
		return
	}
	WriteJSON(w, 200, p)
}

func (s *Server) cors(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r, ps)
	}
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	list, err := s.Repo.ListProjects(r.Context())
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 200, list)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var body struct {
		Name string `json:"name"`
	}
	if err := ReadJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		WriteError(w, 400, "name required")
		return
	}
	p, err := s.Repo.CreateProject(r.Context(), strings.TrimSpace(body.Name))
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	_ = s.Layout.EnsureProjectDir(p.ID)
	WriteJSON(w, 201, p)
}

func (s *Server) handleListCollections(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	list, err := s.Repo.ListCollections(r.Context(), pid)
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 200, list)
}

func (s *Server) handleCreateCollection(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := ReadJSON(r, &body); err != nil || strings.TrimSpace(body.Name) == "" {
		WriteError(w, 400, "name required")
		return
	}
	c, err := s.Repo.CreateCollection(r.Context(), pid, strings.TrimSpace(body.Name))
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	if err := s.Layout.EnsureCollectionDir(pid, c.ID); err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 201, c)
}

func (s *Server) handleGetCollection(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	c, err := s.Repo.GetCollection(r.Context(), id)
	if err != nil || c == nil {
		WriteError(w, 404, "not found")
		return
	}
	WriteJSON(w, 200, c)
}

func (s *Server) handlePatchCollection(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	var body struct {
		FieldSchemaJSON string `json:"fieldSchemaJson"`
	}
	if err := ReadJSON(r, &body); err != nil {
		WriteError(w, 400, "bad json")
		return
	}
	if strings.TrimSpace(body.FieldSchemaJSON) == "" {
		WriteError(w, 400, "fieldSchemaJson required")
		return
	}
	schema, err := fieldschema.Parse(body.FieldSchemaJSON)
	if err != nil {
		WriteError(w, 400, err.Error())
		return
	}
	if err := fieldschema.Validate(schema); err != nil {
		WriteError(w, 400, err.Error())
		return
	}
	if err := s.Repo.UpdateCollectionFieldSchema(r.Context(), id, body.FieldSchemaJSON); err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	c, err := s.Repo.GetCollection(r.Context(), id)
	if err != nil || c == nil {
		WriteError(w, 404, "not found")
		return
	}
	WriteJSON(w, 200, c)
}

func (s *Server) handleListLabels(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	list, err := s.Repo.ListLabels(r.Context(), pid)
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 200, list)
}

func (s *Server) handleCreateLabel(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := ReadJSON(r, &body); err != nil {
		WriteError(w, 400, "bad json")
		return
	}
	l, err := s.Repo.UpsertLabel(r.Context(), pid, body.Name)
	if err != nil {
		WriteError(w, 400, err.Error())
		return
	}
	WriteJSON(w, 201, l)
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	cid, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	col, err := s.Repo.GetCollection(r.Context(), cid)
	if err != nil || col == nil {
		WriteError(w, 404, "collection not found")
		return
	}
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		WriteError(w, 400, err.Error())
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		WriteError(w, 400, "no files")
		return
	}
	var out []models.AudioFile
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		name := fh.Filename
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".zip") {
			_ = f.Close()
			zf, err := fh.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(zf)
			_ = zf.Close()
			if err != nil {
				WriteError(w, 400, err.Error())
				return
			}
			added, err := s.extractZip(r.Context(), col, data)
			if err != nil {
				WriteError(w, 400, err.Error())
				return
			}
			out = append(out, added...)
			continue
		}
		if strings.HasSuffix(lower, ".wav") || strings.HasSuffix(lower, ".mp3") {
			stored, format := safeName(name), formatFromExt(lower)
			dest := filepath.Join(s.Layout.CollectionDir(col.ProjectID, col.ID), stored)
			if err := saveUploadToFile(f, dest); err != nil {
				_ = f.Close()
				WriteError(w, 500, err.Error())
				return
			}
			_ = f.Close()
			var dur *int64
			if format == "wav" {
				if d, err := wavDuration(dest); err == nil {
					dur = &d
				}
			}
			a, err := s.Repo.InsertAudioFile(r.Context(), col.ID, stored, name, format, dur)
			if err != nil {
				WriteError(w, 500, err.Error())
				return
			}
			out = append(out, *a)
			continue
		}
		_ = f.Close()
	}
	WriteJSON(w, 201, out)
}

func saveUploadToFile(r io.Reader, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

func formatFromExt(lower string) string {
	if strings.HasSuffix(lower, ".mp3") {
		return "mp3"
	}
	return "wav"
}

func safeName(name string) string {
	base := filepath.Base(name)
	base = strings.ReplaceAll(base, "..", "")
	return base
}

func (s *Server) extractZip(ctx context.Context, col *models.Collection, data []byte) ([]models.AudioFile, error) {
	z, err := zip.NewReader(bytesReaderAt{data: data}, int64(len(data)))
	if err != nil {
		return nil, err
	}
	var out []models.AudioFile
	seen := map[string]int{}
	for _, zf := range z.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		ln := strings.ToLower(zf.Name)
		if !strings.HasSuffix(ln, ".wav") {
			continue
		}
		flat := flattenZipName(zf.Name)
		if flat == "" {
			continue
		}
		stored := flat
		if n, ok := seen[stored]; ok {
			n++
			seen[stored] = n
			stored = strings.TrimSuffix(flat, ".wav") + "_" + itoa(n) + ".wav"
		} else {
			seen[stored] = 0
		}
		rc, err := zf.Open()
		if err != nil {
			continue
		}
		dest := filepath.Join(s.Layout.CollectionDir(col.ProjectID, col.ID), stored)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			_ = rc.Close()
			return nil, err
		}
		outf, err := os.Create(dest)
		if err != nil {
			_ = rc.Close()
			return nil, err
		}
		_, err = io.Copy(outf, rc)
		_ = rc.Close()
		_ = outf.Close()
		if err != nil {
			return nil, err
		}
		var dur *int64
		if d, err := wavDuration(dest); err == nil {
			dur = &d
		}
		a, err := s.Repo.InsertAudioFile(ctx, col.ID, stored, zf.Name, "wav", dur)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, nil
}

type bytesReaderAt struct {
	data []byte
}

func (b bytesReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n = copy(p, b.data[off:])
	if n < len(p) {
		err = io.EOF
	}
	return
}

func flattenZipName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	parts := strings.Split(name, "/")
	base := strings.Join(parts, "_")
	return safeName(base)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [32]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}

func wavDuration(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	info, err := audio.ReadWavInfo(f)
	if err != nil {
		return 0, err
	}
	return audio.DurationMs(info), nil
}

func (s *Server) handleGetAudioFileRecord(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	a, err := s.Repo.GetAudioFile(r.Context(), id)
	if err != nil || a == nil {
		WriteError(w, 404, "not found")
		return
	}
	WriteJSON(w, 200, a)
}

func (s *Server) handlePatchAudioFileRecord(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	var body filePatchBody
	if err := ReadJSON(r, &body); err != nil {
		WriteError(w, 400, "bad json")
		return
	}
	if body.FieldValues == nil {
		WriteError(w, 400, "fieldValues required")
		return
	}
	a, err := s.Repo.GetAudioFile(r.Context(), id)
	if err != nil || a == nil {
		WriteError(w, 404, "not found")
		return
	}
	col, err := s.Repo.GetCollection(r.Context(), a.CollectionID)
	if err != nil || col == nil {
		WriteError(w, 404, "collection not found")
		return
	}
	jsonStr, err := mergeFileFieldPayload(col, &body, a.FieldValues)
	if err != nil {
		WriteError(w, 400, err.Error())
		return
	}
	if err := s.Repo.UpdateAudioFileFieldValues(r.Context(), id, jsonStr); err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	a, err = s.Repo.GetAudioFile(r.Context(), id)
	if err != nil || a == nil {
		WriteError(w, 404, "not found")
		return
	}
	WriteJSON(w, 200, a)
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	cid, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	list, err := s.Repo.ListAudioFiles(r.Context(), cid)
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 200, list)
}

func (s *Server) handleLabelingQueue(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	cid, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	list, err := s.Repo.LabelingQueue(r.Context(), cid)
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 200, list)
}

func (s *Server) handleAudioFile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	a, err := s.Repo.GetAudioFile(r.Context(), id)
	if err != nil || a == nil {
		WriteError(w, 404, "not found")
		return
	}
	col, err := s.Repo.GetCollection(r.Context(), a.CollectionID)
	if err != nil || col == nil {
		WriteError(w, 404, "not found")
		return
	}
	path := filepath.Join(s.Layout.CollectionDir(col.ProjectID, col.ID), a.StoredFilename)
	f, err := os.Open(path)
	if err != nil {
		WriteError(w, 404, "file missing")
		return
	}
	defer f.Close()
	if a.Format == "mp3" {
		w.Header().Set("Content-Type", "audio/mpeg")
	} else {
		w.Header().Set("Content-Type", "audio/wav")
	}
	w.Header().Set("Accept-Ranges", "bytes")
	_, _ = io.Copy(w, f)
}

func (s *Server) handleDeleteAudioFile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	a, err := s.Repo.GetAudioFile(r.Context(), id)
	if err != nil || a == nil {
		WriteError(w, 404, "not found")
		return
	}
	col, err := s.Repo.GetCollection(r.Context(), a.CollectionID)
	if err != nil || col == nil {
		WriteError(w, 404, "collection not found")
		return
	}
	p := filepath.Join(s.Layout.CollectionDir(col.ProjectID, col.ID), a.StoredFilename)
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		WriteError(w, 500, err.Error())
		return
	}
	if err := s.Repo.DeleteAudioFile(r.Context(), id); err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleListSegments(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	list, err := s.Repo.ListSegments(r.Context(), id)
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 200, list)
}

func (s *Server) handleCreateSegment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fid, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	var body segmentBody
	if err := ReadJSON(r, &body); err != nil {
		WriteError(w, 400, "bad json")
		return
	}
	if body.EndMs <= body.StartMs {
		WriteError(w, 400, "invalid range")
		return
	}
	a, err := s.Repo.GetAudioFile(r.Context(), fid)
	if err != nil || a == nil {
		WriteError(w, 404, "audio not found")
		return
	}
	col, err := s.Repo.GetCollection(r.Context(), a.CollectionID)
	if err != nil || col == nil {
		WriteError(w, 404, "collection not found")
		return
	}
	lid, tr, fvJSON, err := mergeSegmentPayload(r.Context(), s.Repo, col.ProjectID, col, &body, nil)
	if err != nil {
		WriteError(w, 400, err.Error())
		return
	}
	seg, err := s.Repo.CreateSegment(r.Context(), fid, body.StartMs, body.EndMs, lid, tr, fvJSON)
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 201, seg)
}

func (s *Server) handleUpdateSegment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	var body segmentBody
	if err := ReadJSON(r, &body); err != nil {
		WriteError(w, 400, "bad json")
		return
	}
	if body.EndMs <= body.StartMs {
		WriteError(w, 400, "invalid range")
		return
	}
	seg, err := s.Repo.GetSegment(r.Context(), id)
	if err != nil || seg == nil {
		WriteError(w, 404, "segment not found")
		return
	}
	a, err := s.Repo.GetAudioFile(r.Context(), seg.AudioFileID)
	if err != nil || a == nil {
		WriteError(w, 404, "audio not found")
		return
	}
	col, err := s.Repo.GetCollection(r.Context(), a.CollectionID)
	if err != nil || col == nil {
		WriteError(w, 404, "collection not found")
		return
	}
	lid, tr, fvJSON, err := mergeSegmentPayload(r.Context(), s.Repo, col.ProjectID, col, &body, seg)
	if err != nil {
		WriteError(w, 400, err.Error())
		return
	}
	if err := s.Repo.UpdateSegment(r.Context(), id, body.StartMs, body.EndMs, lid, tr, &fvJSON); err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 200, map[string]string{"ok": "true"})
}

func (s *Server) handleDeleteSegment(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	if err := s.Repo.DeleteSegment(r.Context(), id); err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleTrimSilence(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	var body struct {
		Threshold float64 `json:"threshold"`
		WindowMs  int64   `json:"windowMs"`
	}
	_ = ReadJSON(r, &body)
	if body.Threshold <= 0 {
		body.Threshold = 0.02
	}
	if body.WindowMs <= 0 {
		body.WindowMs = 50
	}
	seg, err := s.Repo.GetSegment(r.Context(), id)
	if err != nil || seg == nil {
		WriteError(w, 404, "segment not found")
		return
	}
	a, err := s.Repo.GetAudioFile(r.Context(), seg.AudioFileID)
	if err != nil || a == nil {
		WriteError(w, 404, "audio not found")
		return
	}
	if !strings.EqualFold(a.Format, "wav") {
		WriteError(w, 400, "trim only supported for WAV sources")
		return
	}
	col, err := s.Repo.GetCollection(r.Context(), a.CollectionID)
	if err != nil || col == nil {
		WriteError(w, 404, "collection not found")
		return
	}
	src := filepath.Join(s.Layout.CollectionDir(col.ProjectID, col.ID), a.StoredFilename)
	ns, ne, err := audio.TrimSilenceEdges(src, body.Threshold, body.WindowMs)
	if err != nil {
		WriteError(w, 400, err.Error())
		return
	}
	// intersect with segment bounds
	start := seg.StartMs
	end := seg.EndMs
	if ns > start {
		start = ns
	}
	if ne < end {
		end = ne
	}
	if end <= start {
		WriteError(w, 400, "trim collapsed segment")
		return
	}
	fvB, _ := json.Marshal(seg.FieldValues)
	fvS := string(fvB)
	if err := s.Repo.UpdateSegment(r.Context(), id, start, end, seg.LabelID, seg.Transcription, &fvS); err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 200, map[string]int64{"startMs": start, "endMs": end})
}

func (s *Server) handleCreateDataset(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	var req models.CreateDatasetRequest
	if err := ReadJSON(r, &req); err != nil {
		WriteError(w, 400, "bad json")
		return
	}
	if req.TrainRatio == 0 && req.ValidationRatio == 0 && req.EvaluationRatio == 0 {
		req.TrainRatio, req.ValidationRatio, req.EvaluationRatio = 0.7, 0.15, 0.15
	}
	rows, err := s.Repo.ListSegmentsForExport(r.Context(), pid, req.CollectionIDs)
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	if len(rows) == 0 {
		WriteError(w, 400, "no labeled segments to export")
		return
	}
	ds, err := dataset.Build(r.Context(), s.Repo, s.Layout, pid, req, rows)
	if err != nil {
		WriteError(w, 400, err.Error())
		return
	}
	WriteJSON(w, 201, ds)
}

func (s *Server) handleListDatasets(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	pid, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	list, err := s.Repo.ListDatasets(r.Context(), pid)
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 200, list)
}

func (s *Server) handleGetDataset(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	d, err := s.Repo.GetDataset(r.Context(), id)
	if err != nil || d == nil {
		WriteError(w, 404, "not found")
		return
	}
	WriteJSON(w, 200, d)
}

func (s *Server) handleListDatasetSamples(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	split := r.URL.Query().Get("split")
	var sp *string
	if split != "" {
		sp = &split
	}
	list, err := s.Repo.ListDatasetSamples(r.Context(), id, sp, 5000, 0)
	if err != nil {
		WriteError(w, 500, err.Error())
		return
	}
	WriteJSON(w, 200, list)
}

func (s *Server) handleDatasetSampleAudio(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	did, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	sid, err := ParseID(ps, "sampleId")
	if err != nil {
		WriteError(w, 400, "bad sample id")
		return
	}
	smp, err := s.Repo.GetDatasetSample(r.Context(), did, sid)
	if err != nil || smp == nil {
		WriteError(w, 404, "not found")
		return
	}
	ds, err := s.Repo.GetDataset(r.Context(), did)
	if err != nil || ds == nil {
		WriteError(w, 404, "dataset not found")
		return
	}
	path := filepath.Join(ds.StorageRoot, smp.RelPath)
	f, err := os.Open(path)
	if err != nil {
		WriteError(w, 404, "file missing")
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "audio/wav")
	_, _ = io.Copy(w, f)
}

func (s *Server) handleDatasetDownload(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	id, err := ParseID(ps, "id")
	if err != nil {
		WriteError(w, 400, "bad id")
		return
	}
	ds, err := s.Repo.GetDataset(r.Context(), id)
	if err != nil || ds == nil {
		WriteError(w, 404, "not found")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=dataset_"+strconv.FormatInt(ds.ID, 10)+".zip")
	if err := dataset.ZipDir(ds.StorageRoot, w); err != nil {
		WriteError(w, 500, err.Error())
		return
	}
}
