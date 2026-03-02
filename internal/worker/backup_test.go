package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
)

func TestBackupExportsSharedDatabase(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	backupDir := filepath.Join(t.TempDir(), "backups")

	// Seed some data.
	seedSite(t, ctx, store, 365)

	mgr := newTestTenantMgr(t, store)
	w := NewBackupWorker(mgr, t.TempDir(), backupDir, 60, 24, nil)
	if err := w.Run(ctx); err != nil {
		t.Fatalf("backup run: %v", err)
	}

	// Check that shared backup was created.
	sharedDir := filepath.Join(backupDir, "shared")
	entries, err := os.ReadDir(sharedDir)
	if err != nil {
		t.Fatalf("read shared backup dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one snapshot directory under shared/")
	}

	// Check Parquet files exist within the snapshot.
	snapshotDir := filepath.Join(sharedDir, entries[0].Name())
	files, err := findParquetFiles(snapshotDir)
	if err != nil {
		t.Fatalf("find parquet files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected parquet files in backup snapshot")
	}
}

func TestBackupDisabledWhenPathEmpty(t *testing.T) {
	store := newTestStore(t)
	mgr := newTestTenantMgr(t, store)

	w := NewBackupWorker(mgr, t.TempDir(), "", 60, 24, nil)

	// Start should return immediately (no-op).
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Good — Start returned immediately.
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return immediately when backupPath is empty")
	}
}

func TestBackupPrunesOldLocalSnapshots(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	backupDir := filepath.Join(t.TempDir(), "backups")

	mgr := newTestTenantMgr(t, store)

	// Seed a site so the DB has tables to export.
	seedSite(t, ctx, store, 365)

	retentionCount := 2
	w := NewBackupWorker(mgr, t.TempDir(), backupDir, 60, retentionCount, nil)

	// Run 4 backups.
	for i := range 4 {
		if err := w.Run(ctx); err != nil {
			t.Fatalf("backup run %d: %v", i, err)
		}
		// Small delay so timestamps differ.
		time.Sleep(1100 * time.Millisecond)
	}

	// Check that only retentionCount snapshots remain under shared/.
	sharedDir := filepath.Join(backupDir, "shared")
	entries, err := os.ReadDir(sharedDir)
	if err != nil {
		t.Fatalf("read shared dir: %v", err)
	}

	dirCount := 0
	for _, e := range entries {
		if e.IsDir() {
			dirCount++
		}
	}
	if dirCount != retentionCount {
		t.Fatalf("expected %d snapshots after pruning, got %d", retentionCount, dirCount)
	}
}

func TestBackupAndRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	backupDir := filepath.Join(t.TempDir(), "backups")
	siteID := seedSite(t, ctx, store, 365)

	// Insert a hit.
	isUnique := true
	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:    siteID,
		SessionID: uuid.New(),
		PageID:    uuid.New(),
		Timestamp: time.Now().UTC(),
		Path:      "/test-roundtrip",
		IsUnique:  &isUnique,
	}); err != nil {
		t.Fatalf("create hit: %v", err)
	}

	// Backup.
	mgr := newTestTenantMgr(t, store)
	w := NewBackupWorker(mgr, t.TempDir(), backupDir, 60, 24, nil)
	if err := w.Run(ctx); err != nil {
		t.Fatalf("backup run: %v", err)
	}

	// Find the snapshot directory.
	sharedDir := filepath.Join(backupDir, "shared")
	entries, err := os.ReadDir(sharedDir)
	if err != nil {
		t.Fatalf("read shared dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no snapshot created")
	}
	snapshotPath := filepath.Join(sharedDir, entries[0].Name())

	// Restore into a fresh DB.
	restoredDBPath := filepath.Join(t.TempDir(), "restored.db")
	restoredStore := database.NewStore(restoredDBPath)
	if err := restoredStore.Connect(); err != nil {
		t.Fatalf("connect restored db: %v", err)
	}
	defer restoredStore.Close()

	safePath := filepath.ToSlash(snapshotPath)
	importQuery := "IMPORT DATABASE '" + safePath + "';"
	if _, err := restoredStore.DB().ExecContext(ctx, importQuery); err != nil {
		t.Fatalf("import database: %v", err)
	}

	// Verify data survived the round-trip.
	var count int
	if err := restoredStore.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM hits WHERE site_id = ?", siteID,
	).Scan(&count); err != nil {
		t.Fatalf("count restored hits: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 hit in restored DB, got %d", count)
	}

	var path string
	if err := restoredStore.DB().QueryRowContext(ctx,
		"SELECT path FROM hits WHERE site_id = ? LIMIT 1", siteID,
	).Scan(&path); err != nil {
		t.Fatalf("query restored hit path: %v", err)
	}
	if path != "/test-roundtrip" {
		t.Fatalf("expected path=/test-roundtrip, got %q", path)
	}
}

func TestBackupHandlesMultipleTenants(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	backupDir := filepath.Join(t.TempDir(), "backups")
	dataPath := t.TempDir()

	// Seed shared data.
	seedSite(t, ctx, store, 365)

	// Create a non-default tenant.
	customTenantID := uuid.New()
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		customTenantID, "Test Tenant", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert custom tenant: %v", err)
	}

	mgr := database.NewTenantStoreManager(store, dataPath)
	t.Cleanup(func() { _ = mgr.Close() })

	// Open tenant store to create the DB file.
	tenantStore, err := mgr.ForTenant(ctx, customTenantID)
	if err != nil {
		t.Fatalf("open tenant store: %v", err)
	}

	// Seed tenant data.
	if _, err := tenantStore.DB().ExecContext(ctx,
		"INSERT INTO hits (id, site_id, session_id, page_id, timestamp, path, is_unique) VALUES (?, ?, ?, ?, ?, ?, ?)",
		uuid.New(), uuid.New(), uuid.New(), uuid.New(), time.Now().UTC(), "/tenant-page", true,
	); err != nil {
		t.Fatalf("seed tenant hit: %v", err)
	}

	w := NewBackupWorker(mgr, dataPath, backupDir, 60, 24, nil)
	if err := w.Run(ctx); err != nil {
		t.Fatalf("backup run: %v", err)
	}

	// Verify shared backup exists.
	sharedEntries, err := os.ReadDir(filepath.Join(backupDir, "shared"))
	if err != nil {
		t.Fatalf("read shared dir: %v", err)
	}
	if len(sharedEntries) == 0 {
		t.Fatal("expected shared backup snapshot")
	}

	// Verify tenant backup exists.
	tenantDir := filepath.Join(backupDir, "tenants", customTenantID.String())
	tenantEntries, err := os.ReadDir(tenantDir)
	if err != nil {
		t.Fatalf("read tenant backup dir: %v", err)
	}
	if len(tenantEntries) == 0 {
		t.Fatal("expected tenant backup snapshot")
	}

	// Check parquet files in tenant snapshot.
	files, err := findParquetFiles(filepath.Join(tenantDir, tenantEntries[0].Name()))
	if err != nil {
		t.Fatalf("find tenant parquet files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected parquet files in tenant backup snapshot")
	}
}
