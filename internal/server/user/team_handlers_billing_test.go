//go:build billing

package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hitkeep/internal/api"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/entitlements"
	"hitkeep/internal/server/shared"
)

func TestHandleGetTeamsIncludesPlanMetadata(t *testing.T) {
	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}

	userID, err := store.CreateUser(context.Background(), "plan@test.dev", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	h := &handler{
		ctx: &shared.Context{
			Store:        store,
			TenantStores: database.NewTenantStoreManager(store, t.TempDir()),
			Config:       &config.Config{CloudHosted: true},
			Entitlements: entitlements.NewProvider(&config.Config{
				CloudHosted:              true,
				CloudPlanCode:            "free",
				CloudPlanName:            "Free",
				CloudUpgradeURL:          "https://hitkeep.com/cloud/upgrade",
				CloudSupportURL:          "https://hitkeep.com/cloud/support",
				CloudMaxTeams:            1,
				CloudMaxSitesPerTeam:     3,
				CloudMaxMonthlyEvents:    10000,
				CloudMaxRetentionDays:    60,
				CloudMaxTeamMembers:      3,
				CloudAllowSSO:            false,
				CloudAllowCustomBranding: false,
			}),
		},
	}

	req := withTestUser(httptest.NewRequest(http.MethodGet, "/api/user/teams", nil), userID)
	w := httptest.NewRecorder()
	h.handleGetTeams().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp struct {
		Teams []api.Team `json:"teams"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode teams response: %v", err)
	}
	if len(resp.Teams) == 0 {
		t.Fatalf("expected at least one team")
	}
	if resp.Teams[0].Plan == nil {
		t.Fatalf("expected plan metadata on team payload")
	}
	if resp.Teams[0].Plan.Code != "free" || resp.Teams[0].Plan.Name != "Free" {
		t.Fatalf("unexpected plan metadata: %+v", resp.Teams[0].Plan)
	}
	if resp.Teams[0].Entitlements == nil || resp.Teams[0].Entitlements.MaxRetentionDays != 60 {
		t.Fatalf("expected cloud entitlements on team payload")
	}
}
