package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func setupAppenderStore(t *testing.T) (*Store, uuid.UUID, *api.Site) {
	t.Helper()

	store := NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	userID, err := store.CreateUser(context.Background(), "appender@example.com", "hashed")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	site, err := store.CreateSite(context.Background(), userID, "appender.example.com")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	return store, userID, site
}

func TestCreateHitsBulk(t *testing.T) {
	store, _, site := setupAppenderStore(t)
	ctx := context.Background()

	referrer := "https://example.com"
	language := "de-DE"
	country := "DE"
	viewport := 1440
	unique := true

	hits := []*api.Hit{
		{
			SiteID:        site.ID,
			SessionID:     uuid.New(),
			PageID:        uuid.New(),
			Timestamp:     time.Now().Add(-2 * time.Minute),
			Path:          "/pricing",
			Referrer:      &referrer,
			ViewportWidth: &viewport,
			Language:      &language,
			CountryCode:   &country,
			IsUnique:      &unique,
		},
		{
			SiteID:    site.ID,
			SessionID: uuid.New(),
			PageID:    uuid.New(),
			Path:      "/signup",
		},
	}

	if err := store.CreateHitsBulk(ctx, hits); err != nil {
		t.Fatalf("CreateHitsBulk: %v", err)
	}
	if hits[1].Timestamp.IsZero() {
		t.Fatalf("expected bulk insert to assign zero timestamp")
	}

	result, err := store.GetHits(ctx, api.HitQueryParams{
		SiteID: site.ID,
		Start:  time.Now().Add(-1 * time.Hour),
		End:    time.Now().Add(1 * time.Hour),
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("GetHits: %v", err)
	}

	if result.Total != 2 {
		t.Fatalf("expected 2 hits, got %d", result.Total)
	}

	paths := map[string]api.Hit{}
	for _, hit := range result.Data {
		paths[hit.Path] = hit
	}

	if got, ok := paths["/pricing"]; !ok {
		t.Fatalf("expected /pricing hit in %+v", result.Data)
	} else {
		if got.Language == nil || *got.Language != language {
			t.Fatalf("expected language %q, got %+v", language, got.Language)
		}
		if got.CountryCode == nil || *got.CountryCode != country {
			t.Fatalf("expected country %q, got %+v", country, got.CountryCode)
		}
	}
}

func TestCreateEventsBulk(t *testing.T) {
	store, _, site := setupAppenderStore(t)
	ctx := context.Background()

	events := []*api.Event{
		{
			SiteID:     site.ID,
			SessionID:  uuid.New(),
			Name:       "signup",
			Properties: map[string]any{"plan": "pro"},
			Timestamp:  time.Now().Add(-1 * time.Minute),
		},
		{
			SiteID:    site.ID,
			SessionID: uuid.New(),
			Name:      "checkout_started",
		},
	}

	if err := store.CreateEventsBulk(ctx, events); err != nil {
		t.Fatalf("CreateEventsBulk: %v", err)
	}
	if events[1].Timestamp.IsZero() {
		t.Fatalf("expected bulk insert to assign zero timestamp")
	}

	var count int
	if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE site_id = ?", site.ID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 events, got %d", count)
	}

	var props string
	if err := store.DB().QueryRowContext(ctx, "SELECT CAST(properties AS VARCHAR) FROM events WHERE site_id = ? AND name = ?", site.ID, "signup").Scan(&props); err != nil {
		t.Fatalf("load event properties: %v", err)
	}
	if !strings.Contains(props, "pro") {
		t.Fatalf("expected marshaled properties to contain plan value, got %q", props)
	}
}
