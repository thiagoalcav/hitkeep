package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/auth"
)

func setupTenantStore(t *testing.T) *Store {
	t.Helper()

	store := NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	return store
}

func TestCreateUserAddsDefaultTenantMembership(t *testing.T) {
	store := setupTenantStore(t)
	ctx := context.Background()

	defaultTenantID, err := store.GetDefaultTenantID(ctx)
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	ownerID, err := store.CreateUser(ctx, "owner@tenant.test", "hash")
	if err != nil {
		t.Fatalf("create owner user: %v", err)
	}
	memberID, err := store.CreateUser(ctx, "member@tenant.test", "hash")
	if err != nil {
		t.Fatalf("create member user: %v", err)
	}

	var ownerRole string
	if err := store.DB().QueryRowContext(ctx,
		"SELECT role FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
		defaultTenantID, ownerID,
	).Scan(&ownerRole); err != nil {
		t.Fatalf("query owner tenant role: %v", err)
	}
	if ownerRole != TenantRoleOwner {
		t.Fatalf("expected owner role %q, got %q", TenantRoleOwner, ownerRole)
	}

	var memberRole string
	if err := store.DB().QueryRowContext(ctx,
		"SELECT role FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
		defaultTenantID, memberID,
	).Scan(&memberRole); err != nil {
		t.Fatalf("query member tenant role: %v", err)
	}
	if memberRole != TenantRoleMember {
		t.Fatalf("expected member role %q, got %q", TenantRoleMember, memberRole)
	}

	var isDefault bool
	if err := store.DB().QueryRowContext(ctx, "SELECT is_default FROM tenants WHERE id = ?", defaultTenantID).Scan(&isDefault); err != nil {
		t.Fatalf("query default tenant: %v", err)
	}
	if !isDefault {
		t.Fatal("expected default tenant is_default = true")
	}
}

func TestGetPrimaryTenantIDFallsBackToDefault(t *testing.T) {
	store := setupTenantStore(t)
	ctx := context.Background()

	defaultTenantID, err := store.GetDefaultTenantID(ctx)
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	userID := uuid.New()
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO users (id, email, password, created_at) VALUES (?, ?, ?, ?)",
		userID, "nomembership@tenant.test", "hash", time.Now().UTC(),
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	tenantID, err := store.GetPrimaryTenantID(ctx, userID)
	if err != nil {
		t.Fatalf("get primary tenant: %v", err)
	}
	if tenantID != defaultTenantID {
		t.Fatalf("expected default tenant %s, got %s", defaultTenantID, tenantID)
	}
}

func TestAddSiteMemberRequiresTenantMembership(t *testing.T) {
	store := setupTenantStore(t)
	ctx := context.Background()

	ownerID, err := store.CreateUser(ctx, "owner@cross-tenant.test", "hash")
	if err != nil {
		t.Fatalf("create owner user: %v", err)
	}
	outsiderID, err := store.CreateUser(ctx, "outsider@cross-tenant.test", "hash")
	if err != nil {
		t.Fatalf("create outsider user: %v", err)
	}

	tenantID := uuid.New()
	now := time.Now().UTC()
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		tenantID, "Custom Tenant", now,
	); err != nil {
		t.Fatalf("create custom tenant: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenant_members (id, tenant_id, user_id, role, added_at, added_by) VALUES (?, ?, ?, ?, ?, ?)",
		uuid.New(), tenantID, ownerID, TenantRoleOwner, now, ownerID,
	); err != nil {
		t.Fatalf("add owner to custom tenant: %v", err)
	}

	siteID := uuid.New()
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO sites (id, user_id, domain, created_at) VALUES (?, ?, ?, ?)",
		siteID, ownerID, "cross-tenant.test", now,
	); err != nil {
		t.Fatalf("create tenant site: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO site_tenants (site_id, tenant_id, created_at) VALUES (?, ?, ?)",
		siteID, tenantID, now,
	); err != nil {
		t.Fatalf("create site tenant mapping: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO site_members (id, site_id, user_id, role, added_at, added_by) VALUES (?, ?, ?, ?, ?, ?)",
		uuid.New(), siteID, ownerID, auth.SiteOwner, now, ownerID,
	); err != nil {
		t.Fatalf("create owner site membership: %v", err)
	}

	err = store.AddSiteMember(ctx, siteID, outsiderID, auth.SiteViewer, ownerID)
	if err == nil {
		t.Fatalf("expected tenant membership error")
	}
	if !strings.Contains(err.Error(), "not part of tenant") {
		t.Fatalf("expected tenant membership error, got %v", err)
	}

	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenant_members (id, tenant_id, user_id, role, added_at, added_by) VALUES (?, ?, ?, ?, ?, ?)",
		uuid.New(), tenantID, outsiderID, TenantRoleMember, now, ownerID,
	); err != nil {
		t.Fatalf("add outsider to custom tenant: %v", err)
	}

	if err := store.AddSiteMember(ctx, siteID, outsiderID, auth.SiteViewer, ownerID); err != nil {
		t.Fatalf("add site member after tenant membership: %v", err)
	}
}

