package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/auth"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

func setupAdminTestEnv(t *testing.T) (*handler, *database.Store, *database.TenantStoreManager, string, uuid.UUID, uuid.UUID) {
	t.Helper()

	basePath := t.TempDir()
	store := database.NewStore(filepath.Join(basePath, "shared.db"))
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

	tenantStores := database.NewTenantStoreManager(store, basePath)
	t.Cleanup(func() { _ = tenantStores.Close() })

	ctx := &shared.Context{
		Store:        store,
		TenantStores: tenantStores,
		Config: &config.Config{
			PublicURL: "http://localhost:8080",
			JWTSecret: "test-secret",
		},
	}

	return &handler{ctx: ctx}, store, tenantStores, basePath, actorUserID, targetUserID
}

func withAdminTestUser(req *http.Request, userID uuid.UUID) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
}

func TestHandleDeleteUserReturnsConflictForSoleOwner(t *testing.T) {
	h, store, _, _, actorUserID, targetUserID := setupAdminTestEnv(t)

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
	h, store, _, _, actorUserID, targetUserID := setupAdminTestEnv(t)

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

func TestHandleDisableUser2FARequiresOwner(t *testing.T) {
	h, _, _, _, actorUserID, targetUserID := setupAdminTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodPost, "/api/admin/users/"+targetUserID.String()+"/disable-2fa", nil), actorUserID)
	req.SetPathValue("id", targetUserID.String())
	w := httptest.NewRecorder()

	h.handleDisableUser2FA().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, w.Code, w.Body.String())
	}
}

func TestHandleDisableUser2FADisablesTargetFactors(t *testing.T) {
	h, store, _, _, actorUserID, targetUserID := setupAdminTestEnv(t)
	ctx := context.Background()

	if err := store.UpdateInstanceRole(ctx, actorUserID, auth.InstanceOwner, targetUserID); err != nil {
		t.Fatalf("promote actor to owner: %v", err)
	}

	if err := store.EnableUserTOTP(ctx, targetUserID, "totp-secret"); err != nil {
		t.Fatalf("enable target totp: %v", err)
	}
	if _, err := store.CreateUserPasskey(ctx, targetUserID, "Recovery key", "credential-1", "public-key", nil); err != nil {
		t.Fatalf("create target passkey: %v", err)
	}
	token, err := store.CreateRememberMeToken(ctx, targetUserID)
	if err != nil {
		t.Fatalf("create remember me token: %v", err)
	}

	req := withAdminTestUser(httptest.NewRequest(http.MethodPost, "/api/admin/users/"+targetUserID.String()+"/disable-2fa", nil), actorUserID)
	req.SetPathValue("id", targetUserID.String())
	w := httptest.NewRecorder()

	h.handleDisableUser2FA().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp api.AdminDisableUserMFAResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %q", resp.Status)
	}
	if !resp.TOTPDisabled {
		t.Fatal("expected TOTP to be disabled")
	}
	if resp.PasskeysDeleted != 1 {
		t.Fatalf("expected 1 deleted passkey, got %d", resp.PasskeysDeleted)
	}
	if resp.SessionsInvalidated != 1 {
		t.Fatalf("expected 1 invalidated session, got %d", resp.SessionsInvalidated)
	}

	hasTOTP, err := store.HasEnabledTOTP(ctx, targetUserID)
	if err != nil {
		t.Fatalf("check target totp: %v", err)
	}
	if hasTOTP {
		t.Fatal("expected target totp to be disabled")
	}

	passkeys, err := store.ListUserPasskeys(ctx, targetUserID)
	if err != nil {
		t.Fatalf("list target passkeys: %v", err)
	}
	if len(passkeys) != 0 {
		t.Fatalf("expected target passkeys to be deleted, got %d", len(passkeys))
	}

	rememberedUserID, err := store.ValidateRememberMeToken(ctx, token)
	if err != nil {
		t.Fatalf("validate remember me token: %v", err)
	}
	if rememberedUserID != uuid.Nil {
		t.Fatalf("expected target remember me token to be invalidated, got %s", rememberedUserID)
	}
}

func TestHandleDeleteTeamPurgesArchivedTenantData(t *testing.T) {
	h, store, tenantStores, basePath, actorUserID, _ := setupAdminTestEnv(t)
	ctx := context.Background()

	team, err := store.CreateTenant(ctx, actorUserID, "Archived Team", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if _, err := tenantStores.ForTenant(ctx, team.ID); err != nil {
		t.Fatalf("open tenant store: %v", err)
	}

	dbPath := filepath.Join(basePath, "tenants", team.ID.String(), "hitkeep.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected tenant db to exist before purge: %v", err)
	}

	if err := store.ArchiveTenant(ctx, team.ID, actorUserID); err != nil {
		t.Fatalf("archive team: %v", err)
	}

	req := withAdminTestUser(httptest.NewRequest(http.MethodDelete, "/api/admin/teams/"+team.ID.String(), nil), actorUserID)
	req.SetPathValue("id", team.ID.String())
	w := httptest.NewRecorder()

	h.handleAdminDeleteTeam().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp api.AdminDeleteTeamResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TeamID != team.ID {
		t.Fatalf("expected deleted team id %s, got %s", team.ID, resp.TeamID)
	}

	remaining, err := store.GetTenant(ctx, team.ID)
	if err != nil {
		t.Fatalf("get tenant after purge: %v", err)
	}
	if remaining != nil {
		t.Fatalf("expected tenant to be deleted, got %+v", remaining)
	}

	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("expected tenant db to be removed, stat err=%v", err)
	}
}

func TestHandleDeleteTeamRequiresArchiveFirst(t *testing.T) {
	h, store, _, _, actorUserID, _ := setupAdminTestEnv(t)
	ctx := context.Background()

	team, err := store.CreateTenant(ctx, actorUserID, "Active Team", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}

	req := withAdminTestUser(httptest.NewRequest(http.MethodDelete, "/api/admin/teams/"+team.ID.String(), nil), actorUserID)
	req.SetPathValue("id", team.ID.String())
	w := httptest.NewRecorder()

	h.handleAdminDeleteTeam().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}
