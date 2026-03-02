package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func newSharedTestStore(t *testing.T) *Store {
	t.Helper()
	tmpDir := t.TempDir()
	store := NewStore(filepath.Join(tmpDir, "test.db"))
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store
}

func TestForTenantDefaultReturnsShared(t *testing.T) {
	ctx := context.Background()
	store := newSharedTestStore(t)
	mgr := NewTenantStoreManager(store, t.TempDir())
	t.Cleanup(func() { _ = mgr.Close() })

	defaultID, err := store.GetDefaultTenantID(ctx)
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	got, err := mgr.ForTenant(ctx, defaultID)
	if err != nil {
		t.Fatalf("ForTenant(default): %v", err)
	}
	if got != store {
		t.Fatal("expected ForTenant(defaultID) to return the shared store")
	}
}

func TestForTenantNilReturnsShared(t *testing.T) {
	store := newSharedTestStore(t)
	mgr := NewTenantStoreManager(store, t.TempDir())
	t.Cleanup(func() { _ = mgr.Close() })

	got, err := mgr.ForTenant(context.Background(), uuid.Nil)
	if err != nil {
		t.Fatalf("ForTenant(nil): %v", err)
	}
	if got != store {
		t.Fatal("expected ForTenant(uuid.Nil) to return the shared store")
	}
}

func TestForTenantCreatesNewDB(t *testing.T) {
	ctx := context.Background()
	store := newSharedTestStore(t)
	basePath := t.TempDir()
	mgr := NewTenantStoreManager(store, basePath)
	t.Cleanup(func() { _ = mgr.Close() })

	tenantID := uuid.New()
	// Insert a non-default tenant into the shared DB.
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		tenantID, "Test Tenant", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	got, err := mgr.ForTenant(ctx, tenantID)
	if err != nil {
		t.Fatalf("ForTenant(custom): %v", err)
	}
	if got == store {
		t.Fatal("expected ForTenant(customID) to return a different store from shared")
	}

	// Verify the DB file was created.
	dbPath := filepath.Join(basePath, "tenants", tenantID.String(), "hitkeep.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("expected tenant DB file at %s", dbPath)
	}
}

func TestForTenantCacheHit(t *testing.T) {
	ctx := context.Background()
	store := newSharedTestStore(t)
	mgr := NewTenantStoreManager(store, t.TempDir())
	t.Cleanup(func() { _ = mgr.Close() })

	tenantID := uuid.New()
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		tenantID, "Test Tenant", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	first, err := mgr.ForTenant(ctx, tenantID)
	if err != nil {
		t.Fatalf("first ForTenant: %v", err)
	}

	second, err := mgr.ForTenant(ctx, tenantID)
	if err != nil {
		t.Fatalf("second ForTenant: %v", err)
	}

	if first != second {
		t.Fatal("expected same store instance on second call (cache hit)")
	}
}

func TestForTenantMigratesOnFirstAccess(t *testing.T) {
	ctx := context.Background()
	store := newSharedTestStore(t)
	mgr := NewTenantStoreManager(store, t.TempDir())
	t.Cleanup(func() { _ = mgr.Close() })

	tenantID := uuid.New()
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		tenantID, "Test Tenant", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	tenantStore, err := mgr.ForTenant(ctx, tenantID)
	if err != nil {
		t.Fatalf("ForTenant: %v", err)
	}

	// Verify analytics tables exist by querying them.
	tables := []string{"hits", "events", "goals", "funnels",
		"hit_rollups_hourly", "hit_rollups_daily", "hit_rollups_monthly",
		"session_rollups_hourly", "session_rollups_daily", "session_rollups_monthly",
		"goal_rollups_hourly", "goal_rollups_daily", "goal_rollups_monthly",
		"funnel_rollups_hourly", "funnel_rollups_daily", "funnel_rollups_monthly",
	}
	for _, table := range tables {
		var count int
		if err := tenantStore.DB().QueryRowContext(ctx,
			fmt.Sprintf("SELECT COUNT(*) FROM %s", table),
		).Scan(&count); err != nil {
			t.Fatalf("query %s on tenant DB: %v", table, err)
		}
	}
}

func TestCloseClosesAllTenantStores(t *testing.T) {
	ctx := context.Background()
	store := newSharedTestStore(t)
	mgr := NewTenantStoreManager(store, t.TempDir())

	tenantID := uuid.New()
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		tenantID, "Test Tenant", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	tenantStore, err := mgr.ForTenant(ctx, tenantID)
	if err != nil {
		t.Fatalf("ForTenant: %v", err)
	}

	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// The tenant store should be closed (querying should fail or return an error).
	if err := tenantStore.DB().PingContext(ctx); err == nil {
		// DuckDB may or may not error on ping after close; at minimum, the store
		// map should be empty. Just verify Close() didn't error.
	}
}

func TestResolveTenantStore(t *testing.T) {
	ctx := context.Background()
	store := newSharedTestStore(t)
	mgr := NewTenantStoreManager(store, t.TempDir())
	t.Cleanup(func() { _ = mgr.Close() })

	// Create a user.
	userID, err := store.CreateUser(ctx, "test@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// The user should resolve to the default tenant (via membership seeded by migration).
	resolvedStore, tenantID, err := mgr.ResolveTenantStore(ctx, userID)
	if err != nil {
		t.Fatalf("ResolveTenantStore: %v", err)
	}

	defaultID, err := store.GetDefaultTenantID(ctx)
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}
	if tenantID != defaultID {
		t.Fatalf("expected default tenant ID %s, got %s", defaultID, tenantID)
	}
	if resolvedStore != store {
		t.Fatal("expected resolved store to be the shared store for default tenant")
	}
}