func TestSetActiveTenantIDAndScopedSites(t *testing.T) {
	store := setupTenantStore(t)
	ctx := context.Background()

	userID, err := store.CreateUser(ctx, "owner@active-team.test", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	defaultTenantID, err := store.GetDefaultTenantID(ctx)
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	defaultSite, err := store.CreateSite(ctx, userID, "default-team.test")
	if err != nil {
		t.Fatalf("create default team site: %v", err)
	}

	customTenantID := uuid.New()
	now := time.Now().UTC()
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		customTenantID, "Custom Team", now,
	); err != nil {
		t.Fatalf("create custom tenant: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenant_members (tenant_id, user_id, role, added_by) VALUES (?, ?, ?, ?)",
		customTenantID, userID, TenantRoleAdmin, userID,
	); err != nil {
		t.Fatalf("add user to custom tenant: %v", err)
	}

	otherTenantID := uuid.New()
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		otherTenantID, "Other Team", now,
	); err != nil {
		t.Fatalf("create other tenant: %v", err)
	}

	if err := store.SetActiveTenantID(ctx, userID, otherTenantID); err == nil {
		t.Fatalf("expected non-member active team assignment to fail")
	}

	if err := store.SetActiveTenantID(ctx, userID, customTenantID); err != nil {
		t.Fatalf("set custom active tenant: %v", err)
	}

	activeTenantID, err := store.GetActiveTenantID(ctx, userID)
	if err != nil {
		t.Fatalf("get active tenant: %v", err)
	}
	if activeTenantID != customTenantID {
		t.Fatalf("expected active tenant %s, got %s", customTenantID, activeTenantID)
	}

	customSite, err := store.CreateSite(ctx, userID, "custom-team.test")
	if err != nil {
		t.Fatalf("create custom team site: %v", err)
	}

	sites, err := store.GetSites(ctx, userID)
	if err != nil {
		t.Fatalf("get scoped sites: %v", err)
	}
	if len(sites) != 1 || sites[0].ID != customSite.ID {
		t.Fatalf("expected only custom team site %s, got %+v", customSite.ID, sites)
	}

	if err := store.SetActiveTenantID(ctx, userID, defaultTenantID); err != nil {
		t.Fatalf("set default active tenant: %v", err)
	}
	sites, err = store.GetSites(ctx, userID)
	if err != nil {
		t.Fatalf("get default scoped sites: %v", err)
	}
	if len(sites) != 1 || sites[0].ID != defaultSite.ID {
		t.Fatalf("expected only default team site %s, got %+v", defaultSite.ID, sites)
	}

	_, activeTeamID, err := store.ListUserTeams(ctx, userID)
	if err != nil {
		t.Fatalf("list user teams: %v", err)
	}
	if activeTeamID != defaultTenantID {
		t.Fatalf("expected active team %s, got %s", defaultTenantID, activeTeamID)
	}
}

