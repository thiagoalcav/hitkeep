package database

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/auth"
)

func TestGetReportSubscriptionsAndDigestsRequireCurrentTenantMembership(t *testing.T) {
	store := setupTenantStore(t)
	ctx := context.Background()

	ownerID, err := store.CreateUser(ctx, "digest-owner@test.dev", "hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	recipientID, err := store.CreateUser(ctx, "digest-recipient@test.dev", "hash")
	if err != nil {
		t.Fatalf("create recipient: %v", err)
	}

	teamA, err := store.CreateTenant(ctx, ownerID, "Digest Team A", "")
	if err != nil {
		t.Fatalf("create team A: %v", err)
	}
	teamB, err := store.CreateTenant(ctx, ownerID, "Digest Team B", "")
	if err != nil {
		t.Fatalf("create team B: %v", err)
	}
	if err := store.AddTeamMember(ctx, teamA.ID, recipientID, TenantRoleMember, ownerID); err != nil {
		t.Fatalf("add recipient to team A: %v", err)
	}
	if err := store.AddTeamMember(ctx, teamB.ID, recipientID, TenantRoleMember, ownerID); err != nil {
		t.Fatalf("add recipient to team B: %v", err)
	}

	if err := store.SetActiveTenantID(ctx, ownerID, teamA.ID); err != nil {
		t.Fatalf("set owner active team A: %v", err)
	}
	siteA, err := store.CreateSite(ctx, ownerID, "alpha.example.test")
	if err != nil {
		t.Fatalf("create site A: %v", err)
	}
	if err := store.AddSiteMember(ctx, siteA.ID, recipientID, auth.SiteViewer, ownerID); err != nil {
		t.Fatalf("add recipient to site A: %v", err)
	}

	if err := store.SetActiveTenantID(ctx, ownerID, teamB.ID); err != nil {
		t.Fatalf("set owner active team B: %v", err)
	}
	siteB, err := store.CreateSite(ctx, ownerID, "bravo.example.test")
	if err != nil {
		t.Fatalf("create site B: %v", err)
	}
	if err := store.AddSiteMember(ctx, siteB.ID, recipientID, auth.SiteViewer, ownerID); err != nil {
		t.Fatalf("add recipient to site B: %v", err)
	}

	if err := store.UpsertDigestSubscription(ctx, recipientID, api.ReportFrequencyDaily, true); err != nil {
		t.Fatalf("enable digest subscription: %v", err)
	}

	subs, err := store.GetReportSubscriptions(ctx, recipientID)
	if err != nil {
		t.Fatalf("get report subscriptions: %v", err)
	}
	if got := len(subs.Sites); got != 2 {
		t.Fatalf("expected 2 accessible sites before removal, got %d", got)
	}

	if _, err := store.DB().ExecContext(ctx,
		"DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
		teamB.ID, recipientID,
	); err != nil {
		t.Fatalf("delete team B membership directly: %v", err)
	}

	subs, err = store.GetReportSubscriptions(ctx, recipientID)
	if err != nil {
		t.Fatalf("get report subscriptions after membership deletion: %v", err)
	}
	if got := len(subs.Sites); got != 1 {
		t.Fatalf("expected 1 accessible site after team B removal, got %d", got)
	}
	if subs.Sites[0].SiteID != siteA.ID {
		t.Fatalf("expected only site A to remain accessible, got %s", subs.Sites[0].SiteID)
	}

	pending, err := store.GetPendingDigests(ctx, api.ReportFrequencyDaily)
	if err != nil {
		t.Fatalf("get pending digests: %v", err)
	}
	if got := len(pending); got != 1 {
		t.Fatalf("expected 1 pending digest, got %d", got)
	}
	if got := len(pending[0].Sites); got != 1 {
		t.Fatalf("expected digest to include 1 site after team B removal, got %d", got)
	}
	if pending[0].Sites[0].SiteID != siteA.ID {
		t.Fatalf("expected digest to include only site A, got %s", pending[0].Sites[0].SiteID)
	}
}

func TestUpsertSiteReportSubscriptionRequiresTenantBackedSiteAccess(t *testing.T) {
	store := setupTenantStore(t)
	ctx := context.Background()

	ownerID, err := store.CreateUser(ctx, "sub-owner@test.dev", "hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	targetID, err := store.CreateUser(ctx, "sub-target@test.dev", "hash")
	if err != nil {
		t.Fatalf("create target: %v", err)
	}

	team, err := store.CreateTenant(ctx, ownerID, "Subscriptions Team", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := store.SetActiveTenantID(ctx, ownerID, team.ID); err != nil {
		t.Fatalf("set owner active team: %v", err)
	}

	site, err := store.CreateSite(ctx, ownerID, "subscription.example.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx,
		"INSERT INTO site_members (id, site_id, user_id, role, added_at, added_by) VALUES (?, ?, ?, ?, NOW(), ?)",
		uuid.New(), site.ID, targetID, auth.SiteViewer, ownerID,
	); err != nil {
		t.Fatalf("insert stale site membership: %v", err)
	}

	err = store.UpsertSiteReportSubscription(ctx, targetID, site.ID, api.ReportFrequencyDaily, true)
	if !errors.Is(err, ErrSiteAccessRequired) {
		t.Fatalf("expected ErrSiteAccessRequired, got %v", err)
	}
}
