package dataset

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"com.birdhalfbaked.aml-toolkit/internal/audio"
	"com.birdhalfbaked.aml-toolkit/internal/fieldschema"
	"com.birdhalfbaked.aml-toolkit/internal/models"
	"com.birdhalfbaked.aml-toolkit/internal/repo"
	"com.birdhalfbaked.aml-toolkit/internal/store"
)

type Manifest struct {
	Version   int              `json:"version"`
	DatasetID int64            `json:"datasetId"`
	Name      string           `json:"name"`
	Samples   []ManifestSample `json:"samples"`
}

type ManifestSample struct {
	Split           string            `json:"split"`
	Filename        string            `json:"filename"`
	Label           string            `json:"label"`
	Transcription   *string           `json:"transcription,omitempty"`
	SourceSegmentID int64             `json:"sourceSegmentId"`
	Augmentation    *string           `json:"augmentation,omitempty"`
	Fields          map[string]string `json:"fields,omitempty"`
}

func AssignSplit(r *rand.Rand, train, val, eval float64) string {
	t := train
	v := t + val
	x := r.Float64()
	if x < t {
		return "train"
	}
	if x < v {
		return "validation"
	}
	return "evaluation"
}

func Build(ctx context.Context, rp *repo.Repo, ly *store.Layout, projectID int64, req models.CreateDatasetRequest, rows []repo.SegmentExportRow) (*models.Dataset, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("dataset name required")
	}
	tr := req.TrainRatio
	vr := req.ValidationRatio
	er := req.EvaluationRatio
	if tr+vr+er < 0.999 || tr+vr+er > 1.001 {
		return nil, fmt.Errorf("split ratios must sum to 1")
	}
	var seed int64 = 42
	if req.Seed != nil {
		seed = *req.Seed
	}
	rng := rand.New(rand.NewSource(seed))

	opt, _ := json.Marshal(req)
	ds, err := rp.InsertDataset(ctx, projectID, req.Name, string(opt), "")
	if err != nil {
		return nil, err
	}
	if err := ly.EnsureProjectDir(projectID); err != nil {
		return nil, err
	}
	if err := ly.EnsureDatasetDir(projectID, ds.ID); err != nil {
		return nil, err
	}
	root := ly.DatasetDir(projectID, ds.ID)
	if err := rp.UpdateDatasetStorageRoot(ctx, ds.ID, root); err != nil {
		return nil, err
	}
	ds.StorageRoot = root

	trimRMS := 0.02
	useTrim := req.SilenceTrimRMS != nil
	if useTrim {
		trimRMS = *req.SilenceTrimRMS
	}
	maxAug := req.AugmentVariants
	if maxAug > 10 {
		maxAug = 10
	}

	manifest := Manifest{Version: 1, DatasetID: ds.ID, Name: req.Name}
	idx := 0

	type preparedRow struct {
		row          repo.SegmentExportRow
		schema       *fieldschema.FieldSchema
		segFV        map[string]string
		fileFV       map[string]string
		labelName    *string
		primary      string
		trPtr        *string
		mergedFields map[string]string
	}

	prepared := make([]preparedRow, 0, len(rows))
	for _, row := range rows {
		schema, err := fieldschema.Parse(row.FieldSchemaJSON)
		if err != nil {
			schema, _ = fieldschema.Parse(fieldschema.DefaultSchemaJSON())
		}
		var fv map[string]string
		if err := json.Unmarshal([]byte(row.FieldValuesJSON), &fv); err != nil || fv == nil {
			fv = map[string]string{}
		}
		var fileFV map[string]string
		if err := json.Unmarshal([]byte(row.FileFieldValuesJSON), &fileFV); err != nil || fileFV == nil {
			fileFV = map[string]string{}
		}
		var ln *string
		if row.LabelName != "" {
			t := row.LabelName
			ln = &t
		}
		if !fieldschema.ExportReady(schema, fileFV, fv, row.LabelID, ln, row.Transcription, req.RequireTranscription) {
			continue
		}
		ev := fieldschema.EffectiveValues(schema, fv, ln, row.Transcription)
		primary := fieldschema.PrimaryLabelString(schema, fv, ln, row.Transcription)
		var trPtr *string
		for _, f := range schema.Fields {
			if fieldschema.NormalizedScope(f) != fieldschema.FieldScopeSegment || f.Type != "textarea" {
				continue
			}
			v := strings.TrimSpace(ev[f.ID])
			if v != "" {
				trPtr = &v
			}
			break
		}
		mergedFields := fieldschema.MergeFieldValuesForExport(fileFV, fv)
		prepared = append(prepared, preparedRow{
			row: row, schema: schema, segFV: fv, fileFV: fileFV, labelName: ln,
			primary: primary, trPtr: trPtr, mergedFields: mergedFields,
		})
	}

	// Deterministic split assignment with ratio targets (avoids “all train” on smallish sets).
	total := len(prepared)
	splits := make([]string, total)
	if total > 0 {
		wantTrain := int(math.Floor(tr * float64(total)))
		wantVal := int(math.Floor(vr * float64(total)))
		wantEval := total - wantTrain - wantVal

		if vr > 0 && wantVal == 0 && total >= 2 {
			wantVal = 1
			if wantTrain > 0 {
				wantTrain--
			} else {
				wantEval--
			}
		}
		if er > 0 && wantEval == 0 && total >= 3 {
			wantEval = 1
			if wantTrain > 0 {
				wantTrain--
			} else if wantVal > 0 {
				wantVal--
			}
		}
		if wantTrain < 0 {
			wantTrain = 0
		}
		if wantVal < 0 {
			wantVal = 0
		}
		if wantEval < 0 {
			wantEval = 0
		}
		for wantTrain+wantVal+wantEval < total {
			wantTrain++
		}

		order := rng.Perm(total)
		i := 0
		for ; i < wantTrain && i < total; i++ {
			splits[order[i]] = "train"
		}
		for ; i < wantTrain+wantVal && i < total; i++ {
			splits[order[i]] = "validation"
		}
		for ; i < total; i++ {
			splits[order[i]] = "evaluation"
		}
	}

	for i, pr := range prepared {
		row := pr.row
		split := splits[i]
		colDir := ly.CollectionDir(row.ProjectID, row.CollectionID)
		srcPath := filepath.Join(colDir, row.StoredFilename)
		startMs := row.StartMs
		endMs := row.EndMs

		if useTrim && strings.EqualFold(row.Format, "wav") {
			if ns, ne, err := audio.TrimSilenceEdges(srcPath, trimRMS, 50); err == nil && ne > ns {
				if ns > startMs {
					startMs = ns
				}
				if ne < endMs {
					endMs = ne
				}
			}
		}
		if endMs <= startMs {
			return nil, fmt.Errorf("segment %d collapsed after trim", row.SegmentID)
		}

		idx++
		fname := fmt.Sprintf("%s_%d.wav", sanitize(pr.primary), idx)
		rel := filepath.Join(split, fname)
		outPath := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return nil, err
		}
		if err := extractSegment(srcPath, outPath, row.Format, startMs, endMs); err != nil {
			return nil, fmt.Errorf("segment %d: %w", row.SegmentID, err)
		}

		if _, err := rp.InsertDatasetSample(ctx, ds.ID, split, fname, rel, pr.primary, pr.trPtr, row.SegmentID, nil); err != nil {
			return nil, err
		}
		ms := ManifestSample{
			Split: split, Filename: rel, Label: pr.primary, Transcription: pr.trPtr, SourceSegmentID: row.SegmentID,
		}
		if len(pr.mergedFields) > 0 {
			ms.Fields = pr.mergedFields
		}
		manifest.Samples = append(manifest.Samples, ms)

		augRng := rand.New(rand.NewSource(seed + int64(row.SegmentID)*10007))
		for a := 0; a < maxAug; a++ {
			extra := fmt.Sprintf("%s_aug%d.wav", strings.TrimSuffix(fname, ".wav"), a+1)
			relA := filepath.Join(split, extra)
			dstA := filepath.Join(root, relA)
			steps := make([]map[string]any, 0, 2)

			srcVariant := outPath
			tmpShift := ""

			shiftEnabled := req.AugmentMaxShiftMs != nil && *req.AugmentMaxShiftMs > 0
			noiseEnabled := req.AugmentNoiseDB != nil && *req.AugmentNoiseDB > 0
			if !shiftEnabled && !noiseEnabled {
				continue
			}

			if shiftEnabled {
				maxShiftMs := *req.AugmentMaxShiftMs
				shiftMs := int64(augRng.Int63n(maxShiftMs*2+1)) - maxShiftMs
				if shiftMs == 0 && maxShiftMs > 0 {
					shiftMs = 1
				}
				sampleRate := int64(44100)
				if f, err := os.Open(srcVariant); err == nil {
					if info, err := audio.ReadWavInfo(f); err == nil && info.SampleRate > 0 {
						sampleRate = int64(info.SampleRate)
					}
					_ = f.Close()
				}
				frames := int((float64(sampleRate) * float64(shiftMs)) / 1000.0)
				if frames == 0 {
					if shiftMs > 0 {
						frames = 1
					} else {
						frames = -1
					}
				}

				if noiseEnabled {
					tmpShift = dstA + ".shift.tmp.wav"
					if err := audio.TimeShiftPad(srcVariant, tmpShift, frames); err != nil {
						return nil, err
					}
					srcVariant = tmpShift
				} else {
					if err := audio.TimeShiftPad(srcVariant, dstA, frames); err != nil {
						return nil, err
					}
				}

				steps = append(steps, map[string]any{
					"type":        "shift",
					"mode":        "wrap",
					"shiftMs":     shiftMs,
					"shiftFrames": frames,
					"sampleRate":  sampleRate,
					"maxShiftMs":  maxShiftMs,
				})
			}

			if noiseEnabled {
				maxRMS := *req.AugmentNoiseDB / 100.0
				nrms := augRng.Float64() * maxRMS
				if err := audio.AddNoise(srcVariant, dstA, nrms, augRng); err != nil {
					if tmpShift != "" {
						_ = os.Remove(tmpShift)
					}
					return nil, err
				}
				steps = append(steps, map[string]any{
					"type":         "noise",
					"distribution": "gaussian",
					"noiseRms":     nrms,
					"maxNoiseRms":  maxRMS,
				})
			}

			if tmpShift != "" {
				_ = os.Remove(tmpShift)
			}

			b, _ := json.Marshal(steps)
			m := string(b)
			if _, err := rp.InsertDatasetSample(ctx, ds.ID, split, extra, relA, pr.primary, pr.trPtr, row.SegmentID, &m); err != nil {
				return nil, err
			}
			augSample := ManifestSample{
				Split: split, Filename: relA, Label: pr.primary, Transcription: pr.trPtr, SourceSegmentID: row.SegmentID, Augmentation: &m,
			}
			if len(pr.mergedFields) > 0 {
				augSample.Fields = pr.mergedFields
			}
			manifest.Samples = append(manifest.Samples, augSample)
		}
	}

	mf, err := os.Create(filepath.Join(root, "manifest.json"))
	if err != nil {
		return nil, err
	}
	enc := json.NewEncoder(mf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&manifest); err != nil {
		_ = mf.Close()
		return nil, err
	}
	_ = mf.Close()

	return ds, nil
}

func extractSegment(srcPath, dstPath, format string, startMs, endMs int64) error {
	switch strings.ToLower(format) {
	case "wav":
		return audio.ExtractWavSegment(srcPath, dstPath, startMs, endMs)
	default:
		return audio.ExtractSegmentToWav(srcPath, dstPath, startMs, endMs)
	}
}

func sanitize(s string) string {
	b := make([]rune, 0, len(s))
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b = append(b, r)
		} else {
			b = append(b, '_')
		}
	}
	if len(b) == 0 {
		return "label"
	}
	return string(b)
}

// ExportZipToPath writes a zip of root to destPath. On failure, destPath is removed if it was created.
func ExportZipToPath(root, destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := ZipDir(root, f); err != nil {
		_ = f.Close()
		_ = os.Remove(destPath)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = os.Remove(destPath)
		return err
	}
	return nil
}

// ZipDir streams a zip of dataset root to w.
func ZipDir(root string, w io.Writer) error {
	zw := zip.NewWriter(w)
	defer zw.Close()
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		f, err := zw.Create(rel)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(f, in)
		return err
	})
}
