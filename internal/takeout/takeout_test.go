package takeout

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
	"hitkeep/internal/exportfmt"
)

func TestExportSiteDataCSVIncludesUTMFields(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "takeout.db")
	exportDir := filepath.Join(t.TempDir(), "exports")

	store := database.NewStore(dbPath)
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID, err := store.CreateUser(ctx, "takeout@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	site, err := store.CreateSite(ctx, userID, "takeout.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	now := time.Now().UTC()
	isUnique := true
	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:      site.ID,
		SessionID:   uuid.New(),
		PageID:      uuid.New(),
		Timestamp:   now,
		Path:        "/utm",
		UTMSource:   new("newsletter"),
		UTMMedium:   new("email"),
		UTMCampaign: new("feb-launch"),
		UTMTerm:     new("feature"),
		UTMContent:  new("button-a"),
		IsUnique:    &isUnique,
	}); err != nil {
		t.Fatalf("create hit: %v", err)
	}

	service := NewTakeoutService(store, exportDir)
	filename, err := service.ExportSiteData(ctx, site.ID, "csv")
	if err != nil {
		t.Fatalf("export site data: %v", err)
	}

	f, err := os.Open(filename)
	if err != nil {
		t.Fatalf("open export file: %v", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least header and one row, got %d rows", len(rows))
	}

	header := rows[0]
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[name] = i
	}
	if _, ok := index["record_type"]; !ok {
		t.Fatalf("expected column %q in takeout export header", "record_type")
	}
	for _, col := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"} {
		if _, ok := index[col]; !ok {
			t.Fatalf("expected column %q in takeout export header", col)
		}
	}

	hitRow := -1
	for i := 1; i < len(rows); i++ {
		if rows[i][index["record_type"]] == "hit" {
			hitRow = i
			break
		}
	}
	if hitRow == -1 {
		t.Fatalf("expected at least one hit row in takeout export")
	}

	row := rows[hitRow]
	if got := row[index["utm_source"]]; got != "newsletter" {
		t.Fatalf("expected utm_source=newsletter, got %q", got)
	}
	if got := row[index["utm_medium"]]; got != "email" {
		t.Fatalf("expected utm_medium=email, got %q", got)
	}
	if got := row[index["utm_campaign"]]; got != "feb-launch" {
		t.Fatalf("expected utm_campaign=feb-launch, got %q", got)
	}
	if got := row[index["utm_term"]]; got != "feature" {
		t.Fatalf("expected utm_term=feature, got %q", got)
	}
	if got := row[index["utm_content"]]; got != "button-a" {
		t.Fatalf("expected utm_content=button-a, got %q", got)
	}
}

func TestExportSiteDataNDJSONIncludesUTMFields(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "takeout.db")
	exportDir := filepath.Join(t.TempDir(), "exports")

	store := database.NewStore(dbPath)
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID, err := store.CreateUser(ctx, "takeout@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	site, err := store.CreateSite(ctx, userID, "takeout.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	now := time.Now().UTC()
	isUnique := true
	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:      site.ID,
		SessionID:   uuid.New(),
		PageID:      uuid.New(),
		Timestamp:   now,
		Path:        "/utm",
		UTMSource:   new("newsletter"),
		UTMMedium:   new("email"),
		UTMCampaign: new("feb-launch"),
		UTMTerm:     new("feature"),
		UTMContent:  new("button-a"),
		IsUnique:    &isUnique,
	}); err != nil {
		t.Fatalf("create hit: %v", err)
	}

	service := NewTakeoutService(store, exportDir)
	filename, err := service.ExportSiteData(ctx, site.ID, "ndjson")
	if err != nil {
		t.Fatalf("export site data: %v", err)
	}
	if ext := strings.ToLower(filepath.Ext(filename)); ext != ".ndjson" {
		t.Fatalf("expected ndjson export extension, got %q", ext)
	}

	f, err := os.Open(filename)
	if err != nil {
		t.Fatalf("open export file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	foundHit := false
	for scanner.Scan() {
		var row map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("decode ndjson row: %v", err)
		}

		recordType, _ := row["record_type"].(string)
		if recordType != "hit" {
			continue
		}
		foundHit = true

		if got, _ := row["utm_source"].(string); got != "newsletter" {
			t.Fatalf("expected utm_source=newsletter, got %q", got)
		}
		if got, _ := row["utm_medium"].(string); got != "email" {
			t.Fatalf("expected utm_medium=email, got %q", got)
		}
		if got, _ := row["utm_campaign"].(string); got != "feb-launch" {
			t.Fatalf("expected utm_campaign=feb-launch, got %q", got)
		}
		if got, _ := row["utm_term"].(string); got != "feature" {
			t.Fatalf("expected utm_term=feature, got %q", got)
		}
		if got, _ := row["utm_content"].(string); got != "button-a" {
			t.Fatalf("expected utm_content=button-a, got %q", got)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan ndjson export: %v", err)
	}
	if !foundHit {
		t.Fatalf("expected at least one hit row in ndjson export")
	}
}

func TestResolveExportFormatCoversAllSupportedFormats(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		wantExt    string
		wantFormat string
	}{
		{name: "csv", format: "csv", wantExt: "csv", wantFormat: "CSV, HEADER"},
		{name: "xlsx", format: "xlsx", wantExt: "xlsx", wantFormat: "XLSX, HEADER true"},
		{name: "parquet", format: "parquet", wantExt: "parquet", wantFormat: "PARQUET, COMPRESSION 'SNAPPY'"},
		{name: "json", format: "json", wantExt: "json", wantFormat: "JSON, ARRAY true"},
		{name: "ndjson", format: "ndjson", wantExt: "ndjson", wantFormat: "JSON"},
		{name: "unknown defaults to xlsx", format: "unknown", wantExt: "xlsx", wantFormat: "XLSX, HEADER true"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotExt := exportfmt.Normalize(tc.format, exportfmt.FormatXLSX)
			gotFormat := exportfmt.DuckDBCopyOptions(gotExt)
			if gotExt != tc.wantExt {
				t.Fatalf("expected ext %q, got %q", tc.wantExt, gotExt)
			}
			if gotFormat != tc.wantFormat {
				t.Fatalf("expected format %q, got %q", tc.wantFormat, gotFormat)
			}
		})
	}
}

