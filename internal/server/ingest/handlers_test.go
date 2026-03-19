package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nsqio/go-nsq"

	"hitkeep/internal/blocking"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

func TestIngestCORSPreflightAddsCachingHeaders(t *testing.T) {
	handler := newIngestCORS().Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/ingest", nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("expected echoed origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "86400" {
		t.Fatalf("expected max age header, got %q", got)
	}
	if got := rec.Header().Values("Vary"); len(got) == 0 {
		t.Fatal("expected vary header to be set")
	}
}

func TestIngestCORSActualRequestEchoesOrigin(t *testing.T) {
	handler := newIngestCORS().Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodPost, "/ingest", nil)
	req.Header.Set("Origin", "https://app.example")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("expected echoed origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected credentials header, got %q", got)
	}
}

func TestHandleIngestLeaderDropsBlockedReferrerBeforePublish(t *testing.T) {
	h, cleanup := setupIngestHandler(t, func(ctx *shared.Context) {
		filter := mustNewTestSpamFilter(t, blocking.SpamFeedData{
			ReferrerHostDenylist: []string{"buttons-for-website.example"},
		})
		ctx.SpamFilter = filter
	})
	defer cleanup()

	req := newIngestRequest(t, "https://example.com", "198.51.100.22:1234", map[string]any{
		"path":       "/docs",
		"referrer":   "https://www.buttons-for-website.example/landing",
		"session_id": uuid.New(),
		"page_id":    uuid.New(),
	})

	rec := httptest.NewRecorder()
	h.handleIngestLeader(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func TestHandleIngestLeaderDropsBlockedNetworkBeforePublish(t *testing.T) {
	h, cleanup := setupIngestHandler(t, func(ctx *shared.Context) {
		filter := mustNewTestSpamFilter(t, blocking.SpamFeedData{
			NetworkDenylist: []string{"203.0.113.0/24"},
		})
		ctx.SpamFilter = filter
	})
	defer cleanup()

	req := newIngestRequest(t, "https://example.com", "203.0.113.42:1234", map[string]any{
		"path":       "/pricing",
		"session_id": uuid.New(),
		"page_id":    uuid.New(),
	})

	rec := httptest.NewRecorder()
	h.handleIngestLeader(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
}

func TestHandleIngestLeaderAllowsSameSiteReferrerToContinueToPublish(t *testing.T) {
	producer, err := newTestProducer()
	if err != nil {
		t.Fatalf("create nsq producer: %v", err)
	}
	defer producer.Stop()

	h, cleanup := setupIngestHandler(t, func(ctx *shared.Context) {
		filter := mustNewTestSpamFilter(t, blocking.SpamFeedData{
			ReferrerHostDenylist: []string{"example.com"},
		})
		ctx.SpamFilter = filter
		ctx.Producer = producer
	})
	defer cleanup()

	req := newIngestRequest(t, "https://www.example.com", "198.51.100.33:1234", map[string]any{
		"path":       "/docs/start",
		"referrer":   "https://www.example.com/blog/post",
		"session_id": uuid.New(),
		"page_id":    uuid.New(),
	})

	rec := httptest.NewRecorder()
	h.handleIngestLeader(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected publish attempt to fail with status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func setupIngestHandler(t *testing.T, mutateCtx func(*shared.Context)) (*handler, func()) {
	t.Helper()

	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	userID, err := store.CreateUser(context.Background(), "ingest-test@example.com", "hashed_secret")
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	if _, err := store.CreateSite(context.Background(), userID, "example.com"); err != nil {
		t.Fatalf("create test site: %v", err)
	}

	ctx := &shared.Context{
		Store:  store,
		Config: &config.Config{},
	}
	if mutateCtx != nil {
		mutateCtx(ctx)
	}

	return &handler{ctx: ctx}, func() {
		store.Close()
	}
}

func newIngestRequest(t *testing.T, origin, remoteAddr string, payload map[string]any) *http.Request {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	req.Header.Set("Origin", origin)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	return req
}

func newTestProducer() (*nsq.Producer, error) {
	cfg := nsq.NewConfig()
	cfg.DialTimeout = 100 * time.Millisecond
	cfg.ReadTimeout = 35 * time.Second
	cfg.WriteTimeout = 35 * time.Second
	return nsq.NewProducer("127.0.0.1:1", cfg)
}

func mustNewTestSpamFilter(t *testing.T, data blocking.SpamFeedData) *blocking.SpamFilter {
	t.Helper()

	path := t.TempDir() + "/spam-filter.json"
	if err := blocking.SaveSpamFeedData(path, data); err != nil {
		t.Fatalf("save spam filter data: %v", err)
	}

	filter := blocking.NewSpamFilter(path)
	if err := filter.RefreshFromDisk(); err != nil {
		t.Fatalf("refresh spam filter from disk: %v", err)
	}

	return filter
}
