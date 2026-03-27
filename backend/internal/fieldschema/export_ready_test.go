package fieldschema

import "testing"

func TestExportReady_RequireTranscriptionPermutations(t *testing.T) {
	schema := &FieldSchema{
		Version: 1,
		Fields: []FieldDef{
			{ID: "speaker", Type: "taxonomy", Title: "Speaker", Required: true, Scope: FieldScopeSegment},
			{ID: "transcription", Type: "textarea", Title: "T", Required: false, Scope: FieldScopeSegment},
		},
	}
	lid := int64(1)

	tests := []struct {
		name             string
		requireTr        bool
		segMap           map[string]string
		labelID          *int64
		transcriptionCol *string
		want             bool
	}{
		{
			name:      "requireTr_off_empty_textarea_ok",
			requireTr: false,
			segMap:    map[string]string{"speaker": "pilot", "transcription": ""},
			labelID:   &lid,
			want:      true,
		},
		{
			name:      "requireTr_on_empty_textarea_rejected",
			requireTr: true,
			segMap:    map[string]string{"speaker": "pilot", "transcription": "  "},
			labelID:   &lid,
			want:      false,
		},
		{
			name:      "requireTr_on_textarea_in_segMap",
			requireTr: true,
			segMap:    map[string]string{"speaker": "pilot", "transcription": "hello"},
			labelID:   &lid,
			want:      true,
		},
		{
			name:             "requireTr_on_textarea_from_legacy_column",
			requireTr:        true,
			segMap:           map[string]string{"speaker": "pilot"},
			labelID:          &lid,
			transcriptionCol: strPtr("legacy tr"),
			want:             true,
		},
		{
			name:      "requireTr_on_no_textarea_field_in_schema",
			requireTr: true,
			segMap:    map[string]string{"speaker": "pilot"},
			labelID:   &lid,
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := schema
			if tc.name == "requireTr_on_no_textarea_field_in_schema" {
				s = &FieldSchema{
					Version: 1,
					Fields: []FieldDef{
						{ID: "speaker", Type: "taxonomy", Title: "Speaker", Required: true, Scope: FieldScopeSegment},
					},
				}
			}
			got := ExportReady(s, map[string]string{}, tc.segMap, tc.labelID, nil, tc.transcriptionCol, tc.requireTr)
			if got != tc.want {
				t.Fatalf("ExportReady() = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestExportReady_FileRequiredAndSegmentComplete(t *testing.T) {
	schema := &FieldSchema{
		Version: 1,
		Fields: []FieldDef{
			{ID: "speaker", Type: "taxonomy", Title: "Speaker", Required: true, Scope: FieldScopeSegment},
			{ID: "batch", Type: "text", Title: "Batch", Required: true, Scope: FieldScopeFile},
		},
	}
	lid := int64(1)
	if ExportReady(schema, map[string]string{}, map[string]string{"speaker": "pilot"}, &lid, nil, nil, false) {
		t.Fatal("want false when required file field missing")
	}
	if !ExportReady(schema, map[string]string{"batch": "b1"}, map[string]string{"speaker": "pilot"}, &lid, nil, nil, false) {
		t.Fatal("want true when file + segment required satisfied")
	}
}

func TestMergeFieldValuesForExport_SegmentWins(t *testing.T) {
	file := map[string]string{"k": "file", "onlyF": "1"}
	seg := map[string]string{"k": "segment", "onlyS": "2"}
	got := MergeFieldValuesForExport(file, seg)
	if got["k"] != "segment" || got["onlyF"] != "1" || got["onlyS"] != "2" {
		t.Fatalf("merge = %#v", got)
	}
}

func strPtr(s string) *string { return &s }
