package user

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestHandleUpdateSiteReportSubscriptionReturnsForbiddenWithoutTenantBackedAccess(t *testing.T) {
	h, store, userID := setupUserSecurityTestEnv(t)
	defer store.Close()

	ownerID, err := store.CreateUser(context.Background(), "other-owner@example.com", "hash")
	if err != nil {
		t.Fatalf("create other owner: %v", err)
	}
	team, err := store.CreateTenant(context.Background(), ownerID, "Other Team", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := store.SetActiveTenantID(context.Background(), ownerID, team.ID); err != nil {
		t.Fatalf("set owner active team: %v", err)
	}
	site, err := store.CreateSite(context.Background(), ownerID, "forbidden-site.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	if _, err := store.DB().ExecContext(context.Background(),
		"INSERT INTO site_members (id, site_id, user_id, role, added_at, added_by) VALUES (?, ?, ?, 'viewer', NOW(), ?)",
		uuid.New(), site.ID, userID, ownerID,
	); err != nil {
		t.Fatalf("insert stale site membership: %v", err)
	}

	body, _ := json.Marshal(map[string]bool{
		"daily":   true,
		"weekly":  false,
		"monthly": false,
	})
	req := withTestUser(httptest.NewRequest(http.MethodPut, "/api/user/report-subscriptions/sites/"+site.ID.String(), bytes.NewReader(body)), userID)
	req.SetPathValue("site_id", site.ID.String())
	w := httptest.NewRecorder()

	h.handleUpdateSiteReportSubscription().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var subCount int
	if err := store.DB().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM site_report_subscriptions WHERE user_id = ? AND site_id = ?",
		userID, site.ID,
	).Scan(&subCount); err != nil {
		t.Fatalf("count site report subscriptions: %v", err)
	}
	if subCount != 0 {
		t.Fatalf("expected no stored subscriptions for inaccessible site, got %d", subCount)
	}
}

func TestHandleUpdateSiteReportSubscriptionAllowsAccessibleSiteAcrossAnyCurrentTenant(t *testing.T) {
	h, store, userID := setupUserSecurityTestEnv(t)
	defer store.Close()

	team, err := store.CreateTenant(context.Background(), userID, "Reporting Team", "")
	if err != nil {
		t.Fatalf("create team: %v", err)
	}
	if err := store.SetActiveTenantID(context.Background(), userID, team.ID); err != nil {
		t.Fatalf("set active team: %v", err)
	}
	site, err := store.CreateSite(context.Background(), userID, "allowed-site.test")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	body, _ := json.Marshal(map[string]bool{
		"daily":   true,
		"weekly":  true,
		"monthly": false,
	})
	req := withTestUser(httptest.NewRequest(http.MethodPut, "/api/user/report-subscriptions/sites/"+site.ID.String(), bytes.NewReader(body)), userID)
	req.SetPathValue("site_id", site.ID.String())
	w := httptest.NewRecorder()

	h.handleUpdateSiteReportSubscription().ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	var subCount int
	if err := store.DB().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM site_report_subscriptions WHERE user_id = ? AND site_id = ?",
		userID, site.ID,
	).Scan(&subCount); err != nil {
		t.Fatalf("count site report subscriptions: %v", err)
	}
	if subCount != 3 {
		t.Fatalf("expected 3 frequency rows for accessible site, got %d", subCount)
	}
}
