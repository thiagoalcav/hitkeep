package share

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/exportfmt"
	"hitkeep/internal/server/shared"
)

func TestHandleExportShareHitsSupportsAllFormats(t *testing.T) {
	h, store, token, siteID := setupShareExportTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	tests := []struct {
		name           string
		token          string
		siteID         string
		queryFormat    string
		expectedExt    string
		expectedType   string
		expectedStatus int
	}{
		{name: "csv", token: token, siteID: siteID.String(), queryFormat: "csv", expectedExt: ".csv", expectedType: exportfmt.ContentType(exportfmt.FormatCSV), expectedStatus: http.StatusOK},
		{name: "xlsx", token: token, siteID: siteID.String(), queryFormat: "xlsx", expectedExt: ".xlsx", expectedType: exportfmt.ContentType(exportfmt.FormatXLSX), expectedStatus: http.StatusOK},
		{name: "parquet", token: token, siteID: siteID.String(), queryFormat: "parquet", expectedExt: ".parquet", expectedType: exportfmt.ContentType(exportfmt.FormatParquet), expectedStatus: http.StatusOK},
		{name: "json", token: token, siteID: siteID.String(), queryFormat: "json", expectedExt: ".json", expectedType: exportfmt.ContentType(exportfmt.FormatJSON), expectedStatus: http.StatusOK},
		{name: "ndjson", token: token, siteID: siteID.String(), queryFormat: "ndjson", expectedExt: ".ndjson", expectedType: exportfmt.ContentType(exportfmt.FormatNDJSON), expectedStatus: http.StatusOK},
		{name: "unknown defaults to csv", token: token, siteID: siteID.String(), queryFormat: "xml", expectedExt: ".csv", expectedType: exportfmt.ContentType(exportfmt.FormatCSV), expectedStatus: http.StatusOK},
		{name: "invalid token", token: "invalid-token", siteID: siteID.String(), queryFormat: "csv", expectedStatus: http.StatusNotFound},
		{name: "invalid site id", token: token, siteID: "invalid-uuid", queryFormat: "csv", expectedStatus: http.StatusBadRequest},
		{name: "site mismatch", token: token, siteID: uuid.New().String(), queryFormat: "csv", expectedStatus: http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/share/"+tc.token+"/sites/"+tc.siteID+"/hits/export?format="+tc.queryFormat, nil)
			req.SetPathValue("token", tc.token)
			req.SetPathValue("id", tc.siteID)

			w := httptest.NewRecorder()
			h.handleExportShareHits().ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d (body: %s)", tc.expectedStatus, w.Code, w.Body.String())
			}
			if tc.expectedStatus != http.StatusOK {
				return
			}

			if got := w.Header().Get("Content-Type"); got != tc.expectedType {
				t.Fatalf("expected content-type %q, got %q", tc.expectedType, got)
			}

			disposition := w.Header().Get("Content-Disposition")
			if !strings.Contains(disposition, tc.expectedExt) {
				t.Fatalf("expected content-disposition %q to contain extension %q", disposition, tc.expectedExt)
			}

			if w.Body.Len() == 0 {
				t.Fatalf("expected non-empty export response body")
			}
		})
	}
}

// setupShareEventsTestEnv creates a store with a site, share link, hits, and custom events
// that exercise the event names, property keys, breakdown, timeseries, and audience endpoints.
func setupShareEventsTestEnv(t *testing.T) (*handler, *database.Store, string, uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID, err := store.CreateUser(ctx, "share-events@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	site, err := store.CreateSite(ctx, userID, "share-events.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	sessionID := uuid.New()
	now := time.Now().UTC()
	isUnique := true

	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:      site.ID,
		SessionID:   sessionID,
		PageID:      uuid.New(),
		Timestamp:   now,
		Path:        "/pricing",
		CountryCode: new("US"),
		IsUnique:    &isUnique,
	}); err != nil {
		t.Fatalf("create hit: %v", err)
	}

	if err := store.CreateEvent(ctx, &api.Event{
		SiteID:    site.ID,
		SessionID: sessionID,
		Name:      "newsletter_signup",
		Timestamp: now.Add(5 * time.Minute),
		Properties: map[string]any{
			"source": "footer",
			"format": "weekly",
		},
	}); err != nil {
		t.Fatalf("create event: %v", err)
	}

	if err := store.CreateEvent(ctx, &api.Event{
		SiteID:    site.ID,
		SessionID: sessionID,
		Name:      "trial_started",
		Timestamp: now.Add(10 * time.Minute),
		Properties: map[string]any{
			"plan": "pro",
		},
	}); err != nil {
		t.Fatalf("create event: %v", err)
	}

	_, token, err := store.CreateShareLink(ctx, site.ID, userID)
	if err != nil {
		t.Fatalf("create share link: %v", err)
	}

	h := &handler{ctx: &shared.Context{Store: store, Config: &config.Config{}}}
	return h, store, token, site.ID
}

