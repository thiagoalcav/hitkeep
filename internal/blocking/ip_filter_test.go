package blocking

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"hitkeep/internal/database"
)

func setupFilterStore(t *testing.T) (*database.Store, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()

	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	userID, err := store.CreateUser(context.Background(), "owner@example.com", "hashed-secret")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	site, err := store.CreateSite(context.Background(), userID, "filter.example")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	otherSite, err := store.CreateSite(context.Background(), userID, "filter-2.example")
	if err != nil {
		t.Fatalf("create second site: %v", err)
	}

	return store, userID, site.ID, otherSite.ID
}

func TestIPFilterIsBlocked(t *testing.T) {
	store, userID, siteID, otherSiteID := setupFilterStore(t)
	defer store.Close()

	ctx := context.Background()

	if _, err := store.CreateInstanceExclusion(ctx, "203.0.113.5/32", "global monitor", userID); err != nil {
		t.Fatalf("create instance exclusion: %v", err)
	}
	if _, err := store.CreateSiteExclusion(ctx, siteID, "10.0.0.0/8", "office", userID); err != nil {
		t.Fatalf("create site exclusion: %v", err)
	}

	filter := NewIPFilter(store)
	if err := filter.Refresh(ctx); err != nil {
		t.Fatalf("refresh filter: %v", err)
	}

	if !filter.IsBlocked(siteID, "203.0.113.5") {
		t.Fatalf("expected global blocked ip to be blocked")
	}
	if !filter.IsBlocked(siteID, "10.1.2.3") {
		t.Fatalf("expected site blocked ip to be blocked")
	}
	if filter.IsBlocked(otherSiteID, "10.1.2.3") {
		t.Fatalf("expected site-specific blocked ip to be allowed for other site")
	}
	if filter.IsBlocked(siteID, "198.51.100.1") {
		t.Fatalf("expected non-blocked ip to be allowed")
	}
	if filter.IsBlocked(siteID, "") {
		t.Fatalf("expected empty ip to be allowed")
	}
}
