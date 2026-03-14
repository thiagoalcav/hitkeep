package database

import (
	"reflect"
	"testing"

	"hitkeep/internal/api"
)

func TestBuildHitFilterUTMSource(t *testing.T) {
	clause, args := buildHitFilter("utm_source", "newsletter", "h")
	wantClause := " AND COALESCE(NULLIF(TRIM(h.utm_source), ''), '(Unspecified)') = ?"
	if clause != wantClause {
		t.Fatalf("unexpected clause: got %q want %q", clause, wantClause)
	}
	if !reflect.DeepEqual(args, []any{"newsletter"}) {
		t.Fatalf("unexpected args: got %#v want %#v", args, []any{"newsletter"})
	}
}

func TestBuildHitFilterUTMUnspecifiedNormalization(t *testing.T) {
	tests := []string{"unspecified", "(unspecified)"}
	for _, value := range tests {
		clause, args := buildHitFilter("utm_campaign", value, "h")
		wantClause := " AND COALESCE(NULLIF(TRIM(h.utm_campaign), ''), '(Unspecified)') = ?"
		if clause != wantClause {
			t.Fatalf("unexpected clause for %q: got %q want %q", value, clause, wantClause)
		}
		if !reflect.DeepEqual(args, []any{"(Unspecified)"}) {
			t.Fatalf("unexpected args for %q: got %#v want %#v", value, args, []any{"(Unspecified)"})
		}
	}
}

func TestBuildHitFiltersIncludesUTMClauses(t *testing.T) {
	filters := []api.Filter{
		{Type: "utm_medium", Value: "paid"},
		{Type: "path", Value: "/pricing"},
	}
	clause, args := buildHitFilters(filters, "h")
	wantClause := " AND COALESCE(NULLIF(TRIM(h.utm_medium), ''), '(Unspecified)') = ? AND h.path = ?"
	if clause != wantClause {
		t.Fatalf("unexpected clause: got %q want %q", clause, wantClause)
	}
	if !reflect.DeepEqual(args, []any{"paid", "/pricing"}) {
		t.Fatalf("unexpected args: got %#v want %#v", args, []any{"paid", "/pricing"})
	}
}

func TestBuildHitFilterLanguageNormalizesBaseCode(t *testing.T) {
	clause, args := buildHitFilter("language", "de-DE", "h")
	wantClause := " AND CASE WHEN NULLIF(TRIM(h.language), '') IS NULL THEN '(Unspecified)' ELSE lower(split_part(TRIM(h.language), '-', 1)) END = ?"
	if clause != wantClause {
		t.Fatalf("unexpected clause: got %q want %q", clause, wantClause)
	}
	if !reflect.DeepEqual(args, []any{"de"}) {
		t.Fatalf("unexpected args: got %#v want %#v", args, []any{"de"})
	}
}
