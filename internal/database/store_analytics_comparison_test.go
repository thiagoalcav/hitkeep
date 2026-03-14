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

func TestGetSiteStatsIncludesLandingAndExitPages(t *testing.T) {
	store, userID := setupComparisonStore(t)
	ctx := context.Background()

	site, err := store.CreateSite(ctx, userID, "pages.example.com")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	base := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	sessionA := uuid.New()
	sessionB := uuid.New()
	sessionC := uuid.New()

	for _, hit := range []struct {
		sessionID uuid.UUID
		path      string
		timestamp time.Time
	}{
		{sessionID: sessionA, path: "/home", timestamp: base.Add(-6 * time.Hour)},
		{sessionID: sessionA, path: "/pricing", timestamp: base.Add(-5 * time.Hour)},
		{sessionID: sessionA, path: "/signup", timestamp: base.Add(-4 * time.Hour)},
		{sessionID: sessionB, path: "/blog", timestamp: base.Add(-3 * time.Hour)},
		{sessionID: sessionB, path: "/pricing", timestamp: base.Add(-2 * time.Hour)},
		{sessionID: sessionC, path: "/home", timestamp: base.Add(-90 * time.Minute)},
	} {
		if err := store.CreateHit(ctx, &api.Hit{
			SiteID:    site.ID,
			SessionID: hit.sessionID,
			PageID:    uuid.New(),
			Timestamp: hit.timestamp,
			Path:      hit.path,
		}); err != nil {
			t.Fatalf("create hit %s: %v", hit.path, err)
		}
	}

	result, err := store.GetSiteStats(ctx, api.AnalyticsParams{
		SiteID: site.ID,
		UserID: userID,
		Start:  base.Add(-24 * time.Hour),
		End:    base,
	})
	if err != nil {
		t.Fatalf("GetSiteStats: %v", err)
	}

	if len(result.TopPages) < 2 {
		t.Fatalf("expected top pages, got %v", result.TopPages)
	}
	if !containsMetric(result.TopPages, "/home", 2) {
		t.Fatalf("expected /home with 2 pageviews in top pages, got %+v", result.TopPages)
	}
	if !containsMetric(result.TopPages, "/pricing", 2) {
		t.Fatalf("expected /pricing with 2 pageviews in top pages, got %+v", result.TopPages)
	}
	if result.TopLandingPages[0].Name != "/home" || result.TopLandingPages[0].Value != 2 {
		t.Fatalf("expected /home as top landing page with 2 sessions, got %+v", result.TopLandingPages[0])
	}
	if result.TopLandingPages[1].Name != "/blog" || result.TopLandingPages[1].Value != 1 {
		t.Fatalf("expected /blog as second landing page, got %+v", result.TopLandingPages[1])
	}
	if result.TopExitPages[0].Name != "/home" || result.TopExitPages[0].Value != 1 {
		t.Fatalf("expected /home as first exit page by alphabetical tiebreak, got %+v", result.TopExitPages[0])
	}
	if result.TopExitPages[1].Name != "/pricing" || result.TopExitPages[1].Value != 1 {
		t.Fatalf("expected /pricing as second exit page, got %+v", result.TopExitPages[1])
	}
	if result.TopExitPages[2].Name != "/signup" || result.TopExitPages[2].Value != 1 {
		t.Fatalf("expected /signup as third exit page, got %+v", result.TopExitPages[2])
	}
}

func TestGetSiteStatsLandingAndExitUseFullSessionBoundaries(t *testing.T) {
	store, userID := setupComparisonStore(t)
	ctx := context.Background()

	site, err := store.CreateSite(ctx, userID, "session-boundaries.example.com")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	base := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	sessionID := uuid.New()

	for _, hit := range []struct {
		path      string
		timestamp time.Time
	}{
		{path: "/campaign", timestamp: base.Add(-49 * time.Hour)},
		{path: "/pricing", timestamp: base.Add(-2 * time.Hour)},
		{path: "/checkout", timestamp: base.Add(2 * time.Hour)},
	} {
		if err := store.CreateHit(ctx, &api.Hit{
			SiteID:    site.ID,
			SessionID: sessionID,
			PageID:    uuid.New(),
			Timestamp: hit.timestamp,
			Path:      hit.path,
		}); err != nil {
			t.Fatalf("create hit %s: %v", hit.path, err)
		}
	}

	result, err := store.GetSiteStats(ctx, api.AnalyticsParams{
		SiteID: site.ID,
		UserID: userID,
		Start:  base.Add(-24 * time.Hour),
		End:    base,
	})
	if err != nil {
		t.Fatalf("GetSiteStats: %v", err)
	}

	if len(result.TopPages) != 1 || result.TopPages[0].Name != "/pricing" {
		t.Fatalf("expected only in-range top page /pricing, got %+v", result.TopPages)
	}
	if len(result.TopLandingPages) != 1 || result.TopLandingPages[0].Name != "/campaign" {
		t.Fatalf("expected landing page from full session boundary, got %+v", result.TopLandingPages)
	}
	if len(result.TopExitPages) != 1 || result.TopExitPages[0].Name != "/checkout" {
		t.Fatalf("expected exit page from full session boundary, got %+v", result.TopExitPages)
	}
}

func containsMetric(metrics []api.MetricStat, name string, value int) bool {
	for _, metric := range metrics {
		if metric.Name == name && metric.Value == value {
			return true
		}
	}
	return false
}