func TestHandleGetShareEventNames(t *testing.T) {
	h, store, token, siteID := setupShareEventsTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	tests := []struct {
		name           string
		token          string
		siteID         string
		expectedStatus int
	}{
		{name: "valid", token: token, siteID: siteID.String(), expectedStatus: http.StatusOK},
		{name: "invalid token", token: "bad", siteID: siteID.String(), expectedStatus: http.StatusNotFound},
		{name: "site mismatch", token: token, siteID: uuid.New().String(), expectedStatus: http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/share/"+tc.token+"/sites/"+tc.siteID+"/events/names", nil)
			req.SetPathValue("token", tc.token)
			req.SetPathValue("id", tc.siteID)

			w := httptest.NewRecorder()
			h.handleGetShareEventNames().ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected %d, got %d: %s", tc.expectedStatus, w.Code, w.Body.String())
			}
			if tc.expectedStatus != http.StatusOK {
				return
			}

			var names []string
			if err := json.NewDecoder(w.Body).Decode(&names); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(names) < 2 {
				t.Fatalf("expected at least 2 event names, got %d: %+v", len(names), names)
			}
		})
	}
}

func TestHandleGetShareEventPropertyKeys(t *testing.T) {
	h, store, token, siteID := setupShareEventsTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/events/properties?event_name=newsletter_signup", nil)
		req.SetPathValue("token", token)
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventPropertyKeys().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var keys []string
		if err := json.NewDecoder(w.Body).Decode(&keys); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(keys) < 1 {
			t.Fatalf("expected at least 1 property key, got %d", len(keys))
		}
	})

	t.Run("missing event_name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/events/properties", nil)
		req.SetPathValue("token", token)
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventPropertyKeys().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing event_name, got %d", w.Code)
		}
	})
}

func TestHandleGetShareEventPropertyBreakdown(t *testing.T) {
	h, store, token, siteID := setupShareEventsTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/events/breakdown?event_name=newsletter_signup&property_key=source", nil)
		req.SetPathValue("token", token)
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventPropertyBreakdown().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		if w.Body.Len() == 0 {
			t.Fatal("expected non-empty response body")
		}
	})

	t.Run("missing params", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/events/breakdown?event_name=newsletter_signup", nil)
		req.SetPathValue("token", token)
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventPropertyBreakdown().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleGetShareEventTimeseries(t *testing.T) {
	h, store, token, siteID := setupShareEventsTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/events/timeseries?event_name=newsletter_signup", nil)
		req.SetPathValue("token", token)
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventTimeseries().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Body.Len() == 0 {
			t.Fatal("expected non-empty response body")
		}
	})

	t.Run("multiple filters", func(t *testing.T) {
		now := time.Now().UTC()
		extraCases := []struct {
			session uuid.UUID
			path    string
			country string
		}{
			{session: uuid.New(), path: "/pricing", country: "DE"},
			{session: uuid.New(), path: "/docs", country: "US"},
		}
		for _, tc := range extraCases {
			country := tc.country
			if err := store.CreateHit(context.Background(), &api.Hit{
				SiteID:      siteID,
				SessionID:   tc.session,
				PageID:      uuid.New(),
				Timestamp:   now,
				Path:        tc.path,
				CountryCode: &country,
			}); err != nil {
				t.Fatalf("create filtered hit: %v", err)
			}
			if err := store.CreateEvent(context.Background(), &api.Event{
				SiteID:     siteID,
				SessionID:  tc.session,
				Name:       "newsletter_signup",
				Timestamp:  now.Add(time.Minute),
				Properties: map[string]any{},
			}); err != nil {
				t.Fatalf("create filtered event: %v", err)
			}
		}

		req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/events/timeseries?event_name=newsletter_signup&filter=path:/pricing&filter=country:US", nil)
		req.SetPathValue("token", token)
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventTimeseries().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var series []api.EventSeriesPoint
		if err := json.NewDecoder(w.Body).Decode(&series); err != nil {
			t.Fatalf("decode timeseries: %v", err)
		}
		total := 0
		for _, point := range series {
			total += point.Count
		}
		if total != 1 {
			t.Fatalf("expected exactly one matching shared event, got %d from %+v", total, series)
		}
	})

	t.Run("invalid filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/events/timeseries?event_name=newsletter_signup&filter=unknown:value", nil)
		req.SetPathValue("token", token)
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventTimeseries().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing event_name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/events/timeseries", nil)
		req.SetPathValue("token", token)
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventTimeseries().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/share/bad/sites/"+siteID.String()+"/events/timeseries?event_name=newsletter_signup", nil)
		req.SetPathValue("token", "bad")
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventTimeseries().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}

