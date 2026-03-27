package fieldschema

import "testing"

func TestValidate_AllowsMultipleTaxonomyFields(t *testing.T) {
	s := &FieldSchema{
		Version: 1,
		Fields: []FieldDef{
			{ID: "speaker", Type: "taxonomy", Title: "Speaker", Required: true, Scope: FieldScopeSegment},
			{ID: "phrase_type", Type: "taxonomy", Title: "Phrase type", Required: true, Scope: FieldScopeSegment},
			{ID: "notes", Type: "textarea", Title: "Notes", Required: false, Scope: FieldScopeSegment},
		},
	}
	if err := Validate(s); err != nil {
		t.Fatalf("Validate() error = %v; want nil", err)
	}
}

func TestTaxonomyFieldIDs_PrimaryAndAll(t *testing.T) {
	s := &FieldSchema{
		Version: 1,
		Fields: []FieldDef{
			{ID: "speaker", Type: "taxonomy", Title: "Speaker", Required: true, Scope: FieldScopeSegment},
			{ID: "phrase_type", Type: "taxonomy", Title: "Phrase type", Required: false, Scope: FieldScopeSegment},
		},
	}
	if got := TaxonomyFieldID(s); got != "speaker" {
		t.Fatalf("TaxonomyFieldID() = %q; want %q", got, "speaker")
	}
	got := TaxonomyFieldIDs(s)
	if len(got) != 2 || got[0] != "speaker" || got[1] != "phrase_type" {
		t.Fatalf("TaxonomyFieldIDs() = %#v; want [%q %q]", got, "speaker", "phrase_type")
	}
}

func TestSegmentCompleteEx_MultipleRequiredTaxonomies_BackCompatPrimaryLabelID(t *testing.T) {
	s := &FieldSchema{
		Version: 1,
		Fields: []FieldDef{
			{ID: "speaker", Type: "taxonomy", Title: "Speaker", Required: true, Scope: FieldScopeSegment},
			{ID: "phrase_type", Type: "taxonomy", Title: "Phrase type", Required: true, Scope: FieldScopeSegment},
		},
	}

	// Primary taxonomy can be satisfied by legacy label_id even if no taxonomy value is present.
	lid := int64(123)
	if ok := SegmentCompleteEx(s, map[string]string{"phrase_type": "command"}, &lid, nil, nil); !ok {
		t.Fatalf("SegmentCompleteEx() = false; want true when labelID satisfies primary taxonomy and other taxonomy has value")
	}

	// Non-primary required taxonomy must still have an effective value.
	if ok := SegmentCompleteEx(s, map[string]string{}, &lid, nil, nil); ok {
		t.Fatalf("SegmentCompleteEx() = true; want false when secondary taxonomy missing")
	}
}

