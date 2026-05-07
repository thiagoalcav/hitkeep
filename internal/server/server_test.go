package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/entitlements"
)

func TestServerMountsMCPRouteWhenEnabled(t *testing.T) {
	conf := testServerConfig(t)
	conf.MCPEnabled = true
	store := testServerStore(t)
	defer store.Close()

	srv := New(conf, testPublicFS(), store, nil, entitlements.NewProvider(conf), nil, nil, nil)
	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Host = "localhost:8080"
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected MCP route to require bearer auth with 401, got %d", rec.Code)
	}
}

func TestServerNormalizesRootMCPPath(t *testing.T) {
	conf := testServerConfig(t)
	conf.MCPEnabled = true
	conf.MCPPath = "/"
	store := testServerStore(t)
	defer store.Close()

	srv := New(conf, testPublicFS(), store, nil, entitlements.NewProvider(conf), nil, nil, nil)
	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Host = "localhost:8080"
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected normalized MCP route to require bearer auth with 401, got %d", rec.Code)
	}
	if conf.MCPPath != "/mcp" {
		t.Fatalf("expected server to normalize root MCPPath to /mcp, got %q", conf.MCPPath)
	}
}

func TestServerDoesNotMountMCPRouteWhenDisabled(t *testing.T) {
	conf := testServerConfig(t)
	store := testServerStore(t)
	defer store.Close()

	srv := New(conf, testPublicFS(), store, nil, entitlements.NewProvider(conf), nil, nil, nil)
	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "hitkeep test shell") {
		t.Fatalf("expected disabled MCP path to fall through to SPA, got %d %q", rec.Code, rec.Body.String())
	}
}

func TestServerBackupStatusReflectsConfig(t *testing.T) {
	conf := testServerConfig(t)
	conf.BackupPath = "s3://hitkeep-backups/backup"
	conf.BackupIntervalMinutes = 60
	conf.BackupRetentionCount = 24
	store := testServerStore(t)
	defer store.Close()

	srv := New(conf, testPublicFS(), store, nil, entitlements.NewProvider(conf), nil, nil, nil)
	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	tracker := srv.BackupStatus()
	if tracker == nil {
		t.Fatal("expected backup status tracker")
	}
	status := tracker.Status()
	if !status.Enabled {
		t.Fatal("expected backup status to be enabled")
	}
	if status.ConfigPath != conf.BackupPath {
		t.Fatalf("expected backup path %q, got %q", conf.BackupPath, status.ConfigPath)
	}
	if status.IntervalMin != 60 {
		t.Fatalf("expected interval 60, got %d", status.IntervalMin)
	}
	if status.Retention != 24 {
		t.Fatalf("expected retention 24, got %d", status.Retention)
	}
}

func testServerConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		ApiBurst:         1000,
		ApiRateLimit:     1000,
		AuthBurst:        1000,
		AuthRateLimit:    1000,
		DataPath:         t.TempDir(),
		HTTPAddr:         "127.0.0.1:0",
		IngestBurst:      1000,
		IngestRateLimit:  1000,
		MCPPath:          "/mcp",
		PublicURL:        "http://localhost:8080",
		SpamFilterPath:   filepath.Join(t.TempDir(), "spam-filter.json"),
		Version:          "test",
		WebhookBurst:     1000,
		WebhookRateLimit: 1000,
	}
}

func testServerStore(t *testing.T) *database.Store {
	t.Helper()
	store := database.NewStore(filepath.Join(t.TempDir(), "hitkeep.db"))
	if err := store.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		store.Close()
		t.Fatalf("Migrate: %v", err)
	}
	return store
}

func testPublicFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html": {
			Data: []byte("<html><body>hitkeep test shell</body></html>"),
		},
	}
}