func TestExportSiteDataSupportsAllFormats(t *testing.T) {
	ctx, store, service, _, siteID := setupTakeoutFixture(t)
	t.Cleanup(func() { _ = store.Close() })

	tests := []struct {
		name   string
		format string
		ext    string
	}{
		{name: "csv", format: "csv", ext: ".csv"},
		{name: "xlsx", format: "xlsx", ext: ".xlsx"},
		{name: "parquet", format: "parquet", ext: ".parquet"},
		{name: "json", format: "json", ext: ".json"},
		{name: "ndjson", format: "ndjson", ext: ".ndjson"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filename, err := service.ExportSiteData(ctx, siteID, tc.format)
			if err != nil {
				t.Fatalf("export site data %s: %v", tc.format, err)
			}
			if ext := strings.ToLower(filepath.Ext(filename)); ext != tc.ext {
				t.Fatalf("expected extension %q, got %q", tc.ext, ext)
			}
			info, err := os.Stat(filename)
			if err != nil {
				t.Fatalf("stat exported file: %v", err)
			}
			if info.Size() == 0 {
				t.Fatalf("expected non-empty exported file for format %s", tc.format)
			}
		})
	}
}

func TestExportUserDataSupportsAllFormats(t *testing.T) {
	ctx, store, service, userID, _ := setupTakeoutFixture(t)
	t.Cleanup(func() { _ = store.Close() })

	tests := []struct {
		name   string
		format string
		ext    string
	}{
		{name: "csv", format: "csv", ext: ".csv"},
		{name: "xlsx", format: "xlsx", ext: ".xlsx"},
		{name: "parquet", format: "parquet", ext: ".parquet"},
		{name: "json", format: "json", ext: ".json"},
		{name: "ndjson", format: "ndjson", ext: ".ndjson"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filename, err := service.ExportUserData(ctx, userID, tc.format)
			if err != nil {
				t.Fatalf("export user data %s: %v", tc.format, err)
			}
			if ext := strings.ToLower(filepath.Ext(filename)); ext != tc.ext {
				t.Fatalf("expected extension %q, got %q", tc.ext, ext)
			}
			info, err := os.Stat(filename)
			if err != nil {
				t.Fatalf("stat exported file: %v", err)
			}
			if info.Size() == 0 {
				t.Fatalf("expected non-empty exported file for format %s", tc.format)
			}
		})
	}
}

func TestExportSiteDataJSONIncludesUTMFields(t *testing.T) {
	ctx, store, service, _, siteID := setupTakeoutFixture(t)
	t.Cleanup(func() { _ = store.Close() })

	filename, err := service.ExportSiteData(ctx, siteID, "json")
	if err != nil {
		t.Fatalf("export site data json: %v", err)
	}
	if ext := strings.ToLower(filepath.Ext(filename)); ext != ".json" {
		t.Fatalf("expected json export extension, got %q", ext)
	}

	f, err := os.Open(filename)
	if err != nil {
		t.Fatalf("open export file: %v", err)
	}
	defer f.Close()

	var rows []map[string]any
	if err := json.NewDecoder(f).Decode(&rows); err != nil {
		t.Fatalf("decode json export: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least one row in json export")
	}

	foundHit := false
	for _, row := range rows {
		recordType, _ := row["record_type"].(string)
		if recordType != "hit" {
			continue
		}
		foundHit = true

		if got, _ := row["utm_source"].(string); got != "newsletter" {
			t.Fatalf("expected utm_source=newsletter, got %q", got)
		}
		if got, _ := row["utm_medium"].(string); got != "email" {
			t.Fatalf("expected utm_medium=email, got %q", got)
		}
		if got, _ := row["utm_campaign"].(string); got != "feb-launch" {
			t.Fatalf("expected utm_campaign=feb-launch, got %q", got)
		}
		if got, _ := row["utm_term"].(string); got != "feature" {
			t.Fatalf("expected utm_term=feature, got %q", got)
		}
		if got, _ := row["utm_content"].(string); got != "button-a" {
			t.Fatalf("expected utm_content=button-a, got %q", got)
		}
	}
	if !foundHit {
		t.Fatalf("expected at least one hit row in json export")
	}
}

func setupTakeoutFixture(t *testing.T) (context.Context, *database.Store, *TakeoutService, uuid.UUID, uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "takeout.db")
	exportDir := filepath.Join(t.TempDir(), "exports")

	store := database.NewStore(dbPath)
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID, err := store.CreateUser(ctx, "takeout@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	site, err := store.CreateSite(ctx, userID, "takeout.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	now := time.Now().UTC()
	isUnique := true
	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:      site.ID,
		SessionID:   uuid.New(),
		PageID:      uuid.New(),
		Timestamp:   now,
		Path:        "/utm",
		UTMSource:   new("newsletter"),
		UTMMedium:   new("email"),
		UTMCampaign: new("feb-launch"),
		UTMTerm:     new("feature"),
		UTMContent:  new("button-a"),
		IsUnique:    &isUnique,
	}); err != nil {
		t.Fatalf("create hit: %v", err)
	}

	service := NewTakeoutService(store, exportDir)
	return ctx, store, service, userID, site.ID
}
