package database

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func setupComparisonStore(t *testing.T) (*Store, uuid.UUID) {
	t.Helper()
	store := NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	userID, err := store.CreateUser(context.Background(), "cmp@example.com", "hashed")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return store, userID
}

func TestGetSiteStatsComparison(t *testing.T) {
	store, userID := setupComparisonStore(t)
	ctx := context.Background()

	site, err := store.CreateSite(ctx, userID, "comparison.example.com")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	now := time.Now().UTC()
	currentStart := now.AddDate(0, 0, -7)
	currentEnd := now
	previousStart := now.AddDate(0, 0, -14)
	previousEnd := now.AddDate(0, 0, -7)

	sessionCurrent := uuid.New()
	sessionPrevious := uuid.New()

	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:    site.ID,
		SessionID: sessionCurrent,
		PageID:    uuid.New(),
		Timestamp: now.AddDate(0, 0, -3),
		Path:      "/current",
	}); err != nil {
		t.Fatalf("create current hit: %v", err)
	}

	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:    site.ID,
		SessionID: sessionPrevious,
		PageID:    uuid.New(),
		Timestamp: now.AddDate(0, 0, -10),
		Path:      "/previous",
	}); err != nil {
		t.Fatalf("create previous hit: %v", err)
	}

	params := api.AnalyticsParams{
		SiteID:       site.ID,
		UserID:       userID,
		Start:        currentStart,
		End:          currentEnd,
		CompareStart: previousStart,
		CompareEnd:   previousEnd,
	}

	result, err := store.GetSiteStats(ctx, params)
	if err != nil {
		t.Fatalf("GetSiteStats: %v", err)
	}

	if result.Comparison == nil {
		t.Fatal("expected comparison stats, got nil")
	}
	if result.TotalPageviews != 1 {
		t.Errorf("expected 1 current pageview, got %d", result.TotalPageviews)
	}
	if result.Comparison.TotalPageviews != 1 {
		t.Errorf("expected 1 comparison pageview, got %d", result.Comparison.TotalPageviews)
	}
}

func TestGetSiteStatsNoComparison(t *testing.T) {
	store, userID := setupComparisonStore(t)
	ctx := context.Background()

	site, err := store.CreateSite(ctx, userID, "nocompare.example.com")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	now := time.Now().UTC()
	params := api.AnalyticsParams{
		SiteID: site.ID,
		UserID: userID,
		Start:  now.AddDate(0, 0, -7),
		End:    now,
	}

	result, err := store.GetSiteStats(ctx, params)
	if err != nil {
		t.Fatalf("GetSiteStats: %v", err)
	}
	if result.Comparison != nil {
		t.Error("expected nil comparison when CompareStart is zero")
	}
}

func TestGetSiteStatsComparisonCurrentPeriodEmpty(t *testing.T) {
	store, userID := setupComparisonStore(t)
	ctx := context.Background()

	site, err := store.CreateSite(ctx, userID, "cmp-empty.example.com")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	now := time.Now().UTC()
	currentStart := now.AddDate(0, 0, -7)
	currentEnd := now
	previousStart := now.AddDate(0, 0, -14)
	previousEnd := now.AddDate(0, 0, -7)

	// Only a hit in the previous window; current period should show zero pageviews.
	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:    site.ID,
		SessionID: uuid.New(),
		PageID:    uuid.New(),
		Timestamp: now.AddDate(0, 0, -10),
		Path:      "/old-page",
	}); err != nil {
		t.Fatalf("create previous hit: %v", err)
	}

	params := api.AnalyticsParams{
		SiteID:       site.ID,
		UserID:       userID,
		Start:        currentStart,
		End:          currentEnd,
		CompareStart: previousStart,
		CompareEnd:   previousEnd,
	}

	result, err := store.GetSiteStats(ctx, params)
	if err != nil {
		t.Fatalf("GetSiteStats: %v", err)
	}

	if result.Comparison == nil {
		t.Fatal("expected comparison stats even when current period is empty")
	}
	if result.TotalPageviews != 0 {
		t.Errorf("expected 0 current pageviews, got %d", result.TotalPageviews)
	}
	if result.Comparison.TotalPageviews != 1 {
		t.Errorf("expected 1 comparison pageview, got %d", result.Comparison.TotalPageviews)
	}
}

func TestGetSiteStatsComparisonBothPeriodsPopulated(t *testing.T) {
	store, userID := setupComparisonStore(t)
	ctx := context.Background()

	site, err := store.CreateSite(ctx, userID, "cmp-both.example.com")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	now := time.Now().UTC()
	currentStart := now.AddDate(0, 0, -7)
	currentEnd := now
	previousStart := now.AddDate(0, 0, -14)
	previousEnd := now.AddDate(0, 0, -7)

	sessionA := uuid.New()
	sessionB := uuid.New()

	// Two hits in the current period across the same session.
	for _, path := range []string{"/home", "/about"} {
		if err := store.CreateHit(ctx, &api.Hit{
			SiteID:    site.ID,
			SessionID: sessionA,
			PageID:    uuid.New(),
			Timestamp: now.AddDate(0, 0, -2),
			Path:      path,
		}); err != nil {
			t.Fatalf("create current hit %s: %v", path, err)
		}
	}

	// One hit in the previous period on a different session.
	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:    site.ID,
		SessionID: sessionB,
		PageID:    uuid.New(),
		Timestamp: now.AddDate(0, 0, -9),
		Path:      "/landing",
	}); err != nil {
		t.Fatalf("create previous hit: %v", err)
	}

	params := api.AnalyticsParams{
		SiteID:       site.ID,
		UserID:       userID,
		Start:        currentStart,
		End:          currentEnd,
		CompareStart: previousStart,
		CompareEnd:   previousEnd,
	}

	result, err := store.GetSiteStats(ctx, params)
	if err != nil {
		t.Fatalf("GetSiteStats: %v", err)
	}

	if result.Comparison == nil {
		t.Fatal("expected comparison stats, got nil")
	}
	if result.TotalPageviews != 2 {
		t.Errorf("expected 2 current pageviews, got %d", result.TotalPageviews)
	}
	if result.Comparison.TotalPageviews != 1 {
		t.Errorf("expected 1 comparison pageview, got %d", result.Comparison.TotalPageviews)
	}
}
