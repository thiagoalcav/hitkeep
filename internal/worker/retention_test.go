package worker

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
)

func TestRetentionArchivesAndPrunesUTMHits(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "retention.db")
	archiveDir := filepath.Join(tmpDir, "archive")

	store := database.NewStore(dbPath)
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID, err := store.CreateUser(ctx, "retention@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	site, err := store.CreateSite(ctx, userID, "retention.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	if _, err := store.DB().ExecContext(ctx, "UPDATE sites SET data_retention_days = ? WHERE id = ?", 1, site.ID); err != nil {
		t.Fatalf("set retention policy: %v", err)
	}

	old := time.Now().UTC().Add(-48 * time.Hour)
	isUnique := true
	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:      site.ID,
		SessionID:   uuid.New(),
		PageID:      uuid.New(),
		Timestamp:   old,
		Path:        "/old-utm",
		UTMSource:   strPtr("search"),
		UTMMedium:   strPtr("paid"),
		UTMCampaign: strPtr("retention-check"),
		UTMTerm:     strPtr("audit"),
		UTMContent:  strPtr("copy-a"),
		IsUnique:    &isUnique,
	}); err != nil {
		t.Fatalf("create hit: %v", err)
	}
	if err := store.CreateEvent(ctx, &api.Event{
		SiteID:     site.ID,
		SessionID:  uuid.New(),
		Name:       "old_event",
		Properties: map[string]any{"kind": "test"},
		Timestamp:  old,
	}); err != nil {
		t.Fatalf("create event: %v", err)
	}

	worker := NewRetentionWorker(store, archiveDir, 365)
	if err := worker.Run(ctx); err != nil {
		t.Fatalf("run retention: %v", err)
	}

	files, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("read archive dir: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("expected retention archive file, found none")
	}

	var archivePath string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".parquet") {
			archivePath = filepath.Join(archiveDir, f.Name())
			break
		}
	}
	if archivePath == "" {
		t.Fatalf("expected parquet archive file, found: %v", files)
	}

	safePath := strings.ReplaceAll(archivePath, "'", "''")
	query := fmt.Sprintf("SELECT utm_source, utm_campaign FROM read_parquet('%s') WHERE utm_source IS NOT NULL LIMIT 1", safePath)
	var utmSource sql.NullString
	var utmCampaign sql.NullString
	if err := store.DB().QueryRowContext(ctx, query).Scan(&utmSource, &utmCampaign); err != nil {
		t.Fatalf("query archived parquet: %v", err)
	}
	if !utmSource.Valid || utmSource.String != "search" {
		t.Fatalf("expected archived utm_source=search, got %q (valid=%v)", utmSource.String, utmSource.Valid)
	}
	if !utmCampaign.Valid || utmCampaign.String != "retention-check" {
		t.Fatalf("expected archived utm_campaign=retention-check, got %q (valid=%v)", utmCampaign.String, utmCampaign.Valid)
	}

	var remainingHits int
	if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM hits WHERE site_id = ?", site.ID).Scan(&remainingHits); err != nil {
		t.Fatalf("count remaining hits: %v", err)
	}
	if remainingHits != 0 {
		t.Fatalf("expected 0 retained hits in hot storage, got %d", remainingHits)
	}

	var remainingEvents int
	if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE site_id = ?", site.ID).Scan(&remainingEvents); err != nil {
		t.Fatalf("count remaining events: %v", err)
	}
	if remainingEvents != 0 {
		t.Fatalf("expected 0 retained events in hot storage, got %d", remainingEvents)
	}
}

func strPtr(v string) *string {
	return &v
}
