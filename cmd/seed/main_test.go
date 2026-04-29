package main

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"

	"hitkeep/internal/database"
)

func TestEnsureUserResetsExistingUserPassword(t *testing.T) {
	ctx := context.Background()
	store := database.NewStore(filepath.Join(t.TempDir(), "seed.db"))
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	userID := ensureUser(ctx, store, "demo@example.com", "old-password")
	original, err := store.GetUserByEmail(ctx, "demo@example.com")
	if err != nil {
		t.Fatalf("load original user: %v", err)
	}
	if original == nil {
		t.Fatal("expected original user")
	}
	if !seedPasswordMatches(t, "old-password", original.Password) {
		t.Fatal("expected original password to match")
	}

	reusedID := ensureUser(ctx, store, "demo@example.com", "demo1234")
	if reusedID != userID {
		t.Fatalf("expected existing user id %s, got %s", userID, reusedID)
	}

	updated, err := store.GetUserByEmail(ctx, "demo@example.com")
	if err != nil {
		t.Fatalf("load updated user: %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated user")
	}
	if !seedPasswordMatches(t, "demo1234", updated.Password) {
		t.Fatal("expected reseeded password to match")
	}
	if seedPasswordMatches(t, "old-password", updated.Password) {
		t.Fatal("expected old password to stop matching")
	}
}

func TestSeedActivationFixturesKeepsPrimaryDemoSiteFirst(t *testing.T) {
	ctx := context.Background()
	store := database.NewStore(filepath.Join(t.TempDir(), "seed.db"))
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	userID := ensureUser(ctx, store, "demo@example.com", "demo1234")
	seedTeam(ctx, store, userID)

	primary, err := ensureSiteInActiveTeam(ctx, store, userID, "acme-analytics.io")
	if err != nil {
		t.Fatalf("ensure primary site: %v", err)
	}

	seedActivationFixtures(ctx, store, userID, primary.ID)

	sites, err := store.GetSites(ctx, userID)
	if err != nil {
		t.Fatalf("get sites: %v", err)
	}
	if len(sites) < 3 {
		t.Fatalf("expected primary and activation fixture sites, got %d", len(sites))
	}
	if sites[0].ID != primary.ID {
		t.Fatalf("expected primary demo site first, got %s (%s)", sites[0].Domain, sites[0].ID)
	}
}

func seedPasswordMatches(t *testing.T, password string, encoded string) bool {
	t.Helper()

	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		t.Fatalf("invalid password hash format: %q", encoded)
	}
	if parts[1] != "argon2id" {
		t.Fatalf("unexpected password algorithm: %q", parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		t.Fatalf("parse argon2 version: %v", err)
	}
	if version != argon2.Version {
		t.Fatalf("unexpected argon2 version: %d", version)
	}

	var memory uint32
	var timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		t.Fatalf("parse argon2 params: %v", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		t.Fatalf("decode salt: %v", err)
	}
	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		t.Fatalf("decode hash: %v", err)
	}

	comparisonHash := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(decodedHash)))
	return subtle.ConstantTimeCompare(decodedHash, comparisonHash) == 1
}
