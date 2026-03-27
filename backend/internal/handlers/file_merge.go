package handlers

import (
	"encoding/json"
	"fmt"

	"com.birdhalfbaked.aml-toolkit/internal/fieldschema"
	"com.birdhalfbaked.aml-toolkit/internal/models"
)

type filePatchBody struct {
	FieldValues map[string]string `json:"fieldValues"`
}

func mergeFileFieldPayload(col *models.Collection, body *filePatchBody, existing map[string]string) (string, error) {
	schema, err := fieldschema.Parse(col.FieldSchemaJSON)
	if err != nil {
		schema, _ = fieldschema.Parse(fieldschema.DefaultSchemaJSON())
	}
	for k := range body.FieldValues {
		allowed := false
		for _, f := range schema.Fields {
			if f.ID != k {
				continue
			}
			if fieldschema.NormalizedScope(f) == fieldschema.FieldScopeFile {
				allowed = true
				break
			}
			return "", fmt.Errorf("field %q is not file-scoped", k)
		}
		if !allowed {
			return "", fmt.Errorf("unknown field id: %s", k)
		}
	}
	out := map[string]string{}
	for _, f := range schema.Fields {
		if fieldschema.NormalizedScope(f) != fieldschema.FieldScopeFile {
			continue
		}
		if existing != nil {
			if v, ok := existing[f.ID]; ok {
				out[f.ID] = v
			}
		}
	}
	for k, v := range body.FieldValues {
		out[k] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
