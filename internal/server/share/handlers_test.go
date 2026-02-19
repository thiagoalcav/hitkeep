package share

import (
	"context"
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
