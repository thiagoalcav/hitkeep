package database

import (
	"bytes"
	"context"
	"encoding/csv"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func TestExportHitsCSVIncludesUTMFields(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hits-export.db")

	store := NewStore(dbPath)
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID, err := store.CreateUser(ctx, "utm-export@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	site, err := store.CreateSite(ctx, userID, "utm-export.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	now := time.Now().UTC()
	isUnique := true
	hit := &api.Hit{
		SiteID:      site.ID,
		SessionID:   uuid.New(),
		PageID:      uuid.New(),
		Timestamp:   now,
		Path:        "/landing",
		UTMSource:   new("google"),
		UTMMedium:   new("cpc"),
		UTMCampaign: new("spring-launch"),
		UTMTerm:     new("privacy analytics"),
		UTMContent:  new("hero-cta"),
		IsUnique:    &isUnique,
	}
	if err := store.CreateHit(ctx, hit); err != nil {
		t.Fatalf("create hit: %v", err)
	}

	params := api.HitQueryParams{
		SiteID: site.ID,
		UserID: userID,
		Start:  now.Add(-time.Hour),
		End:    now.Add(time.Hour),
		Limit:  10,
		Offset: 0,
	}

	var buf bytes.Buffer
	if err := store.ExportHitsCSV(ctx, params, &buf); err != nil {
		t.Fatalf("export csv: %v", err)
	}

	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least header and one row, got %d rows", len(rows))
	}

	header := rows[0]
	row := rows[1]
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[name] = i
	}

	for _, col := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"} {
		if _, ok := index[col]; !ok {
			t.Fatalf("expected header column %q to exist", col)
		}
	}

	if got := row[index["utm_source"]]; got != "google" {
		t.Fatalf("expected utm_source=google, got %q", got)
	}
	if got := row[index["utm_medium"]]; got != "cpc" {
		t.Fatalf("expected utm_medium=cpc, got %q", got)
	}
	if got := row[index["utm_campaign"]]; got != "spring-launch" {
		t.Fatalf("expected utm_campaign=spring-launch, got %q", got)
	}
	if got := row[index["utm_term"]]; got != "privacy analytics" {
		t.Fatalf("expected utm_term=privacy analytics, got %q", got)
	}
	if got := row[index["utm_content"]]; got != "hero-cta" {
		t.Fatalf("expected utm_content=hero-cta, got %q", got)
	}
}
