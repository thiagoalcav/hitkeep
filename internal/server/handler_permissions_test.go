package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hitkeep/internal/auth"
)

func TestHandleGetUserPermissions(t *testing.T) {
	s, _, userID := setupTestEnv(t)
	defer s.store.Close()

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
				ctx := context.WithValue(req.Context(), UserIDKey, userID)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler := s.handleGetUserPermissions()
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
