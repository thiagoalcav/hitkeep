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

	"hitkeep/internal/api"
	"hitkeep/internal/database"
)

func TestHandleGetTeams(t *testing.T) {
	h, store, userID := setupUserSecurityTestEnv(t)
	defer store.Close()

	req := withTestUser(httptest.NewRequest(http.MethodGet, "/api/user/teams", nil), userID)
	w := httptest.NewRecorder()

	h.handleGetTeams().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp struct {
		ActiveTeamID  uuid.UUID   `json:"active_team_id"`
		RecentTeamIDs []uuid.UUID `json:"recent_team_ids"`
		Teams         []api.Team  `json:"teams"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ActiveTeamID == uuid.Nil {
		t.Fatalf("expected active_team_id")
	}
	if len(resp.Teams) == 0 {
		t.Fatalf("expected at least one team")
	}
	if len(resp.RecentTeamIDs) == 0 || resp.RecentTeamIDs[0] != resp.ActiveTeamID {
		t.Fatalf("expected recent_team_ids to start with active team, got %+v", resp.RecentTeamIDs)
	}
}

func TestHandleSetActiveTeam(t *testing.T) {
	h, store, userID := setupUserSecurityTestEnv(t)
	defer store.Close()

	customTenantID := uuid.New()
	if _, err := store.DB().ExecContext(context.Background(),
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		customTenantID, "Custom Active", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert custom tenant: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"team_id": customTenantID.String()})
	req := withTestUser(httptest.NewRequest(http.MethodPut, "/api/user/teams/active", bytes.NewReader(body)), userID)
	w := httptest.NewRecorder()
	h.handleSetActiveTeam().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	if err := store.AddTeamMember(context.Background(), customTenantID, userID, database.TenantRoleAdmin, userID); err != nil {
		t.Fatalf("add team member: %v", err)
	}

	req = withTestUser(httptest.NewRequest(http.MethodPut, "/api/user/teams/active", bytes.NewReader(body)), userID)
	w = httptest.NewRecorder()
	h.handleSetActiveTeam().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var setResp struct {
		Status        string      `json:"status"`
		ActiveTeamID  uuid.UUID   `json:"active_team_id"`
		RecentTeamIDs []uuid.UUID `json:"recent_team_ids"`
	}
	if err := json.NewDecoder(w.Body).Decode(&setResp); err != nil {
		t.Fatalf("decode set active response: %v", err)
	}
	if setResp.Status != "ok" {
		t.Fatalf("expected status %q, got %q", "ok", setResp.Status)
	}
	if setResp.ActiveTeamID != customTenantID {
		t.Fatalf("expected active team %s, got %s", customTenantID, setResp.ActiveTeamID)
	}
	if len(setResp.RecentTeamIDs) == 0 || setResp.RecentTeamIDs[0] != customTenantID {
		t.Fatalf("expected recent_team_ids to start with %s, got %+v", customTenantID, setResp.RecentTeamIDs)
	}
}

func TestHandleAddTeamMemberAdminCannotAssignOwner(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	adminID, err := store.CreateUser(context.Background(), "admin@team.test", "hash")
	if err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	if err := store.AddTeamMember(context.Background(), defaultTenantID, adminID, database.TenantRoleAdmin, ownerID); err != nil {
		t.Fatalf("add admin user to team: %v", err)
	}

	targetID, err := store.CreateUser(context.Background(), "target-owner@team.test", "hash")
	if err != nil {
		t.Fatalf("create target user: %v", err)
	}
	if err := store.RemoveTeamMember(context.Background(), defaultTenantID, targetID); err != nil {
		t.Fatalf("remove default team membership for target: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"email": "target-owner@team.test",
		"role":  database.TenantRoleOwner,
	})

	req := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/teams/"+defaultTenantID.String()+"/members", bytes.NewReader(body)), adminID)
	req.SetPathValue("id", defaultTenantID.String())
	w := httptest.NewRecorder()

	h.handleAddTeamMember().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestHandleUpdateTeamOwnerSuccess(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"name":     "Updated Team Name",
		"logo_url": "https://example.com/logo.png",
	})
	req := withTestUser(httptest.NewRequest(http.MethodPatch, "/api/user/teams/"+defaultTenantID.String(), bytes.NewReader(body)), ownerID)
	req.SetPathValue("id", defaultTenantID.String())
	w := httptest.NewRecorder()

	h.handleUpdateTeam().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	team, err := store.GetTenant(context.Background(), defaultTenantID)
	if err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if team.Name != "Updated Team Name" {
		t.Fatalf("expected name %q, got %q", "Updated Team Name", team.Name)
	}
	if team.LogoURL != "https://example.com/logo.png" {
		t.Fatalf("expected logo_url %q, got %q", "https://example.com/logo.png", team.LogoURL)
	}
}

func TestHandleUpdateTeamOwnerSuccessViaDeprecatedPutAlias(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"name":     "Legacy Update Alias",
		"logo_url": "https://example.com/legacy-logo.png",
	})
	req := withTestUser(httptest.NewRequest(http.MethodPut, "/api/user/teams/"+defaultTenantID.String(), bytes.NewReader(body)), ownerID)
	req.SetPathValue("id", defaultTenantID.String())
	w := httptest.NewRecorder()

	h.handleUpdateTeam().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestHandleUpdateTeamMemberForbidden(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	memberID, err := store.CreateUser(context.Background(), "member@team.test", "hash")
	if err != nil {
		t.Fatalf("create member user: %v", err)
	}
	if err := store.AddTeamMember(context.Background(), defaultTenantID, memberID, database.TenantRoleMember, ownerID); err != nil {
		t.Fatalf("add member to team: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"name": "Should Fail"})
	req := withTestUser(httptest.NewRequest(http.MethodPatch, "/api/user/teams/"+defaultTenantID.String(), bytes.NewReader(body)), memberID)
	req.SetPathValue("id", defaultTenantID.String())
	w := httptest.NewRecorder()

	h.handleUpdateTeam().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestHandleUpdateTeamEmptyNameRejected(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"name": ""})
	req := withTestUser(httptest.NewRequest(http.MethodPatch, "/api/user/teams/"+defaultTenantID.String(), bytes.NewReader(body)), ownerID)
	req.SetPathValue("id", defaultTenantID.String())
	w := httptest.NewRecorder()

	h.handleUpdateTeam().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleAddTeamMemberSelfInviteBlocked(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	// The owner is already a member — inviting themselves should be rejected.
	ownerUser, err := store.GetUserByID(context.Background(), ownerID)
	if err != nil || ownerUser == nil {
		t.Fatalf("get owner user: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"email": ownerUser.Email,
		"role":  database.TenantRoleMember,
	})

	req := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/teams/"+defaultTenantID.String()+"/members", bytes.NewReader(body)), ownerID)
	req.SetPathValue("id", defaultTenantID.String())
	w := httptest.NewRecorder()

	h.handleAddTeamMember().ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
	}
}

func TestHandleAddTeamMemberRoleChangeAllowed(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	// Create a user and add them as a member.
	memberID, err := store.CreateUser(context.Background(), "existing-member@team.test", "hash")
	if err != nil {
		t.Fatalf("create member user: %v", err)
	}
	if err := store.AddTeamMember(context.Background(), defaultTenantID, memberID, database.TenantRoleMember, ownerID); err != nil {
		t.Fatalf("add member to team: %v", err)
	}

	// Changing an existing member's role should succeed.
	body, _ := json.Marshal(map[string]string{
		"email": "existing-member@team.test",
		"role":  database.TenantRoleAdmin,
	})

	req := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/teams/"+defaultTenantID.String()+"/members", bytes.NewReader(body)), ownerID)
	req.SetPathValue("id", defaultTenantID.String())
	w := httptest.NewRecorder()

	h.handleAddTeamMember().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify the role was actually updated.
	updatedRole, err := store.GetTenantRole(context.Background(), defaultTenantID, memberID)
	if err != nil {
		t.Fatalf("get updated role: %v", err)
	}
	if updatedRole != database.TenantRoleAdmin {
		t.Fatalf("expected role %q, got %q", database.TenantRoleAdmin, updatedRole)
	}
}

func TestHandleAddTeamMemberCreatesPendingInvite(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	customTenantID := uuid.New()
	if _, err := store.DB().ExecContext(context.Background(),
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		customTenantID, "Invites", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert custom tenant: %v", err)
	}
	if err := store.AddTeamMember(context.Background(), customTenantID, ownerID, database.TenantRoleOwner, ownerID); err != nil {
		t.Fatalf("add owner to custom tenant: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"email": "pending-invite@team.test",
		"role":  database.TenantRoleAdmin,
	})
	req := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/teams/"+customTenantID.String()+"/members", bytes.NewReader(body)), ownerID)
	req.SetPathValue("id", customTenantID.String())
	w := httptest.NewRecorder()

	h.handleAddTeamMember().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp struct {
		Status   string         `json:"status"`
		IsInvite bool           `json:"is_invite"`
		Invite   api.TeamInvite `json:"invite"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.IsInvite {
		t.Fatalf("expected pending invite response")
	}
	if resp.Invite.Status != database.TeamInviteStatusPending {
		t.Fatalf("expected invite status %q, got %q", database.TeamInviteStatusPending, resp.Invite.Status)
	}

	invites, err := store.ListTeamInvites(context.Background(), customTenantID)
	if err != nil {
		t.Fatalf("list team invites: %v", err)
	}
	if len(invites) != 1 {
		t.Fatalf("expected 1 pending invite, got %d", len(invites))
	}

	invitee, err := store.GetUserByEmail(context.Background(), "pending-invite@team.test")
	if err != nil || invitee == nil {
		t.Fatalf("get invitee user: %v", err)
	}
	isMember, err := store.IsTenantMember(context.Background(), customTenantID, invitee.ID)
	if err != nil {
		t.Fatalf("check tenant membership: %v", err)
	}
	if isMember {
		t.Fatalf("expected pending invite not to create active team membership")
	}
}

func TestHandleGetAndRevokeTeamInvites(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	customTenantID := uuid.New()
	if _, err := store.DB().ExecContext(context.Background(),
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		customTenantID, "Invite Admin", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert custom tenant: %v", err)
	}
	if err := store.AddTeamMember(context.Background(), customTenantID, ownerID, database.TenantRoleOwner, ownerID); err != nil {
		t.Fatalf("add owner to custom tenant: %v", err)
	}

	invite, err := store.CreateTeamInvite(context.Background(), customTenantID, "revoke-invite@team.test", database.TenantRoleMember, nil, ownerID)
	if err != nil {
		t.Fatalf("create team invite: %v", err)
	}

	listReq := withTestUser(httptest.NewRequest(http.MethodGet, "/api/user/teams/"+customTenantID.String()+"/invites", nil), ownerID)
	listReq.SetPathValue("id", customTenantID.String())
	listW := httptest.NewRecorder()
	h.handleGetTeamInvites().ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, listW.Code, listW.Body.String())
	}

	var invites []api.TeamInvite
	if err := json.NewDecoder(listW.Body).Decode(&invites); err != nil {
		t.Fatalf("decode invites response: %v", err)
	}
	if len(invites) != 1 || invites[0].ID != invite.ID {
		t.Fatalf("expected invite %s, got %+v", invite.ID, invites)
	}

	revokeReq := withTestUser(httptest.NewRequest(http.MethodDelete, "/api/user/teams/"+customTenantID.String()+"/invites/"+invite.ID.String(), nil), ownerID)
	revokeReq.SetPathValue("id", customTenantID.String())
	revokeReq.SetPathValue("inviteId", invite.ID.String())
	revokeW := httptest.NewRecorder()
	h.handleRevokeTeamInvite().ServeHTTP(revokeW, revokeReq)
	if revokeW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, revokeW.Code, revokeW.Body.String())
	}

	invites, err = store.ListTeamInvites(context.Background(), customTenantID)
	if err != nil {
		t.Fatalf("list team invites after revoke: %v", err)
	}
	if len(invites) != 0 {
		t.Fatalf("expected no pending invites after revoke, got %d", len(invites))
	}
}

