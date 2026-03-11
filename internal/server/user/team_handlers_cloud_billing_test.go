//go:build billing

package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/database"
	serverauth "hitkeep/internal/server/auth"
)

func TestHandleCreateTeamRejectsHostedCloud(t *testing.T) {
	h, store, userID := setupUserSecurityTestEnv(t)
	defer store.Close()

	h.ctx.Config.CloudHosted = true

	body, err := json.Marshal(map[string]string{
		"name": "Second Team",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/teams", bytes.NewReader(body)), userID)
	w := httptest.NewRecorder()
	h.handleCreateTeam().ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d: %s", http.StatusForbidden, w.Code, w.Body.String())
	}
}

func TestHandleUpsertTeamMemberRejectsExistingHostedCloudUserFromOtherTeam(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	h.ctx.Config.CloudHosted = true

	activeTeamID, err := store.GetActiveTenantID(context.Background(), ownerID)
	if err != nil {
		t.Fatalf("get active tenant: %v", err)
	}

	existingHash, err := serverauth.HashPassword("password123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	existingUserID, err := store.CreateUserWithoutDefaultTenant(context.Background(), "member@cloud.test", existingHash)
	if err != nil {
		t.Fatalf("create hosted user: %v", err)
	}

	otherTeamID := uuid.New()
	if _, err := store.DB().ExecContext(context.Background(),
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		otherTeamID, "Existing Hosted Team", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert hosted team: %v", err)
	}
	if err := store.AddTeamMember(context.Background(), otherTeamID, existingUserID, database.TenantRoleOwner, existingUserID); err != nil {
		t.Fatalf("add hosted member: %v", err)
	}

	body, err := json.Marshal(map[string]string{
		"email": "member@cloud.test",
		"role":  database.TenantRoleMember,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/teams/"+activeTeamID.String()+"/members", bytes.NewReader(body)), ownerID)
	req.SetPathValue("id", activeTeamID.String())
	w := httptest.NewRecorder()
	h.handleAddTeamMember().ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
	}
}
