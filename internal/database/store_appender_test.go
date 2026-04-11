package database

import (
	"context"
	"database/sql"
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
	hostname := "appender.example.com"
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
			Hostname:      &hostname,
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
		if got.Hostname == nil || *got.Hostname != hostname {
			t.Fatalf("expected hostname %q, got %+v", hostname, got.Hostname)
		}
		if got.CountryCode == nil || *got.CountryCode != country {
			t.Fatalf("expected country %q, got %+v", country, got.CountryCode)
		}
	}
}

func TestCreateHitsBulkWithLegacyHostnameColumnOrder(t *testing.T) {
	ctx := context.Background()

	store := NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.DB().ExecContext(ctx, `
		CREATE TABLE hits (
			id              UUID        PRIMARY KEY,
			site_id         UUID        NOT NULL,
			session_id      UUID        NOT NULL,
			page_id         UUID        NOT NULL,
			timestamp       TIMESTAMPTZ NOT NULL,
			path            VARCHAR     NOT NULL,
			referrer        VARCHAR,
			user_agent      VARCHAR,
			viewport_width  INT,
			viewport_height INT,
			screen_width    INT,
			screen_height   INT,
			language        VARCHAR,
			is_unique       BOOLEAN,
			country_code    VARCHAR,
			utm_source      VARCHAR,
			utm_medium      VARCHAR,
			utm_campaign    VARCHAR,
			utm_term        VARCHAR,
			utm_content     VARCHAR,
			hostname        VARCHAR
		)
	`); err != nil {
		t.Fatalf("create legacy hits table: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, `
		CREATE TABLE rollup_dirty_buckets (
			site_id     UUID        NOT NULL,
			rollup_type VARCHAR     NOT NULL,
			bucket_unit VARCHAR     NOT NULL,
			bucket      TIMESTAMPTZ NOT NULL,
			updated_at  TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (site_id, rollup_type, bucket_unit, bucket)
		)
	`); err != nil {
		t.Fatalf("create rollup_dirty_buckets table: %v", err)
	}

	siteID := uuid.New()
	sessionID := uuid.New()
	pageID := uuid.New()
	hostname := "legacy.example.com"
	referrer := "https://ref.example.com/post"
	userAgent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"
	language := "de-DE"
	country := "DE"
	viewportWidth := 1440
	viewportHeight := 900
	screenWidth := 1728
	screenHeight := 1117
	isUnique := true

	hit := &api.Hit{
		SiteID:         siteID,
		SessionID:      sessionID,
		PageID:         pageID,
		Timestamp:      time.Now().UTC(),
		Path:           "/legacy-order",
		Hostname:       &hostname,
		Referrer:       &referrer,
		UserAgent:      &userAgent,
		ViewportWidth:  &viewportWidth,
		ViewportHeight: &viewportHeight,
		ScreenWidth:    &screenWidth,
		ScreenHeight:   &screenHeight,
		Language:       &language,
		CountryCode:    &country,
		IsUnique:       &isUnique,
	}

	if err := store.CreateHit(ctx, hit); err != nil {
		t.Fatalf("CreateHit with legacy schema: %v", err)
	}

	var (
		gotHostname       sql.NullString
		gotReferrer       sql.NullString
		gotUserAgent      sql.NullString
		gotViewportWidth  sql.NullInt32
		gotViewportHeight sql.NullInt32
		gotScreenWidth    sql.NullInt32
		gotScreenHeight   sql.NullInt32
	)
	if err := store.DB().QueryRowContext(ctx, `
		SELECT hostname, referrer, user_agent, viewport_width, viewport_height, screen_width, screen_height
		FROM hits
		WHERE site_id = ?
	`, siteID).Scan(
		&gotHostname,
		&gotReferrer,
		&gotUserAgent,
		&gotViewportWidth,
		&gotViewportHeight,
		&gotScreenWidth,
		&gotScreenHeight,
	); err != nil {
		t.Fatalf("load stored hit: %v", err)
	}

	if gotHostname.String != hostname {
		t.Fatalf("expected hostname %q, got %+v", hostname, gotHostname)
	}
	if gotReferrer.String != referrer {
		t.Fatalf("expected referrer %q, got %+v", referrer, gotReferrer)
	}
	if gotUserAgent.String != userAgent {
		t.Fatalf("expected user agent %q, got %+v", userAgent, gotUserAgent)
	}
	if gotViewportWidth.Int32 != int32(viewportWidth) {
		t.Fatalf("expected viewport width %d, got %+v", viewportWidth, gotViewportWidth)
	}
	if gotViewportHeight.Int32 != int32(viewportHeight) {
		t.Fatalf("expected viewport height %d, got %+v", viewportHeight, gotViewportHeight)
	}
	if gotScreenWidth.Int32 != int32(screenWidth) {
		t.Fatalf("expected screen width %d, got %+v", screenWidth, gotScreenWidth)
	}
	if gotScreenHeight.Int32 != int32(screenHeight) {
		t.Fatalf("expected screen height %d, got %+v", screenHeight, gotScreenHeight)
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

func TestCreateAIFetchesBulk(t *testing.T) {
	store, _, site := setupAppenderStore(t)
	ctx := context.Background()

	hostname := "appender.example.com"
	contentType := "text/html; charset=utf-8"
	responseMs := 123
	bytesServed := int64(4567)
	userAgent := "GPTBot/1.0"

	fetches := []*api.AIFetch{
		{
			SiteID:          site.ID,
			Timestamp:       time.Now().Add(-2 * time.Minute),
			AssistantName:   "GPTBot",
			AssistantFamily: "OpenAI",
			Path:            "/pricing",
			Hostname:        &hostname,
			StatusCode:      200,
			ContentType:     &contentType,
			ResourceType:    "html",
			ResponseMs:      &responseMs,
			BytesServed:     &bytesServed,
			UserAgent:       &userAgent,
		},
		{
			SiteID:          site.ID,
			AssistantName:   "PerplexityBot",
			AssistantFamily: "Perplexity",
			Path:            "/docs/getting-started",
			StatusCode:      404,
			ResourceType:    "html",
		},
	}

	if err := store.CreateAIFetchesBulk(ctx, fetches); err != nil {
		t.Fatalf("CreateAIFetchesBulk: %v", err)
	}
	if fetches[1].Timestamp.IsZero() {
		t.Fatalf("expected bulk insert to assign zero timestamp")
	}
	if fetches[1].ID == uuid.Nil {
		t.Fatalf("expected bulk insert to assign zero id")
	}

	var count int
	if err := store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM ai_fetches WHERE site_id = ?", site.ID).Scan(&count); err != nil {
		t.Fatalf("count ai fetches: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 ai fetches, got %d", count)
	}

	var storedPath string
	if err := store.DB().QueryRowContext(ctx, "SELECT path FROM ai_fetches WHERE site_id = ? AND assistant_name = ?", site.ID, "GPTBot").Scan(&storedPath); err != nil {
		t.Fatalf("load ai fetch path: %v", err)
	}
	if storedPath != "/pricing" {
		t.Fatalf("expected stored path /pricing, got %q", storedPath)
	}
}