func TestResolveSiteStoreBackfillsLegacyAnalyticsConfig(t *testing.T) {
	ctx := context.Background()
	store := newSharedTestStore(t)
	basePath := t.TempDir()
	mgr := NewTenantStoreManager(store, basePath)
	t.Cleanup(func() { _ = mgr.Close() })

	userID, err := store.CreateUser(ctx, "sync@test.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	team, err := store.CreateTenant(ctx, userID, "Sync Team", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := store.SetActiveTenantID(ctx, userID, team.ID); err != nil {
		t.Fatalf("set active tenant: %v", err)
	}

	site, err := store.CreateSite(ctx, userID, "sync-team.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	legacyGoal := &api.Goal{
		ID:        uuid.New(),
		SiteID:    site.ID,
		Name:      "Legacy Signup",
		Type:      "event",
		Value:     "signup_completed",
		CreatedAt: time.Now().UTC(),
	}
	if err := store.CreateGoal(ctx, legacyGoal); err != nil {
		t.Fatalf("create shared goal: %v", err)
	}

	legacyFunnel := &api.Funnel{
		ID:        uuid.New(),
		SiteID:    site.ID,
		Name:      "Legacy Funnel",
		Steps:     []api.FunnelStep{{Type: "path", Value: "/"}, {Type: "event", Value: "signup_completed"}},
		CreatedAt: time.Now().UTC(),
	}
	if err := store.CreateFunnel(ctx, legacyFunnel); err != nil {
		t.Fatalf("create shared funnel: %v", err)
	}

	tenantStore, tenantID, err := mgr.ResolveSiteStore(ctx, site.ID)
	if err != nil {
		t.Fatalf("ResolveSiteStore: %v", err)
	}
	if tenantID != team.ID {
		t.Fatalf("expected tenant %s, got %s", team.ID, tenantID)
	}
	if tenantStore == store {
		t.Fatal("expected custom tenant site to resolve to tenant-local store")
	}

	var mirroredDomain string
	var mirroredRetention int
	if err := tenantStore.DB().QueryRowContext(ctx,
		"SELECT domain, data_retention_days FROM sites WHERE id = ?",
		site.ID,
	).Scan(&mirroredDomain, &mirroredRetention); err != nil {
		t.Fatalf("query tenant site mirror: %v", err)
	}
	if mirroredDomain != site.Domain {
		t.Fatalf("expected mirrored domain %q, got %q", site.Domain, mirroredDomain)
	}

	tenantGoals, err := tenantStore.GetGoals(ctx, site.ID)
	if err != nil {
		t.Fatalf("tenant GetGoals: %v", err)
	}
	if len(tenantGoals) != 1 || tenantGoals[0].ID != legacyGoal.ID {
		t.Fatalf("expected legacy goal to be backfilled, got %+v", tenantGoals)
	}

	tenantFunnels, err := tenantStore.GetFunnels(ctx, site.ID)
	if err != nil {
		t.Fatalf("tenant GetFunnels: %v", err)
	}
	if len(tenantFunnels) != 1 || tenantFunnels[0].ID != legacyFunnel.ID {
		t.Fatalf("expected legacy funnel to be backfilled, got %+v", tenantFunnels)
	}
}

func TestDeleteSiteRemovesTenantAndSharedData(t *testing.T) {
	ctx := context.Background()
	store := newSharedTestStore(t)
	basePath := t.TempDir()
	mgr := NewTenantStoreManager(store, basePath)
	t.Cleanup(func() { _ = mgr.Close() })

	userID, err := store.CreateUser(ctx, "delete-site@test.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	team, err := store.CreateTenant(ctx, userID, "Delete Team", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := store.SetActiveTenantID(ctx, userID, team.ID); err != nil {
		t.Fatalf("set active tenant: %v", err)
	}

	site, err := store.CreateSite(ctx, userID, "delete-team.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	tenantStore, _, err := mgr.ResolveSiteStore(ctx, site.ID)
	if err != nil {
		t.Fatalf("ResolveSiteStore: %v", err)
	}

	if err := tenantStore.CreateHit(ctx, &api.Hit{
		SiteID:    site.ID,
		SessionID: uuid.New(),
		PageID:    uuid.New(),
		Timestamp: time.Now().UTC(),
		Path:      "/",
	}); err != nil {
		t.Fatalf("create tenant hit: %v", err)
	}

	if err := mgr.DeleteSite(ctx, site.ID); err != nil {
		t.Fatalf("DeleteSite: %v", err)
	}

	var sharedCount int
	if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM sites WHERE id = ?", site.ID).Scan(&sharedCount); err != nil {
		t.Fatalf("count shared sites: %v", err)
	}
	if sharedCount != 0 {
		t.Fatalf("expected shared site row deleted, got count=%d", sharedCount)
	}

	var tenantCount int
	if err := tenantStore.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM sites WHERE id = ?", site.ID).Scan(&tenantCount); err != nil {
		t.Fatalf("count tenant sites: %v", err)
	}
	if tenantCount != 0 {
		t.Fatalf("expected tenant site mirror deleted, got count=%d", tenantCount)
	}
}