func TestHandleGetShareEventAudience(t *testing.T) {
	h, store, token, siteID := setupShareEventsTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	t.Run("multiple filters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/events/audience?event_name=newsletter_signup&filter=path:/pricing&filter=country:US", nil)
		req.SetPathValue("token", token)
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventAudience().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if w.Body.Len() == 0 {
			t.Fatal("expected non-empty response body")
		}

		var audience api.EventAudience
		if err := json.NewDecoder(w.Body).Decode(&audience); err != nil {
			t.Fatalf("decode audience: %v", err)
		}
		if len(audience.TopPages) != 1 || audience.TopPages[0].Name != "/pricing" {
			t.Fatalf("expected filtered audience to contain only /pricing, got %+v", audience.TopPages)
		}
	})

	t.Run("invalid filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/events/audience?event_name=newsletter_signup&filter=unknown:value", nil)
		req.SetPathValue("token", token)
		req.SetPathValue("id", siteID.String())

		w := httptest.NewRecorder()
		h.handleGetShareEventAudience().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// setupShareEcommerceTestEnv creates a store with a site, share link, hits, and
// ecommerce events (begin_checkout + purchase) to exercise the ecommerce share handlers.
func setupShareEcommerceTestEnv(t *testing.T) (*handler, *database.Store, string, uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userID, err := store.CreateUser(ctx, "share-ecom@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	site, err := store.CreateSite(ctx, userID, "share-ecom.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	sessionID := uuid.New()
	now := time.Now().UTC()
	isUnique := true

	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:      site.ID,
		SessionID:   sessionID,
		PageID:      uuid.New(),
		Timestamp:   now,
		Path:        "/checkout",
		CountryCode: new("US"),
		UTMSource:   new("google"),
		UTMMedium:   new("cpc"),
		IsUnique:    &isUnique,
	}); err != nil {
		t.Fatalf("create hit: %v", err)
	}

	if err := store.CreateEvent(ctx, &api.Event{
		SiteID:    site.ID,
		SessionID: sessionID,
		Name:      "begin_checkout",
		Timestamp: now.Add(5 * time.Minute),
		Properties: map[string]any{
			"items": []map[string]any{
				{"item_id": "pro", "item_name": "Pro Plan", "quantity": 1, "price": 79.0},
			},
		},
	}); err != nil {
		t.Fatalf("create checkout event: %v", err)
	}

	if err := store.CreateEvent(ctx, &api.Event{
		SiteID:    site.ID,
		SessionID: sessionID,
		Name:      "purchase",
		Timestamp: now.Add(10 * time.Minute),
		Properties: map[string]any{
			"transaction_id": "ord_share_001",
			"value":          79.0,
			"currency":       "USD",
			"items": []map[string]any{
				{"item_id": "pro", "item_name": "Pro Plan", "quantity": 1, "price": 79.0},
			},
		},
	}); err != nil {
		t.Fatalf("create purchase event: %v", err)
	}

	_, token, err := store.CreateShareLink(ctx, site.ID, userID)
	if err != nil {
		t.Fatalf("create share link: %v", err)
	}

	h := &handler{ctx: &shared.Context{Store: store, Config: &config.Config{}}}
	return h, store, token, site.ID
}

