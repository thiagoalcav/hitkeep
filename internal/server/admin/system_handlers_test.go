package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/auth"
	"hitkeep/internal/blocking"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/mailer"
	"hitkeep/internal/server/shared"
)

func setupSystemTestEnv(t *testing.T) (*handler, *database.Store, *database.TenantStoreManager, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()

	basePath := t.TempDir()
	store := database.NewStore(filepath.Join(basePath, "shared.db"))
	if err := store.Connect(); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ownerUserID, err := store.CreateUser(context.Background(), "owner@example.com", "hash")
	if err != nil {
		t.Fatalf("create owner user: %v", err)
	}
	adminUserID, err := store.CreateUser(context.Background(), "admin@example.com", "hash")
	if err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	regularUserID, err := store.CreateUser(context.Background(), "user@example.com", "hash")
	if err != nil {
		t.Fatalf("create regular user: %v", err)
	}

	if err := store.UpdateInstanceRole(context.Background(), ownerUserID, auth.InstanceOwner, ownerUserID); err != nil {
		t.Fatalf("promote owner: %v", err)
	}
	if err := store.UpdateInstanceRole(context.Background(), adminUserID, auth.InstanceAdmin, ownerUserID); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	tenantStores := database.NewTenantStoreManager(store, basePath)
	t.Cleanup(func() { _ = tenantStores.Close() })

	systemCounters := &database.SystemCounter{}
	backupStatus := &database.BackupStatusTracker{}
	backupStatus.SetConfig(false, "", 0, 0)
	mailTestTracker := &database.MailTestTracker{}

	ctx := &shared.Context{
		Store:           store,
		TenantStores:    tenantStores,
		SystemCounters:  systemCounters,
		BackupStatus:    backupStatus,
		MailTestTracker: mailTestTracker,
		Config: &config.Config{
			PublicURL: "http://localhost:8080",
			JWTSecret: "test-secret",
			DBPath:    filepath.Join(basePath, "shared.db"),
			DataPath:  basePath,
		},
		StartedAt: time.Now().UTC(),
	}

	return &handler{ctx: ctx}, store, tenantStores, ownerUserID, adminUserID, regularUserID
}

func TestHandleGetSystem(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)
	h.ctx.Config.MCPEnabled = true
	h.ctx.Config.MCPPath = "/agent"
	h.ctx.Config.MCPDocsEnabled = true
	h.ctx.Config.MCPDocsURL = "https://docs.example.com"
	h.ctx.Config.BackupPath = "s3://hitkeep/backups"
	h.ctx.Config.SpamFilterAutoUpdate = true
	h.ctx.Config.SpamFilterUpdateIntervalMin = 60
	h.ctx.Config.MailDriver = "smtp"
	h.ctx.Config.CloudHosted = true
	h.ctx.Config.CloudPlanName = "Pro"
	h.ctx.Config.CloudSignupEnabled = true
	h.ctx.Config.StripeSecretKey = "sk_test_123"
	h.ctx.Mailer = mailer.NewWithDriver(&adminTestMailDriver{}, h.ctx.Config)

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleGetSystem().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var info api.SystemInfo
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if info.PublicURL != "http://localhost:8080" {
		t.Fatalf("expected public_url 'http://localhost:8080', got %q", info.PublicURL)
	}
	if _, ok := info.ConfigFlags["user_count"]; ok {
		t.Fatal("did not expect user_count to be reported as a config flag")
	}
	if _, ok := info.ConfigFlags["site_count"]; ok {
		t.Fatal("did not expect site_count to be reported as a config flag")
	}

	features := featureStatusByKey(info.EnabledFeatures)
	for _, key := range []string{"mcp", "mcp_docs", "automatic_backups", "spam_auto_update", "mail_delivery", "managed_cloud", "cloud_signup", "billing"} {
		feature, ok := features[key]
		if !ok {
			t.Fatalf("expected feature %q to be reported", key)
		}
		if !feature.Enabled {
			t.Fatalf("expected feature %q to be enabled", key)
		}
	}
	if features["automatic_backups"].Detail != "s3" {
		t.Fatalf("expected S3 backup detail, got %q", features["automatic_backups"].Detail)
	}
	if features["spam_auto_update"].Detail != "1h" {
		t.Fatalf("expected spam update interval detail 1h, got %q", features["spam_auto_update"].Detail)
	}
	if features["managed_cloud"].Detail != "Pro" {
		t.Fatalf("expected managed cloud detail Pro, got %q", features["managed_cloud"].Detail)
	}
}

func featureStatusByKey(features []api.SystemFeatureStatus) map[string]api.SystemFeatureStatus {
	byKey := make(map[string]api.SystemFeatureStatus, len(features))
	for _, feature := range features {
		byKey[feature.Key] = feature
	}
	return byKey
}

