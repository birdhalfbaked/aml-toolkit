package handlers

import (
	"context"
	"encoding/json"
	"strings"

	"com.birdhalfbaked.aml-toolkit/internal/fieldschema"
	"com.birdhalfbaked.aml-toolkit/internal/models"
	"com.birdhalfbaked.aml-toolkit/internal/repo"
)

type segmentBody struct {
	StartMs       int64             `json:"startMs"`
	EndMs         int64             `json:"endMs"`
	LabelID       *int64            `json:"labelId"`
	Transcription *string           `json:"transcription"`
	FieldValues   map[string]string `json:"fieldValues"`
}

func mergeSegmentPayload(ctx context.Context, rp *repo.Repo, projectID int64, col *models.Collection, body *segmentBody, existing *models.Segment) (labelID *int64, transcription *string, fvJSON string, err error) {
	schema, err := fieldschema.Parse(col.FieldSchemaJSON)
	if err != nil {
		schema, _ = fieldschema.Parse(fieldschema.DefaultSchemaJSON())
	}
	fv := map[string]string{}
	if existing != nil {
		for k, v := range existing.FieldValues {
			fv[k] = v
		}
		primaryTaxID := fieldschema.TaxonomyFieldID(schema)
		if primaryTaxID != "" && existing.LabelID != nil && fieldschema.ValueString(fv, primaryTaxID) == "" {
			lab, e := rp.GetLabelByID(ctx, *existing.LabelID)
			if e != nil {
				return nil, nil, "", e
			}
			if lab != nil {
				fv[primaryTaxID] = lab.Name
			}
		}
	}
	if body.FieldValues != nil {
		for k, v := range body.FieldValues {
			fv[k] = v
		}
	}
	primaryTaxID := fieldschema.TaxonomyFieldID(schema)
	if primaryTaxID != "" {
		// Back-compat: if client sends labelId but not the primary taxonomy value, fill it.
		if body.LabelID != nil {
			lab, e := rp.GetLabelByID(ctx, *body.LabelID)
			if e != nil {
				return nil, nil, "", e
			}
			if lab != nil && fieldschema.ValueString(fv, primaryTaxID) == "" {
				fv[primaryTaxID] = lab.Name
			}
		}

		// Taxonomy fields are categorical strings backed by the project's label set.
		// The first taxonomy field is "primary" and is mirrored into legacy segments.label_id for display/export/back-compat.
		var primaryID *int64
		for _, tid := range fieldschema.TaxonomyFieldIDs(schema) {
			name := fieldschema.ValueString(fv, tid)
			if name == "" {
				continue
			}
			lab, e := rp.UpsertLabel(ctx, projectID, name)
			if e != nil {
				return nil, nil, "", e
			}
			if tid == primaryTaxID {
				id := lab.ID
				primaryID = &id
			}
		}
		if primaryID != nil {
			labelID = primaryID
		} else {
			labelID = body.LabelID
		}
	} else if body.LabelID != nil {
		labelID = body.LabelID
	}
	var tr *string
	for _, f := range schema.Fields {
		if fieldschema.NormalizedScope(f) != fieldschema.FieldScopeSegment || f.Type != "textarea" {
			continue
		}
		if v := fieldschema.ValueString(fv, f.ID); v != "" {
			t := v
			tr = &t
		}
		break
	}
	if body.Transcription != nil {
		tr = body.Transcription
		for _, f := range schema.Fields {
			if fieldschema.NormalizedScope(f) == fieldschema.FieldScopeSegment && f.Type == "textarea" {
				fv[f.ID] = strings.TrimSpace(*body.Transcription)
				break
			}
		}
	}
	segKeys := map[string]struct{}{}
	for _, f := range schema.Fields {
		if fieldschema.NormalizedScope(f) == fieldschema.FieldScopeSegment {
			segKeys[f.ID] = struct{}{}
		}
	}
	filtered := map[string]string{}
	for k, v := range fv {
		if _, ok := segKeys[k]; ok {
			filtered[k] = v
		}
	}
	b, err := json.Marshal(filtered)
	if err != nil {
		return nil, nil, "", err
	}
	return labelID, tr, string(b), nil
}
