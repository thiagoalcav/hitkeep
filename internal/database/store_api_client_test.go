package database

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/auth"
)

func setupAPIClientStore(t *testing.T) (*Store, uuid.UUID, uuid.UUID) {
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
	site, err := store.CreateSite(context.Background(), userID, "example.com")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	return store, userID, site.ID
}

func TestAPIClientLifecycle(t *testing.T) {
	store, userID, siteID := setupAPIClientStore(t)
	defer store.Close()

	ctx := context.Background()
	expiresAt := time.Now().UTC().Add(2 * time.Hour)

	client, token, err := store.CreateAPIClient(ctx, userID, "CI Sync", "pipeline", auth.InstanceAdmin, map[uuid.UUID]auth.SiteRole{
		siteID: auth.SiteOwner,
	}, &expiresAt)
	if err != nil {
		t.Fatalf("create api client: %v", err)
	}
	if token == "" {
		t.Fatalf("expected token")
	}
	if client.ID == uuid.Nil {
		t.Fatalf("expected client id")
	}
	if len(client.SiteRoles) != 1 {
		t.Fatalf("expected one site role, got %d", len(client.SiteRoles))
	}

	list, err := store.ListAPIClients(ctx, userID)
	if err != nil {
		t.Fatalf("list api clients: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one api client, got %d", len(list))
	}

	authz, err := store.GetAPIClientAuth(ctx, token)
	if err != nil {
		t.Fatalf("get api client auth: %v", err)
	}
	if authz == nil {
		t.Fatalf("expected api client auth")
	}
	if authz.UserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, authz.UserID)
	}
	if authz.InstanceRole != auth.InstanceAdmin {
		t.Fatalf("expected instance role %s, got %s", auth.InstanceAdmin, authz.InstanceRole)
	}
	if authz.SiteRoles[siteID] != auth.SiteOwner {
		t.Fatalf("expected site owner role for %s", siteID)
	}

	updated, err := store.UpdateAPIClient(ctx, userID, client.ID, "CI Sync Updated", "pipeline-updated", auth.InstanceUser, map[uuid.UUID]auth.SiteRole{
		siteID: auth.SiteViewer,
	}, &expiresAt, true)
	if err != nil {
		t.Fatalf("update api client: %v", err)
	}
	if updated == nil {
		t.Fatalf("expected updated client")
	}
	if updated.RevokedAt == nil {
		t.Fatalf("expected client to be revoked")
	}
	if updated.InstanceRole != string(auth.InstanceUser) {
		t.Fatalf("expected updated instance role %s, got %s", auth.InstanceUser, updated.InstanceRole)
	}

	revokedAuth, err := store.GetAPIClientAuth(ctx, token)
	if err != nil {
		t.Fatalf("get revoked api client auth: %v", err)
	}
	if revokedAuth != nil {
		t.Fatalf("expected revoked client token to be invalid")
	}

	if err := store.DeleteAPIClient(ctx, userID, client.ID); err != nil {
		t.Fatalf("delete api client: %v", err)
	}
	remaining, err := store.ListAPIClients(ctx, userID)
	if err != nil {
		t.Fatalf("list api clients after delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected no api clients after delete, got %d", len(remaining))
	}
}

func TestAPIClientAuthHonorsExpiration(t *testing.T) {
	store, userID, _ := setupAPIClientStore(t)
	defer store.Close()

	expiredAt := time.Now().UTC().Add(-1 * time.Minute)
	_, token, err := store.CreateAPIClient(context.Background(), userID, "Expired", "", auth.InstanceUser, nil, &expiredAt)
	if err != nil {
		t.Fatalf("create expired api client: %v", err)
	}

	authz, err := store.GetAPIClientAuth(context.Background(), token)
	if err != nil {
		t.Fatalf("get api client auth for expired token: %v", err)
	}
	if authz != nil {
		t.Fatalf("expected expired token to be invalid")
	}
}
