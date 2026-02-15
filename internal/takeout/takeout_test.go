package takeout

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
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
		UTMSource:   strPtr("newsletter"),
		UTMMedium:   strPtr("email"),
		UTMCampaign: strPtr("feb-launch"),
		UTMTerm:     strPtr("feature"),
		UTMContent:  strPtr("button-a"),
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

func strPtr(v string) *string {
	return &v
}