func TestLeaveTeamReassignsActiveTenant(t *testing.T) {
	store := setupTenantStore(t)
	ctx := context.Background()

	userID, err := store.CreateUser(ctx, "leave-team@tenant.test", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	customTenantID := uuid.New()
	now := time.Now().UTC()
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO tenants (id, name, created_at) VALUES (?, ?, ?)",
		customTenantID, "Leave Team", now,
	); err != nil {
		t.Fatalf("create custom tenant: %v", err)
	}
	if err := store.AddTeamMember(ctx, customTenantID, userID, TenantRoleAdmin, userID); err != nil {
		t.Fatalf("add user to custom tenant: %v", err)
	}

	if err := store.SetActiveTenantID(ctx, userID, customTenantID); err != nil {
		t.Fatalf("set active custom tenant: %v", err)
	}

	defaultTenantID, err := store.GetDefaultTenantID(ctx)
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	nextActiveTenantID, err := store.LeaveTeam(ctx, customTenantID, userID)
	if err != nil {
		t.Fatalf("leave team: %v", err)
	}
	if nextActiveTenantID != defaultTenantID {
		t.Fatalf("expected next active tenant %s, got %s", defaultTenantID, nextActiveTenantID)
	}

	isMember, err := store.IsTenantMember(ctx, customTenantID, userID)
	if err != nil {
		t.Fatalf("check custom tenant membership: %v", err)
	}
	if isMember {
		t.Fatalf("expected user to be removed from custom tenant")
	}
}

func TestAppendAndListTeamAuditEntries(t *testing.T) {
	store := setupTenantStore(t)
	ctx := context.Background()

	actorID, err := store.CreateUser(ctx, "audit-actor@tenant.test", "hash")
	if err != nil {
		t.Fatalf("create actor user: %v", err)
	}
	targetID, err := store.CreateUser(ctx, "audit-target@tenant.test", "hash")
	if err != nil {
		t.Fatalf("create target user: %v", err)
	}

	tenantID, err := store.GetDefaultTenantID(ctx)
	if err != nil {
		t.Fatalf("get default tenant: %v", err)
	}

	if err := store.AppendTeamAuditEntry(ctx, tenantID, actorID, "member.added", "Added target user", &targetID); err != nil {
		t.Fatalf("append team audit entry: %v", err)
	}

	entries, err := store.ListTeamAuditEntries(ctx, tenantID, 20)
	if err != nil {
		t.Fatalf("list team audit entries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one team audit entry")
	}

	entry := entries[0]
	if entry.Action != "member.added" {
		t.Fatalf("expected action %q, got %q", "member.added", entry.Action)
	}
	if entry.TeamID != tenantID {
		t.Fatalf("expected team_id %s, got %s", tenantID, entry.TeamID)
	}
	if entry.ActorUserID == nil || *entry.ActorUserID != actorID {
		t.Fatalf("expected actor %s, got %v", actorID, entry.ActorUserID)
	}
	if entry.TargetUserID == nil || *entry.TargetUserID != targetID {
		t.Fatalf("expected target %s, got %v", targetID, entry.TargetUserID)
	}
}

func TestCanAssignTenantRole(t *testing.T) {
	cases := []struct {
		name      string
		actor     string
		requested string
		want      bool
	}{
		{name: "owner_to_owner", actor: TenantRoleOwner, requested: TenantRoleOwner, want: true},
		{name: "owner_to_admin", actor: TenantRoleOwner, requested: TenantRoleAdmin, want: true},
		{name: "owner_to_member", actor: TenantRoleOwner, requested: TenantRoleMember, want: true},
		{name: "admin_to_owner", actor: TenantRoleAdmin, requested: TenantRoleOwner, want: false},
		{name: "admin_to_admin", actor: TenantRoleAdmin, requested: TenantRoleAdmin, want: true},
		{name: "admin_to_member", actor: TenantRoleAdmin, requested: TenantRoleMember, want: true},
		{name: "member_to_admin", actor: TenantRoleMember, requested: TenantRoleAdmin, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CanAssignTenantRole(tc.actor, tc.requested)
			if got != tc.want {
				t.Fatalf("CanAssignTenantRole(%q,%q) = %v, want %v", tc.actor, tc.requested, got, tc.want)
			}
		})
	}
}
