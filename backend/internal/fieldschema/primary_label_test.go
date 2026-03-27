package fieldschema

import "testing"

func TestPrimaryLabelString_FirstTaxonomyThenLegacyName(t *testing.T) {
	s := &FieldSchema{
		Version: 1,
		Fields: []FieldDef{
			{ID: "a", Type: "taxonomy", Title: "A", Required: true, Scope: FieldScopeSegment},
			{ID: "b", Type: "taxonomy", Title: "B", Required: false, Scope: FieldScopeSegment},
		},
	}
	ln := "legacy"
	if got := PrimaryLabelString(s, map[string]string{"a": "first", "b": "second"}, &ln, nil); got != "first" {
		t.Fatalf("got %q; want first taxonomy", got)
	}
	if got := PrimaryLabelString(s, map[string]string{"b": "second"}, &ln, nil); got != "legacy" {
		t.Fatalf("got %q; want legacy label join when primary taxonomy empty", got)
	}
}
