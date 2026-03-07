package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

func setupAdminTestEnv(t *testing.T) (*handler, *database.Store, uuid.UUID, uuid.UUID) {
	t.Helper()

	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	targetUserID, err := store.CreateUser(context.Background(), "target-owner@example.com", "hash")
	if err != nil {
		t.Fatalf("create target user: %v", err)
	}
	actorUserID, err := store.CreateUser(context.Background(), "admin@example.com", "hash")
	if err != nil {
		t.Fatalf("create actor user: %v", err)
	}

	ctx := &shared.Context{
		Store: store,
		Config: &config.Config{
			PublicURL: "http://localhost:8080",
			JWTSecret: "test-secret",
		},
	}

	return &handler{ctx: ctx}, store, actorUserID, targetUserID
}

func withAdminTestUser(req *http.Request, userID uuid.UUID) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
}

func TestHandleDeleteUserReturnsConflictForSoleOwner(t *testing.T) {
	h, store, actorUserID, targetUserID := setupAdminTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+targetUserID.String(), nil), actorUserID)
	req.SetPathValue("id", targetUserID.String())
	w := httptest.NewRecorder()

	h.handleDeleteUser().ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
	}

	var resp api.AdminDeleteUserBlockedResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != "user_owns_teams" {
		t.Fatalf("expected code %q, got %q", "user_owns_teams", resp.Code)
	}
	if len(resp.Teams) != 1 || resp.Teams[0].Name != "Default Tenant" {
		t.Fatalf("expected default tenant blocking payload, got %+v", resp.Teams)
	}

	user, err := store.GetUserByID(context.Background(), targetUserID)
	if err != nil {
		t.Fatalf("lookup blocked user: %v", err)
	}
	if user == nil {
		t.Fatal("expected blocked user to remain")
	}
}

func TestHandleDeleteUserDeletesWhenTeamHasAnotherOwner(t *testing.T) {
	h, store, actorUserID, targetUserID := setupAdminTestEnv(t)

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}
	if err := store.AddTeamMember(context.Background(), defaultTenantID, actorUserID, database.TenantRoleOwner, targetUserID); err != nil {
		t.Fatalf("promote actor to owner: %v", err)
	}

	req := withAdminTestUser(httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+targetUserID.String(), nil), actorUserID)
	req.SetPathValue("id", targetUserID.String())
	w := httptest.NewRecorder()

	h.handleDeleteUser().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	user, err := store.GetUserByID(context.Background(), targetUserID)
	if err != nil {
		t.Fatalf("lookup deleted user: %v", err)
	}
	if user != nil {
		t.Fatalf("expected user to be deleted, got %+v", user)
	}
}
