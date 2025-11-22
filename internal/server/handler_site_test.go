package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
)

// setupTestEnv initializes an in-memory database and a Server instance with nil cluster/nsq dependencies
// as they are not required for site management handlers.
func setupTestEnv(t *testing.T) (*Server, *database.Store, uuid.UUID) {
	t.Helper()

	// Use in-memory DuckDB
	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	// Create a dummy user
	userID, err := store.CreateUser(context.Background(), "test@example.com", "hashed_secret")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	s := &Server{
		store: store,
		conf:  &config.Config{},
		// Cluster and Producer are not used in site handlers, so we leave them nil
	}

	return s, store, userID
}

func TestHandleCreateSite(t *testing.T) {
	s, _, userID := setupTestEnv(t)
	defer s.store.Close()

	// Pre-create a site to test conflict
	_, _ = s.store.CreateSite(context.Background(), userID, "taken.com")

	tests := []struct {
		name           string
		body           map[string]string
		injectAuth     bool
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "Unauthorized",
			body:           map[string]string{"domain": "new.com"},
			injectAuth:     false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Empty Body",
			body:           nil,
			injectAuth:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty Domain",
			body:           map[string]string{"domain": ""},
			injectAuth:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Protocol - HTTP",
			body:           map[string]string{"domain": "http://example.com"},
			injectAuth:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Protocol - HTTPS",
			body:           map[string]string{"domain": "https://example.com"},
			injectAuth:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Prefix - WWW",
			body:           map[string]string{"domain": "www.example.com"},
			injectAuth:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid Characters",
			body:           map[string]string{"domain": "inva lid.com"},
			injectAuth:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Duplicate Domain",
			body:           map[string]string{"domain": "taken.com"}, // Already exists
			injectAuth:     true,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "Success",
			body:           map[string]string{"domain": "example.com"},
			injectAuth:     true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var site api.Site
				if err := json.NewDecoder(w.Body).Decode(&site); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if site.Domain != "example.com" {
					t.Errorf("expected domain example.com, got %s", site.Domain)
				}
				if site.ID == uuid.Nil {
					t.Error("expected valid UUID, got nil")
				}
			},
		},
		{
			name:           "Success - Case Insensitive",
			body:           map[string]string{"domain": "UPPERCASE.com"},
			injectAuth:     true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var site api.Site
				if err := json.NewDecoder(w.Body).Decode(&site); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if site.Domain != "uppercase.com" {
					t.Errorf("expected normalized domain uppercase.com, got %s", site.Domain)
				}
			},
		},
		{
			name:           "Success",
			body:           map[string]string{"domain": "sub.example.com"},
			injectAuth:     true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Success",
			body:           map[string]string{"domain": "sub.sub.example.com"},
			injectAuth:     true,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var bodyBytes []byte
			if tc.body != nil {
				bodyBytes, _ = json.Marshal(tc.body)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/sites", bytes.NewReader(bodyBytes))

			if tc.injectAuth {
				ctx := context.WithValue(req.Context(), UserIDKey, userID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler := s.handleCreateSite()
			handler.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, w.Code, w.Body.String())
			}

			if tc.checkResponse != nil {
				tc.checkResponse(t, w)
			}
		})
	}
}

func TestHandleGetSites(t *testing.T) {
	s, _, userID := setupTestEnv(t)
	defer s.store.Close()

	_, _ = s.store.CreateSite(context.Background(), userID, "site1.com")
	_, _ = s.store.CreateSite(context.Background(), userID, "site2.com")

	otherUserID := uuid.New()
	_, _ = s.store.CreateSite(context.Background(), otherUserID, "other.com")

	tests := []struct {
		name           string
		injectAuth     bool
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "Unauthorized",
			injectAuth:     false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Authorized - Returns User Sites",
			injectAuth:     true,
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/sites", nil)
			if tc.injectAuth {
				ctx := context.WithValue(req.Context(), UserIDKey, userID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler := s.handleGetSites()
			handler.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedStatus == http.StatusOK {
				var sites []api.Site
				if err := json.NewDecoder(w.Body).Decode(&sites); err != nil {
					t.Fatalf("failed to decode sites: %v", err)
				}
				if len(sites) != tc.expectedCount {
					t.Errorf("expected %d sites, got %d", tc.expectedCount, len(sites))
				}
			}
		})
	}
}

func TestHandleGetSiteStats(t *testing.T) {
	s, _, userID := setupTestEnv(t)
	defer s.store.Close()

	site, _ := s.store.CreateSite(context.Background(), userID, "stats.com")

	tests := []struct {
		name           string
		siteID         string
		injectAuth     bool
		expectedStatus int
	}{
		{
			name:           "Unauthorized",
			siteID:         site.ID.String(),
			injectAuth:     false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid Site ID",
			siteID:         "not-a-uuid",
			injectAuth:     true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Site Not Found (or not owned)",
			siteID:         uuid.New().String(),
			injectAuth:     true,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Success",
			siteID:         site.ID.String(),
			injectAuth:     true,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/sites/"+tc.siteID+"/stats", nil)
			// Manually inject PathValue since we are bypassing the mux
			req.SetPathValue("id", tc.siteID)

			if tc.injectAuth {
				ctx := context.WithValue(req.Context(), UserIDKey, userID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler := s.handleGetSiteStats()
			handler.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}

func TestHandleGetSiteHits(t *testing.T) {
	s, _, userID := setupTestEnv(t)
	defer s.store.Close()

	site, _ := s.store.CreateSite(context.Background(), userID, "hits.com")

	tests := []struct {
		name           string
		siteID         string
		injectAuth     bool
		queryParams    string
		expectedStatus int
	}{
		{
			name:           "Unauthorized",
			siteID:         site.ID.String(),
			injectAuth:     false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Success - Defaults",
			siteID:         site.ID.String(),
			injectAuth:     true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Success - With Params",
			siteID:         site.ID.String(),
			injectAuth:     true,
			queryParams:    "?limit=5&offset=0",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/sites/"+tc.siteID+"/hits"+tc.queryParams, nil)
			req.SetPathValue("id", tc.siteID)

			if tc.injectAuth {
				ctx := context.WithValue(req.Context(), UserIDKey, userID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler := s.handleGetSiteHits()
			handler.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}
		})
	}
}
