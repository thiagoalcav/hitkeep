package takeout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
	"hitkeep/internal/exportfmt"
	"hitkeep/internal/server/shared"
	takeoutsvc "hitkeep/internal/takeout"
)

func TestHandleUserTakeoutSupportsAllFormats(t *testing.T) {
	h, store, userID, _ := setupTakeoutHandlerTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	tests := []struct {
		name            string
		queryFormat     string
		expectedExt     string
		expectedType    string
		expectedStatus  int
		withAuthContext bool
	}{
		{name: "csv", queryFormat: "csv", expectedExt: ".csv", expectedType: "text/csv; charset=utf-8", expectedStatus: http.StatusOK, withAuthContext: true},
		{name: "xlsx", queryFormat: "xlsx", expectedExt: ".xlsx", expectedType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", expectedStatus: http.StatusOK, withAuthContext: true},
		{name: "parquet", queryFormat: "parquet", expectedExt: ".parquet", expectedType: "application/octet-stream", expectedStatus: http.StatusOK, withAuthContext: true},
		{name: "json", queryFormat: "json", expectedExt: ".json", expectedType: "application/json; charset=utf-8", expectedStatus: http.StatusOK, withAuthContext: true},
		{name: "ndjson", queryFormat: "ndjson", expectedExt: ".ndjson", expectedType: "application/x-ndjson; charset=utf-8", expectedStatus: http.StatusOK, withAuthContext: true},
		{name: "unknown defaults to xlsx", queryFormat: "xml", expectedExt: ".xlsx", expectedType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", expectedStatus: http.StatusOK, withAuthContext: true},
		{name: "unauthorized", queryFormat: "csv", expectedStatus: http.StatusUnauthorized, withAuthContext: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/user/takeout?format="+tc.queryFormat, nil)
			if tc.withAuthContext {
				req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
			}

			w := httptest.NewRecorder()
			h.handleUserTakeout().ServeHTTP(w, req)

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
				t.Fatalf("expected non-empty takeout response body")
			}
		})
	}
}

func TestHandleSiteTakeoutSupportsAllFormats(t *testing.T) {
	h, store, _, siteID := setupTakeoutHandlerTestEnv(t)
	t.Cleanup(func() { _ = store.Close() })

	tests := []struct {
		name           string
		siteID         string
		queryFormat    string
		expectedExt    string
		expectedType   string
		expectedStatus int
	}{
		{name: "csv", siteID: siteID.String(), queryFormat: "csv", expectedExt: ".csv", expectedType: "text/csv; charset=utf-8", expectedStatus: http.StatusOK},
		{name: "xlsx", siteID: siteID.String(), queryFormat: "xlsx", expectedExt: ".xlsx", expectedType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", expectedStatus: http.StatusOK},
		{name: "parquet", siteID: siteID.String(), queryFormat: "parquet", expectedExt: ".parquet", expectedType: "application/octet-stream", expectedStatus: http.StatusOK},
		{name: "json", siteID: siteID.String(), queryFormat: "json", expectedExt: ".json", expectedType: "application/json; charset=utf-8", expectedStatus: http.StatusOK},
		{name: "ndjson", siteID: siteID.String(), queryFormat: "ndjson", expectedExt: ".ndjson", expectedType: "application/x-ndjson; charset=utf-8", expectedStatus: http.StatusOK},
		{name: "unknown defaults to xlsx", siteID: siteID.String(), queryFormat: "xml", expectedExt: ".xlsx", expectedType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", expectedStatus: http.StatusOK},
		{name: "invalid site id", siteID: "invalid-uuid", queryFormat: "csv", expectedStatus: http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/sites/"+tc.siteID+"/takeout?format="+tc.queryFormat, nil)
			req.SetPathValue("id", tc.siteID)

			w := httptest.NewRecorder()
			h.handleSiteTakeout().ServeHTTP(w, req)

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
				t.Fatalf("expected non-empty takeout response body")
			}
		})
	}
}

func TestContentTypeForExportUsesSharedMappings(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{filename: "export.csv", want: exportfmt.ContentType(exportfmt.FormatCSV)},
		{filename: "export.xlsx", want: exportfmt.ContentType(exportfmt.FormatXLSX)},
		{filename: "export.parquet", want: exportfmt.ContentType(exportfmt.FormatParquet)},
		{filename: "export.json", want: exportfmt.ContentType(exportfmt.FormatJSON)},
		{filename: "export.ndjson", want: exportfmt.ContentType(exportfmt.FormatNDJSON)},
		{filename: "export.bin", want: "application/octet-stream"},
	}

	for _, tc := range tests {
		if got := exportfmt.ContentTypeForFilename(tc.filename); got != tc.want {
			t.Fatalf("expected content type %q for filename %q, got %q", tc.want, tc.filename, got)
		}
	}
}

func setupTakeoutHandlerTestEnv(t *testing.T) (*TakeoutHandler, *database.Store, uuid.UUID, uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	userID, err := store.CreateUser(ctx, "takeout-handler@example.com", "hash")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	site, err := store.CreateSite(ctx, userID, "takeout-handler.test")
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
		Path:        "/takeout",
		UTMSource:   new("newsletter"),
		UTMMedium:   new("email"),
		UTMCampaign: new("launch"),
		UTMTerm:     new("term"),
		UTMContent:  new("cta"),
		IsUnique:    &isUnique,
	}); err != nil {
		t.Fatalf("failed to seed hit: %v", err)
	}

	service := takeoutsvc.NewTakeoutService(store, filepath.Join(t.TempDir(), "exports"))
	return NewTakeoutHandler(service), store, userID, site.ID
}
