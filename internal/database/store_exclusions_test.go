package database

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func setupExclusionStore(t *testing.T) (*Store, uuid.UUID, uuid.UUID) {
	t.Helper()

	store := NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	userID, err := store.CreateUser(context.Background(), "owner@example.com", "hashed-secret")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	site, err := store.CreateSite(context.Background(), userID, "exclusion-test.example")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	return store, userID, site.ID
}

func TestExclusionCRUD(t *testing.T) {
	store, userID, siteID := setupExclusionStore(t)
	defer store.Close()

	ctx := context.Background()

	instanceRule, err := store.CreateInstanceExclusion(ctx, "203.0.113.5/32", "monitor", userID)
	if err != nil {
		t.Fatalf("create instance exclusion: %v", err)
	}
	if instanceRule.ID == uuid.Nil {
		t.Fatalf("expected instance exclusion id")
	}

	siteRule, err := store.CreateSiteExclusion(ctx, siteID, "10.0.0.0/8", "internal", userID)
	if err != nil {
		t.Fatalf("create site exclusion: %v", err)
	}
	if siteRule.ID == uuid.Nil {
		t.Fatalf("expected site exclusion id")
	}
	if siteRule.SiteID == nil || *siteRule.SiteID != siteID {
		t.Fatalf("expected site id %s on site exclusion", siteID)
	}

	instanceRules, err := store.ListInstanceExclusions(ctx)
	if err != nil {
		t.Fatalf("list instance exclusions: %v", err)
	}
	if len(instanceRules) != 1 {
		t.Fatalf("expected 1 instance exclusion, got %d", len(instanceRules))
	}

	siteRules, err := store.ListSiteExclusions(ctx, siteID)
	if err != nil {
		t.Fatalf("list site exclusions: %v", err)
	}
	if len(siteRules) != 1 {
		t.Fatalf("expected 1 site exclusion, got %d", len(siteRules))
	}

	instanceCIDRs, err := store.ListInstanceExclusionCIDRs(ctx)
	if err != nil {
		t.Fatalf("list instance exclusion cidrs: %v", err)
	}
	if len(instanceCIDRs) != 1 || instanceCIDRs[0] != "203.0.113.5/32" {
		t.Fatalf("unexpected instance cidrs: %#v", instanceCIDRs)
	}

	siteCIDRs, err := store.ListSiteExclusionCIDRs(ctx)
	if err != nil {
		t.Fatalf("list site exclusion cidrs: %v", err)
	}
	if len(siteCIDRs) != 1 {
		t.Fatalf("expected 1 site cidr rule, got %d", len(siteCIDRs))
	}
	if siteCIDRs[0].SiteID != siteID || siteCIDRs[0].CIDR != "10.0.0.0/8" {
		t.Fatalf("unexpected site cidr rule: %#v", siteCIDRs[0])
	}

	deleted, err := store.DeleteSiteExclusion(ctx, siteID, siteRule.ID)
	if err != nil {
		t.Fatalf("delete site exclusion: %v", err)
	}
	if !deleted {
		t.Fatalf("expected site exclusion to be deleted")
	}

	deleted, err = store.DeleteInstanceExclusion(ctx, instanceRule.ID)
	if err != nil {
		t.Fatalf("delete instance exclusion: %v", err)
	}
	if !deleted {
		t.Fatalf("expected instance exclusion to be deleted")
	}

	siteRules, err = store.ListSiteExclusions(ctx, siteID)
	if err != nil {
		t.Fatalf("list site exclusions after delete: %v", err)
	}
	if len(siteRules) != 0 {
		t.Fatalf("expected no site exclusions after delete, got %d", len(siteRules))
	}

	instanceRules, err = store.ListInstanceExclusions(ctx)
	if err != nil {
		t.Fatalf("list instance exclusions after delete: %v", err)
	}
	if len(instanceRules) != 0 {
		t.Fatalf("expected no instance exclusions after delete, got %d", len(instanceRules))
	}
}