func TestHandleRemoveTeamMemberLastOwnerBlocked(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	req := withTestUser(
		httptest.NewRequest(
			http.MethodDelete,
			"/api/user/teams/"+defaultTenantID.String()+"/members/"+ownerID.String(),
			nil,
		),
		ownerID,
	)
	req.SetPathValue("id", defaultTenantID.String())
	req.SetPathValue("userId", ownerID.String())
	w := httptest.NewRecorder()

	h.handleRemoveTeamMember().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleCreateTeamSuccess(t *testing.T) {
	h, store, userID := setupUserSecurityTestEnv(t)
	defer store.Close()

	body, _ := json.Marshal(map[string]string{
		"name":     "My New Team",
		"logo_url": "https://example.com/logo.png",
	})

	req := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/teams", bytes.NewReader(body)), userID)
	w := httptest.NewRecorder()

	h.handleCreateTeam().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp struct {
		Team api.Team `json:"team"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Team.Name != "My New Team" {
		t.Fatalf("expected name %q, got %q", "My New Team", resp.Team.Name)
	}
	if resp.Team.Role != database.TenantRoleOwner {
		t.Fatalf("expected role %q, got %q", database.TenantRoleOwner, resp.Team.Role)
	}

	// Verify creator is an owner member.
	role, err := store.GetTenantRole(context.Background(), resp.Team.ID, userID)
	if err != nil {
		t.Fatalf("get tenant role: %v", err)
	}
	if role != database.TenantRoleOwner {
		t.Fatalf("expected role %q, got %q", database.TenantRoleOwner, role)
	}
}

func TestHandleCreateTeamEmptyNameRejected(t *testing.T) {
	h, store, userID := setupUserSecurityTestEnv(t)
	defer store.Close()

	body, _ := json.Marshal(map[string]string{"name": ""})

	req := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/teams", bytes.NewReader(body)), userID)
	w := httptest.NewRecorder()

	h.handleCreateTeam().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestHandleLeaveTeamSuccess(t *testing.T) {
	h, store, userID := setupUserSecurityTestEnv(t)
	defer store.Close()

	customTenantID := uuid.New()
	if _, err := store.DB().ExecContext(context.Background(),
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		customTenantID, "Leave Team", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert custom tenant: %v", err)
	}
	if err := store.AddTeamMember(context.Background(), customTenantID, userID, database.TenantRoleAdmin, userID); err != nil {
		t.Fatalf("add team member: %v", err)
	}
	if err := store.SetActiveTenantID(context.Background(), userID, customTenantID); err != nil {
		t.Fatalf("set active tenant: %v", err)
	}

	req := withTestUser(httptest.NewRequest(http.MethodDelete, "/api/user/teams/"+customTenantID.String()+"/leave", nil), userID)
	req.SetPathValue("id", customTenantID.String())
	w := httptest.NewRecorder()

	h.handleLeaveTeam().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	isMember, err := store.IsTenantMember(context.Background(), customTenantID, userID)
	if err != nil {
		t.Fatalf("check membership: %v", err)
	}
	if isMember {
		t.Fatalf("expected user to be removed from team")
	}
}

func TestHandleGetTeamAudit(t *testing.T) {
	h, store, ownerID := setupUserSecurityTestEnv(t)
	defer store.Close()

	defaultTenantID, err := store.GetDefaultTenantID(context.Background())
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	memberID, err := store.CreateUser(context.Background(), "audit-member@team.test", "hash")
	if err != nil {
		t.Fatalf("create member user: %v", err)
	}
	if err := store.AddTeamMember(context.Background(), defaultTenantID, memberID, database.TenantRoleMember, ownerID); err != nil {
		t.Fatalf("add member to team: %v", err)
	}

	// Trigger an auditable action.
	body, _ := json.Marshal(map[string]string{
		"email": "audit-member@team.test",
		"role":  database.TenantRoleAdmin,
	})
	updateReq := withTestUser(httptest.NewRequest(http.MethodPost, "/api/user/teams/"+defaultTenantID.String()+"/members", bytes.NewReader(body)), ownerID)
	updateReq.SetPathValue("id", defaultTenantID.String())
	updateW := httptest.NewRecorder()
	h.handleAddTeamMember().ServeHTTP(updateW, updateReq)
	if updateW.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, updateW.Code)
	}

	req := withTestUser(httptest.NewRequest(http.MethodGet, "/api/user/teams/"+defaultTenantID.String()+"/audit", nil), ownerID)
	req.SetPathValue("id", defaultTenantID.String())
	w := httptest.NewRecorder()
	h.handleGetTeamAudit().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var entries []api.TeamAuditEntry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one audit entry")
	}
}
