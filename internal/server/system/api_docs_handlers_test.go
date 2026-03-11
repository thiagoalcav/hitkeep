package system

import (
	"reflect"
	"strings"
	"testing"

	"hitkeep/internal/exportfmt"
)

func TestOpenAPISpecV1FormatParameterIncludesAllExportFormats(t *testing.T) {
	spec := openAPISpecV1("http://localhost:8080")

	components := requireMap(t, spec, "components")
	parameters := requireMap(t, components, "parameters")
	formatParam := requireMap(t, parameters, "format")
	schema := requireMap(t, formatParam, "schema")

	gotFormats := asStringSlice(t, schema["enum"])
	wantFormats := exportfmt.SupportedFormats()
	if !reflect.DeepEqual(gotFormats, wantFormats) {
		t.Fatalf("unexpected format enum, got %v want %v", gotFormats, wantFormats)
	}

	description, ok := formatParam["description"].(string)
	if !ok {
		t.Fatalf("expected format parameter description to be string, got %T", formatParam["description"])
	}
	if !strings.Contains(description, "json") || !strings.Contains(description, "ndjson") {
		t.Fatalf("format parameter description should mention json/ndjson, got %q", description)
	}
}

func TestOpenAPISpecV1TakeoutAndExportPathsListAllFormats(t *testing.T) {
	spec := openAPISpecV1("http://localhost:8080")
	paths := requireMap(t, spec, "paths")

	tests := []struct {
		path             string
		expectedContains []string
	}{
		{
			path:             "/api/user/takeout",
			expectedContains: []string{"xlsx", "csv", "parquet", "json", "ndjson"},
		},
		{
			path:             "/api/sites/{id}/takeout",
			expectedContains: []string{"xlsx", "csv", "parquet", "json", "ndjson"},
		},
		{
			path:             "/api/sites/{id}/hits/export",
			expectedContains: []string{"xlsx", "csv", "parquet", "json", "ndjson"},
		},
		{
			path:             "/api/share/{token}/sites/{id}/hits/export",
			expectedContains: []string{"xlsx", "csv", "parquet", "json", "ndjson"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			pathItem := requireMap(t, paths, tc.path)
			getOp := requireMap(t, pathItem, "get")

			description, ok := getOp["description"].(string)
			if !ok {
				t.Fatalf("expected description for %s to be string, got %T", tc.path, getOp["description"])
			}

			for _, expected := range tc.expectedContains {
				if !strings.Contains(description, expected) {
					t.Fatalf("expected description for %s to contain %q, got %q", tc.path, expected, description)
				}
			}

			parameters, ok := getOp["parameters"].([]any)
			if !ok {
				t.Fatalf("expected parameters for %s to be []any, got %T", tc.path, getOp["parameters"])
			}
			if !hasFormatParamRef(parameters) {
				t.Fatalf("expected parameters for %s to include format parameter ref", tc.path)
			}
		})
	}
}

func TestOpenAPISpecV1TeamSchemasExposeUsageAndEntitlements(t *testing.T) {
	spec := openAPISpecV1("http://localhost:8080")
	components := requireMap(t, spec, "components")
	schemas := requireMap(t, components, "schemas")

	teamSchema := requireMap(t, schemas, "Team")
	teamProperties := requireMap(t, teamSchema, "properties")

	if _, ok := teamProperties["usage"]; !ok {
		t.Fatalf("expected Team schema to include usage")
	}
	if _, ok := teamProperties["entitlements"]; !ok {
		t.Fatalf("expected Team schema to include entitlements")
	}
	if _, ok := teamProperties["plan"]; !ok {
		t.Fatalf("expected Team schema to include plan")
	}

	if _, ok := schemas["TeamUsageSummary"]; !ok {
		t.Fatalf("expected TeamUsageSummary schema to exist")
	}
	if _, ok := schemas["TeamEntitlements"]; !ok {
		t.Fatalf("expected TeamEntitlements schema to exist")
	}
	if _, ok := schemas["TeamPlan"]; !ok {
		t.Fatalf("expected TeamPlan schema to exist")
	}
	if _, ok := schemas["CloudStatus"]; !ok {
		t.Fatalf("expected CloudStatus schema to exist")
	}
}

func hasFormatParamRef(params []any) bool {
	for _, p := range params {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		ref, ok := pm["$ref"].(string)
		if !ok {
			continue
		}
		if ref == "#/components/parameters/format" {
			return true
		}
	}
	return false
}

func requireMap(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("expected key %q to exist", key)
	}
	out, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected key %q to be map[string]any, got %T", key, raw)
	}
	return out
}

func asStringSlice(t *testing.T, v any) []string {
	t.Helper()
	switch values := v.(type) {
	case []string:
		out := make([]string, len(values))
		copy(out, values)
		return out
	case []any:
		out := make([]string, 0, len(values))
		for _, item := range values {
			str, ok := item.(string)
			if !ok {
				t.Fatalf("expected enum item to be string, got %T", item)
			}
			out = append(out, str)
		}
		return out
	default:
		t.Fatalf("expected []string or []any, got %T", v)
		return nil
	}
}