func TestHandleGetHealth(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/health", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleGetHealth().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var health api.SystemHealth
	if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if health.Database != "ok" {
		t.Fatalf("expected database 'ok', got %q", health.Database)
	}
}

func TestHandleGetStorage(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/storage", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleGetStorage().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var storage api.SystemStorage
	if err := json.NewDecoder(w.Body).Decode(&storage); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if storage.SharedDBPath == "" {
		t.Fatal("expected non-empty shared_db_path")
	}
	if storage.SharedDBBytes <= 0 {
		t.Fatalf("expected positive shared_db_bytes, got %d", storage.SharedDBBytes)
	}
}

func TestHandleGetIngestStats(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/ingest", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleGetIngestStats().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var stats api.SystemIngestStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Should have zero values but not error
	if stats.RecentHits < 0 {
		t.Fatalf("unexpected negative hits: %d", stats.RecentHits)
	}
}

func TestHandleGetBackups(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/backups", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleGetBackups().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status api.SystemBackupStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Backups disabled by default
	if status.Enabled {
		t.Fatal("expected backups disabled by default")
	}
}

func TestHandleGetSpamFilter(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)
	generatedAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	spamPath := filepath.Join(t.TempDir(), "spam-filter.json")
	if err := blocking.SaveSpamFeedData(spamPath, blocking.SpamFeedData{
		GeneratedAt:          generatedAt,
		ReferrerHostDenylist: []string{"spam.example"},
		NetworkDenylist:      []string{"203.0.113.0/24"},
	}); err != nil {
		t.Fatalf("save spam feed data: %v", err)
	}
	filter := blocking.NewSpamFilter(spamPath)
	if err := filter.RefreshFromDisk(); err != nil {
		t.Fatalf("refresh spam filter: %v", err)
	}
	h.ctx.SpamFilter = filter
	h.ctx.Config.SpamFilterPath = spamPath

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/spam-filter", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleGetSpamFilter().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status api.SystemSpamStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.DBPath == "" {
		t.Fatal("expected non-empty db_path")
	}
	if status.RuleCount != 2 {
		t.Fatalf("expected rule_count 2, got %d", status.RuleCount)
	}
	if status.LastRefresh == nil || !status.LastRefresh.Equal(generatedAt) {
		t.Fatalf("expected last_refresh %s, got %v", generatedAt.Format(time.RFC3339), status.LastRefresh)
	}
}

func TestHandleGetCaches(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/caches", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleGetCaches().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status api.SystemCacheStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.PermissionsCache.MaxSize != 8192 {
		t.Fatalf("expected max size 8192, got %d", status.PermissionsCache.MaxSize)
	}
}

func TestHandleGetMail(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/mail", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleGetMail().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status api.SystemMailStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Configured {
		t.Fatal("expected mail not configured in test")
	}
}

