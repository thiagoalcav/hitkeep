package goals

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

func setupTenantGoalsTestEnv(t *testing.T) (*handler, *database.Store, *database.Store, uuid.UUID) {
	t.Helper()

	ctx := context.Background()
	basePath := t.TempDir()
	store := database.NewStore(filepath.Join(basePath, "hitkeep.db"))
	if err := store.Connect(); err != nil {
		t.Fatalf("connect shared store: %v", err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate shared store: %v", err)
	}

	tenantMgr := database.NewTenantStoreManager(store, basePath)

	t.Cleanup(func() {
		_ = tenantMgr.Close()
		_ = store.Close()
	})

	userID, err := store.CreateUser(ctx, "team-owner@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	team, err := store.CreateTenant(ctx, userID, "Acme Analytics", "")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if err := store.SetActiveTenantID(ctx, userID, team.ID); err != nil {
		t.Fatalf("set active tenant: %v", err)
	}

	site, err := store.CreateSite(ctx, userID, "acme-analytics.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	tenantStore, err := tenantMgr.ForTenant(ctx, team.ID)
	if err != nil {
		t.Fatalf("open tenant store: %v", err)
	}

	h := &handler{
		ctx: &shared.Context{
			Store:        store,
			TenantStores: tenantMgr,
			Config:       &config.Config{},
		},
	}

	return h, store, tenantStore, site.ID
}

func TestHandleGoalCRUDUsesTenantAnalyticsStore(t *testing.T) {
	h, sharedStore, tenantStore, siteID := setupTenantGoalsTestEnv(t)
	ctx := context.Background()

	body, err := json.Marshal(api.Goal{
		Name:  "Signup",
		Type:  "event",
		Value: "signup_completed",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/sites/"+siteID.String()+"/goals", bytes.NewReader(body))
	createReq.SetPathValue("id", siteID.String())
	createResp := httptest.NewRecorder()
	h.handleCreateGoal().ServeHTTP(createResp, createReq)

	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, createResp.Code, createResp.Body.String())
	}

	sharedGoals, err := sharedStore.GetGoals(ctx, siteID)
	if err != nil {
		t.Fatalf("shared GetGoals: %v", err)
	}
	if len(sharedGoals) != 1 {
		t.Fatalf("expected 1 legacy goal in shared store, got %d", len(sharedGoals))
	}

	tenantGoals, err := tenantStore.GetGoals(ctx, siteID)
	if err != nil {
		t.Fatalf("tenant GetGoals: %v", err)
	}
	if len(tenantGoals) != 1 {
		t.Fatalf("expected 1 goal in tenant store, got %d", len(tenantGoals))
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/sites/"+siteID.String()+"/goals", nil)
	getReq.SetPathValue("id", siteID.String())
	getResp := httptest.NewRecorder()
	h.handleGetGoals().ServeHTTP(getResp, getReq)

	if getResp.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d: %s", http.StatusOK, getResp.Code, getResp.Body.String())
	}

	var gotGoals []api.Goal
	if err := json.NewDecoder(getResp.Body).Decode(&gotGoals); err != nil {
		t.Fatalf("decode goals response: %v", err)
	}
	if len(gotGoals) != 1 || gotGoals[0].Name != "Signup" {
		t.Fatalf("expected tenant goal in response, got %+v", gotGoals)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/sites/"+siteID.String()+"/goals/"+tenantGoals[0].ID.String(), nil)
	deleteReq.SetPathValue("id", siteID.String())
	deleteReq.SetPathValue("goalID", tenantGoals[0].ID.String())
	deleteResp := httptest.NewRecorder()
	h.handleDeleteGoal().ServeHTTP(deleteResp, deleteReq)

	if deleteResp.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d: %s", http.StatusOK, deleteResp.Code, deleteResp.Body.String())
	}

	tenantGoals, err = tenantStore.GetGoals(ctx, siteID)
	if err != nil {
		t.Fatalf("tenant GetGoals after delete: %v", err)
	}
	if len(tenantGoals) != 0 {
		t.Fatalf("expected tenant goal to be deleted, got %d remaining", len(tenantGoals))
	}

	sharedGoals, err = sharedStore.GetGoals(ctx, siteID)
	if err != nil {
		t.Fatalf("shared GetGoals after delete: %v", err)
	}
	if len(sharedGoals) != 0 {
		t.Fatalf("expected legacy shared goal to be deleted, got %d remaining", len(sharedGoals))
	}
}

func TestHandleGetFunnelsUsesTenantAnalyticsStore(t *testing.T) {
	h, sharedStore, tenantStore, siteID := setupTenantGoalsTestEnv(t)
	ctx := context.Background()

	err := tenantStore.CreateFunnel(ctx, &api.Funnel{
		SiteID: siteID,
		Name:   "Checkout Funnel",
		Steps: []api.FunnelStep{
			{Type: "path", Value: "/pricing"},
			{Type: "event", Value: "signup_completed"},
		},
	})
	if err != nil {
		t.Fatalf("create funnel in tenant store: %v", err)
	}

	sharedFunnels, err := sharedStore.GetFunnels(ctx, siteID)
	if err != nil {
		t.Fatalf("shared GetFunnels: %v", err)
	}
	if len(sharedFunnels) != 0 {
		t.Fatalf("expected no funnels in shared store, got %d", len(sharedFunnels))
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/sites/"+siteID.String()+"/funnels", nil)
	getReq.SetPathValue("id", siteID.String())
	getResp := httptest.NewRecorder()
	h.handleGetFunnels().ServeHTTP(getResp, getReq)

	if getResp.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d: %s", http.StatusOK, getResp.Code, getResp.Body.String())
	}

	var gotFunnels []api.Funnel
	if err := json.NewDecoder(getResp.Body).Decode(&gotFunnels); err != nil {
		t.Fatalf("decode funnels response: %v", err)
	}
	if len(gotFunnels) != 1 || gotFunnels[0].Name != "Checkout Funnel" {
		t.Fatalf("expected tenant funnel in response, got %+v", gotFunnels)
	}
}