func TestHandleGetShareEcommerceSummary(t *testing.T) {
	h, store, token, siteID := setupShareEcommerceTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	tests := []struct {
		name           string
		token          string
		siteID         string
		expectedStatus int
	}{
		{name: "valid", token: token, siteID: siteID.String(), expectedStatus: http.StatusOK},
		{name: "invalid token", token: "bad", siteID: siteID.String(), expectedStatus: http.StatusNotFound},
		{name: "site mismatch", token: token, siteID: uuid.New().String(), expectedStatus: http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/share/"+tc.token+"/sites/"+tc.siteID+"/ecommerce", nil)
			req.SetPathValue("token", tc.token)
			req.SetPathValue("id", tc.siteID)

			w := httptest.NewRecorder()
			h.handleGetShareEcommerceSummary().ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("expected %d, got %d: %s", tc.expectedStatus, w.Code, w.Body.String())
			}
			if tc.expectedStatus != http.StatusOK {
				return
			}

			var summary api.EcommerceSummary
			if err := json.NewDecoder(w.Body).Decode(&summary); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if summary.Orders != 1 {
				t.Fatalf("expected 1 order, got %d", summary.Orders)
			}
			if summary.CheckoutStarts != 1 {
				t.Fatalf("expected 1 checkout start, got %d", summary.CheckoutStarts)
			}
			if summary.Revenue != 79 {
				t.Fatalf("expected revenue 79, got %f", summary.Revenue)
			}
		})
	}
}

func TestHandleGetShareEcommerceTimeseries(t *testing.T) {
	h, store, token, siteID := setupShareEcommerceTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/ecommerce/timeseries", nil)
	req.SetPathValue("token", token)
	req.SetPathValue("id", siteID.String())

	w := httptest.NewRecorder()
	h.handleGetShareEcommerceTimeseries().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var series []api.EcommerceSeriesPoint
	if err := json.NewDecoder(w.Body).Decode(&series); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(series) == 0 {
		t.Fatal("expected non-empty timeseries")
	}
}

func TestHandleGetShareEcommerceProducts(t *testing.T) {
	h, store, token, siteID := setupShareEcommerceTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/ecommerce/products", nil)
	req.SetPathValue("token", token)
	req.SetPathValue("id", siteID.String())

	w := httptest.NewRecorder()
	h.handleGetShareEcommerceProducts().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var products []api.EcommerceProductStat
	if err := json.NewDecoder(w.Body).Decode(&products); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(products) == 0 {
		t.Fatal("expected at least 1 product")
	}
	if products[0].ItemName != "Pro Plan" {
		t.Fatalf("expected product 'Pro Plan', got %q", products[0].ItemName)
	}
}

func TestHandleGetShareEcommerceSources(t *testing.T) {
	h, store, token, siteID := setupShareEcommerceTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	req := httptest.NewRequest(http.MethodGet, "/api/share/"+token+"/sites/"+siteID.String()+"/ecommerce/sources", nil)
	req.SetPathValue("token", token)
	req.SetPathValue("id", siteID.String())

	w := httptest.NewRecorder()
	h.handleGetShareEcommerceSources().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var sources []api.EcommerceSourceStat
	if err := json.NewDecoder(w.Body).Decode(&sources); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("expected at least 1 source")
	}
}

func setupShareExportTestEnv(t *testing.T) (*handler, *database.Store, string, uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	userID, err := store.CreateUser(ctx, "share-export@example.com", "hash")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	site, err := store.CreateSite(ctx, userID, "share-export.test")
	if err != nil {
		t.Fatalf("failed to create test site: %v", err)
	}

	now := time.Now().UTC()
	isUnique := true
	if err := store.CreateHit(ctx, &api.Hit{
		SiteID:      site.ID,
		SessionID:   uuid.New(),
		PageID:      uuid.New(),
		Timestamp:   now,
		Path:        "/share-export",
		UTMSource:   new("newsletter"),
		UTMMedium:   new("email"),
		UTMCampaign: new("launch"),
		UTMTerm:     new("share"),
		UTMContent:  new("cta"),
		IsUnique:    &isUnique,
	}); err != nil {
		t.Fatalf("failed to seed hit: %v", err)
	}

	_, token, err := store.CreateShareLink(ctx, site.ID, userID)
	if err != nil {
		t.Fatalf("failed to create share link: %v", err)
	}

	h := &handler{
		ctx: &shared.Context{
			Store:  store,
			Config: &config.Config{},
		},
	}
	return h, store, token, site.ID
}
