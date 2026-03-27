package fieldschema

import "strings"

// EffectiveValues merges segment field_values JSON with legacy label_name and transcription columns (segment-scoped fields only).
func EffectiveValues(s *FieldSchema, m map[string]string, labelName *string, transcription *string) map[string]string {
	out := map[string]string{}
	if s == nil {
		return out
	}
	for _, f := range s.Fields {
		if NormalizedScope(f) != FieldScopeSegment {
			continue
		}
		if m != nil {
			out[f.ID] = m[f.ID]
		}
	}
	taxID := TaxonomyFieldID(s)
	if taxID != "" {
		if strings.TrimSpace(out[taxID]) == "" && labelName != nil {
			out[taxID] = strings.TrimSpace(*labelName)
		}
	}
	for _, f := range s.Fields {
		if NormalizedScope(f) != FieldScopeSegment || f.Type != "textarea" {
			continue
		}
		if strings.TrimSpace(out[f.ID]) == "" && transcription != nil {
			out[f.ID] = strings.TrimSpace(*transcription)
		}
	}
	return out
}

// FileCompleteEx checks required file-scoped fields.
func FileCompleteEx(s *FieldSchema, m map[string]string) bool {
	if s == nil {
		return true
	}
	for _, f := range s.Fields {
		if NormalizedScope(f) != FieldScopeFile || !f.Required {
			continue
		}
		if strings.TrimSpace(ValueString(m, f.ID)) == "" {
			return false
		}
	}
	return true
}

// SegmentCompleteEx checks required segment-scoped fields using merged effective values.
func SegmentCompleteEx(s *FieldSchema, m map[string]string, labelID *int64, labelName *string, transcription *string) bool {
	if s == nil {
		return labelID != nil
	}
	ev := EffectiveValues(s, m, labelName, transcription)
	primaryTaxID := TaxonomyFieldID(s)
	for _, f := range s.Fields {
		if NormalizedScope(f) != FieldScopeSegment || !f.Required {
			continue
		}
		if f.Type == "taxonomy" {
			// Back-compat: the primary taxonomy can be satisfied by legacy label_id OR by effective value.
			if f.ID == primaryTaxID && labelID != nil {
				continue
			}
			return strings.TrimSpace(ev[f.ID]) != ""
		}
		if strings.TrimSpace(ev[f.ID]) == "" {
			return false
		}
	}
	return true
}