func TestHandleSpamRefreshAction(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	// Without spam filter, should return 503
	req := withAdminTestUser(httptest.NewRequest(http.MethodPost, "/api/admin/system/spam-filter/refresh", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleRefreshSpamFilter().ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleMailTestAction(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	// Without mailer, should return 503
	req := withAdminTestUser(httptest.NewRequest(http.MethodPost, "/api/admin/system/mail/test", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleTestMail().ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleListAudit(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/audit", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleListAudit().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp api.InstanceAuditListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Empty audit log is valid
	if resp.Total != 0 {
		t.Fatalf("expected empty audit, got %d entries", resp.Total)
	}
}

func TestHandleExportAuditJSON(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system", nil), ownerID)
	h.appendAudit(req, "export.test", "system", "", "", "success", "Export test")

	exportReq := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/audit/export", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleExportAudit().ServeHTTP(w, exportReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}

	var entries []api.InstanceAuditEntry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one exported entry")
	}
}

func TestHandleAuditAppendsAndListsEntries(t *testing.T) {
	h, store, _, ownerID, _, _ := setupSystemTestEnv(t)
	ctx := context.Background()

	// Append an audit entry via helper
	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system", nil), ownerID)
	h.appendAudit(req, "test.action", "test_type", "test-1", "Test Label", "success", "Test details")

	// List audit entries
	listReq := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/audit", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleListAudit().ServeHTTP(w, listReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp api.InstanceAuditListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total < 1 {
		t.Fatalf("expected at least 1 audit entry, got %d", resp.Total)
	}

	found := false
	for _, entry := range resp.Entries {
		if entry.Action == "test.action" {
			found = true
			if entry.TargetType != "test_type" {
				t.Fatalf("expected target_type 'test_type', got %q", entry.TargetType)
			}
			if entry.Outcome != "success" {
				t.Fatalf("expected outcome 'success', got %q", entry.Outcome)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected test audit entry not found in list")
	}

	_ = ctx
	_ = store
}

func TestHandleExportAuditCSV(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system", nil), ownerID)
	h.appendAudit(req, "csv.test", "system", "", "", "success", "CSV export test")

	exportReq := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/audit/export?format=csv", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleExportAudit().ServeHTTP(w, exportReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Fatalf("expected CSV content type, got %q", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "action") {
		t.Fatal("expected CSV header row with 'action'")
	}
	if !strings.Contains(body, "csv.test") {
		t.Fatal("expected CSV data row with 'csv.test'")
	}
}

func TestHandleMailTestSuccess(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	drv := &adminTestMailDriver{}
	h.ctx.Mailer = mailer.NewWithDriver(drv, h.ctx.Config)

	h.ctx.Config.MailHost = "localhost"
	h.ctx.Config.MailPort = 25

	body := strings.NewReader(`{}`)
	req := withAdminTestUser(httptest.NewRequest(http.MethodPost, "/api/admin/system/mail/test", body), ownerID)
	w := httptest.NewRecorder()
	h.handleTestMail().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if drv.subject == "" {
		t.Fatal("expected mail driver to have sent an email")
	}
	if !strings.Contains(drv.subject, "HitKeep System Test Email") {
		t.Fatalf("expected test email subject, got %q", drv.subject)
	}
	if len(drv.recipients) != 1 || drv.recipients[0] != "owner@example.com" {
		t.Fatalf("expected default recipient owner@example.com, got %#v", drv.recipients)
	}
}

func TestHandleMailTestUsesRequestedRecipient(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	drv := &adminTestMailDriver{}
	h.ctx.Mailer = mailer.NewWithDriver(drv, h.ctx.Config)

	body := strings.NewReader(`{"email":"ops@example.com"}`)
	req := withAdminTestUser(httptest.NewRequest(http.MethodPost, "/api/admin/system/mail/test", body), ownerID)
	w := httptest.NewRecorder()
	h.handleTestMail().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(drv.recipients) != 1 || drv.recipients[0] != "ops@example.com" {
		t.Fatalf("expected requested recipient ops@example.com, got %#v", drv.recipients)
	}
}

func TestHandleMailTestRejectsInvalidRecipient(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	drv := &adminTestMailDriver{}
	h.ctx.Mailer = mailer.NewWithDriver(drv, h.ctx.Config)

	body := strings.NewReader(`{"email":"not-an-email"}`)
	req := withAdminTestUser(httptest.NewRequest(http.MethodPost, "/api/admin/system/mail/test", body), ownerID)
	w := httptest.NewRecorder()
	h.handleTestMail().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	if len(drv.recipients) != 0 {
		t.Fatalf("expected no outbound email, got %#v", drv.recipients)
	}
}

// failMailDriver returns an error on Send to simulate a broken mail transport.
type failMailDriver struct{}

func (d *failMailDriver) Send(_ []string, _, _, _ string) error {
	return errors.New("simulated mail transport failure")
}
func (d *failMailDriver) Close() error { return nil }

func TestHandleMailTestFailure(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	h.ctx.Mailer = mailer.NewWithDriver(&failMailDriver{}, h.ctx.Config)

	body := strings.NewReader(`{}`)
	req := withAdminTestUser(httptest.NewRequest(http.MethodPost, "/api/admin/system/mail/test", body), ownerID)
	w := httptest.NewRecorder()
	h.handleTestMail().ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSystemMailRedaction(t *testing.T) {
	h, _, _, ownerID, _, _ := setupSystemTestEnv(t)

	h.ctx.Config.MailUsername = "admin@example.com"
	h.ctx.Config.MailPassword = "super-secret-password"
	h.ctx.Config.MailHost = "smtp.example.com"
	h.ctx.Config.MailPort = 587

	req := withAdminTestUser(httptest.NewRequest(http.MethodGet, "/api/admin/system/mail", nil), ownerID)
	w := httptest.NewRecorder()
	h.handleGetMail().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status api.SystemMailStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !status.Configured {
		t.Fatal("expected mail configured")
	}
	if status.Host != "smtp.example.com" {
		t.Fatalf("expected host 'smtp.example.com', got %q", status.Host)
	}
	if !strings.Contains(status.Username, "****") {
		t.Fatalf("expected username to be redacted, got %q", status.Username)
	}
	if !status.PasswordSet {
		t.Fatal("expected password_set true")
	}
	// Full password should not appear in response
	if strings.Contains(status.Username, "super-secret") {
		t.Fatal("username should not leak full value")
	}
}
