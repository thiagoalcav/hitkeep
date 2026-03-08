package database

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func TestBuildTeamUsageSummary(t *testing.T) {
	store := NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	ownerID, err := store.CreateUser(ctx, "usage-owner@test.dev", "hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	memberID, err := store.CreateUser(ctx, "usage-member@test.dev", "hash")
	if err != nil {
		t.Fatalf("create member: %v", err)
	}

	team, err := store.CreateTenant(ctx, ownerID, "Usage Team", "")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if err := store.SetActiveTenantID(ctx, ownerID, team.ID); err != nil {
		t.Fatalf("set active tenant: %v", err)
	}
	if err := store.AddTeamMember(ctx, team.ID, memberID, TenantRoleMember, ownerID); err != nil {
		t.Fatalf("add member: %v", err)
	}

	site, err := store.CreateSite(ctx, ownerID, "usage-team.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	if _, err := store.CreateTeamInvite(ctx, team.ID, "pending-usage@test.dev", TenantRoleMember, nil, ownerID); err != nil {
		t.Fatalf("create invite: %v", err)
	}

	now := time.Now().UTC()
	for range 3 {
		if err := store.CreateHit(ctx, &api.Hit{
			SiteID:    site.ID,
			SessionID: uuid.New(),
			PageID:    uuid.New(),
			Path:      "/",
			Timestamp: now,
		}); err != nil {
			t.Fatalf("create hit: %v", err)
		}
	}
	for range 2 {
		if err := store.CreateEvent(ctx, &api.Event{
			SiteID:     site.ID,
			SessionID:  uuid.New(),
			Name:       "trial_started",
			Timestamp:  now,
			Properties: map[string]any{"plan": "pro"},
		}); err != nil {
			t.Fatalf("create event: %v", err)
		}
	}

	summary, err := store.BuildTeamUsageSummary(ctx, team.ID, store)
	if err != nil {
		t.Fatalf("build team usage summary: %v", err)
	}

	if summary.CurrentSites != 1 {
		t.Fatalf("expected 1 site, got %d", summary.CurrentSites)
	}
	if summary.CurrentMembers != 2 {
		t.Fatalf("expected 2 members, got %d", summary.CurrentMembers)
	}
	if summary.CurrentPendingInvites != 1 {
		t.Fatalf("expected 1 pending invite, got %d", summary.CurrentPendingInvites)
	}
	if summary.CurrentMonthlyEvents != 5 {
		t.Fatalf("expected 5 monthly ingested events, got %d", summary.CurrentMonthlyEvents)
	}
}
