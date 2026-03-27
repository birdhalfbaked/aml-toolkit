package fieldschema

import (
	"encoding/json"
	"fmt"
	"strings"
)

const DefaultVersion = 1

// Field scope: segment = per segment row; file = per audio file row.
const (
	FieldScopeSegment = "segment"
	FieldScopeFile    = "file"
)

// FieldSchema is stored in collections.field_schema_json.
type FieldSchema struct {
	Version int          `json:"version"`
	Fields  []FieldDef   `json:"fields"`
}

type FieldDef struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // text | textarea | taxonomy
	Title    string `json:"title"`
	Required bool   `json:"required"`
	Scope    string `json:"scope,omitempty"` // segment (default) | file
}

// NormalizedScope returns segment or file; empty defaults to segment.
func NormalizedScope(f FieldDef) string {
	if strings.TrimSpace(f.Scope) == "" || strings.EqualFold(f.Scope, FieldScopeSegment) {
		return FieldScopeSegment
	}
	return FieldScopeFile
}

// SegmentFields returns defs with segment scope.
func SegmentFields(s *FieldSchema) []FieldDef {
	if s == nil {
		return nil
	}
	var out []FieldDef
	for _, f := range s.Fields {
		if NormalizedScope(f) == FieldScopeSegment {
			out = append(out, f)
		}
	}
	return out
}

// FileFields returns defs with file scope.
func FileFields(s *FieldSchema) []FieldDef {
	if s == nil {
		return nil
	}
	var out []FieldDef
	for _, f := range s.Fields {
		if NormalizedScope(f) == FieldScopeFile {
			out = append(out, f)
		}
	}
	return out
}

// MergeFieldValuesForExport overlays segment values on file values (segment wins on key conflict).
func MergeFieldValuesForExport(fileMap, segMap map[string]string) map[string]string {
	out := map[string]string{}
	if fileMap != nil {
		for k, v := range fileMap {
			out[k] = v
		}
	}
	if segMap != nil {
		for k, v := range segMap {
			out[k] = v
		}
	}
	return out
}

// DefaultSchemaJSON returns the legacy label + transcription shape.
func DefaultSchemaJSON() string {
	s := FieldSchema{
		Version: DefaultVersion,
		Fields: []FieldDef{
			{ID: "label", Type: "taxonomy", Title: "Label", Required: true, Scope: FieldScopeSegment},
			{ID: "transcription", Type: "textarea", Title: "Transcription", Required: false, Scope: FieldScopeSegment},
		},
	}
	b, _ := json.Marshal(s)
	return string(b)
}

func Parse(data string) (*FieldSchema, error) {
	if strings.TrimSpace(data) == "" {
		return Parse(DefaultSchemaJSON())
	}
	var s FieldSchema
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		return nil, err
	}
	if s.Version == 0 {
		s.Version = 1
	}
	if len(s.Fields) == 0 {
		return nil, fmt.Errorf("schema must define at least one field")
	}
	for i := range s.Fields {
		if strings.TrimSpace(s.Fields[i].Scope) == "" {
			s.Fields[i].Scope = FieldScopeSegment
		}
	}
	return &s, nil
}

func Validate(s *FieldSchema) error {
	if s == nil || len(s.Fields) == 0 {
		return fmt.Errorf("invalid schema")
	}
	seen := map[string]struct{}{}
	for _, f := range s.Fields {
		if f.ID == "" {
			return fmt.Errorf("field id required")
		}
		if _, ok := seen[f.ID]; ok {
			return fmt.Errorf("duplicate field id: %s", f.ID)
		}
		seen[f.ID] = struct{}{}
		sc := NormalizedScope(f)
		if f.Type == "taxonomy" {
			if sc != FieldScopeSegment {
				return fmt.Errorf("taxonomy field %q must be segment-scoped", f.ID)
			}
		}
		if sc == FieldScopeFile && f.Type != "text" && f.Type != "textarea" {
			return fmt.Errorf("file-scoped field %q must be text or textarea", f.ID)
		}
		switch f.Type {
		case "text", "textarea", "taxonomy":
		default:
			return fmt.Errorf("invalid field type: %s", f.Type)
		}
	}
	return nil
}

// TaxonomyFieldID returns the id of the *primary* segment-scoped taxonomy field, or "" if none.
// Primary is defined as the first taxonomy field in schema order.
func TaxonomyFieldID(s *FieldSchema) string {
	for _, f := range s.Fields {
		if f.Type == "taxonomy" && NormalizedScope(f) == FieldScopeSegment {
			return f.ID
		}
	}
	return ""
}

// TaxonomyFieldIDs returns all segment-scoped taxonomy ids in schema order.
func TaxonomyFieldIDs(s *FieldSchema) []string {
	if s == nil {
		return nil
	}
	var out []string
	for _, f := range s.Fields {
		if f.Type == "taxonomy" && NormalizedScope(f) == FieldScopeSegment {
			out = append(out, f.ID)
		}
	}
	return out
}

// ValueString returns the trimmed string value for a field id from m.
func ValueString(m map[string]string, id string) string {
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[id])
}

// ExportReady is true if file + segment required fields pass and optional transcription policy holds.
func ExportReady(s *FieldSchema, fileMap, segMap map[string]string, labelID *int64, ln *string, tr *string, requireTranscription bool) bool {
	if !FileCompleteEx(s, fileMap) {
		return false
	}
	if !SegmentCompleteEx(s, segMap, labelID, ln, tr) {
		return false
	}
	if !requireTranscription {
		return true
	}
	ev := EffectiveValues(s, segMap, ln, tr)
	for _, f := range s.Fields {
		if NormalizedScope(f) != FieldScopeSegment || f.Type != "textarea" {
			continue
		}
		if strings.TrimSpace(ev[f.ID]) != "" {
			return true
		}
	}
	return false
}

// PrimaryLabelString returns display label for export filename: taxonomy value or fallback join name.
func PrimaryLabelString(s *FieldSchema, m map[string]string, labelName *string, tr *string) string {
	ev := EffectiveValues(s, m, labelName, tr)
	tid := TaxonomyFieldID(s)
	if tid != "" {
		if v := ev[tid]; v != "" {
			return v
		}
	}
	if labelName != nil && strings.TrimSpace(*labelName) != "" {
		return strings.TrimSpace(*labelName)
	}
	for _, f := range s.Fields {
		if NormalizedScope(f) != FieldScopeSegment || f.Type == "taxonomy" {
			continue
		}
		if v := ev[f.ID]; v != "" {
			return v
		}
	}
	return "sample"
}
