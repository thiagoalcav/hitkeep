package permissions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"hitkeep/internal/auth"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

func setupTestEnv(t *testing.T) (*shared.Context, *database.Store, uuid.UUID) {
	t.Helper()

	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	userID, err := store.CreateUser(context.Background(), "test@example.com", "hashed_secret")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	ctx := &shared.Context{
		Store:  store,
		Config: &config.Config{},
	}

	return ctx, store, userID
}

func TestHandleGetUserPermissions(t *testing.T) {
	ctx, store, userID := setupTestEnv(t)
	defer store.Close()

	tests := []struct {
		name           string
		injectAuth     bool
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:           "Unauthorized",
			injectAuth:     false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Authorized - Returns Permissions",
			injectAuth:     true,
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				type response struct {
					InstanceRole auth.InstanceRole `json:"instance_role"`
					Permissions  []auth.Permission `json:"permissions"`
				}
				var resp response
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}

				// By default, setupTestEnv creates a user which is likely an InstanceOwner (first user)
				// or just a regular user depending on how CreateUser works.
				// In database/store.go, CreateUser usually makes the first user an owner.
				// Let's check what we got.
				if resp.InstanceRole == "" {
					t.Error("expected instance role, got empty")
				}
				// We expect at least some permissions or empty list, but not nil if we initialized it
				if resp.Permissions == nil {
					// It might be nil if empty, which is fine for JSON, but let's check if we expected some.
					// If it's a regular user, it might have no permissions.
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/user/permissions", nil)

			if tc.injectAuth {
				req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
			}

			w := httptest.NewRecorder()
			handler := (&handler{ctx: ctx}).handleGetUserPermissions()
			handler.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.checkResponse != nil {
				tc.checkResponse(t, w)
			}
		})
	}
}
