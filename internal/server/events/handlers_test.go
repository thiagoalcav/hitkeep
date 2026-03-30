package events

import (
	"bytes"
	"context"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/auth"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/exportfmt"
	"hitkeep/internal/server/shared"
)

func setupEventHandlerTestEnv(t *testing.T) (*database.Store, *shared.Context, uuid.UUID, uuid.UUID, string) {
	t.Helper()

	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	userID, err := store.CreateUser(context.Background(), "events@example.com", "hashed")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	site, err := store.CreateSite(context.Background(), userID, "events.example.com")
	if err != nil {
		t.Fatalf("CreateSite: %v", err)
	}
	_, token, err := store.CreateAPIClient(context.Background(), userID, "Events", "test", auth.InstanceUser, map[uuid.UUID]auth.SiteRole{
		site.ID: auth.SiteOwner,
	}, nil)
	if err != nil {
		t.Fatalf("CreateAPIClient: %v", err)
	}

	ctx := &shared.Context{
		Store:  store,
		Config: &config.Config{},
	}

	return store, ctx, userID, site.ID, token
}

func TestHandleExportAIChatbotsCSV(t *testing.T) {
	store, ctx, userID, siteID, _ := setupEventHandlerTestEnv(t)

	base := time.Now().UTC()
	sessionID := uuid.New()
	for _, event := range []*api.Event{
		{
			SiteID:    siteID,
			SessionID: sessionID,
			Name:      "assistant.chat_started",
			Properties: map[string]any{
				"provider": "OpenAI",
				"bot_id":   "support-bot",
				"surface":  "pricing",
				"model":    "gpt-4.1-mini",
			},
			Timestamp: base.Add(-2 * time.Hour),
		},
		{
			SiteID:    siteID,
			SessionID: sessionID,
			Name:      "assistant.message_sent",
			Properties: map[string]any{
				"provider": "OpenAI",
				"bot_id":   "support-bot",
				"surface":  "pricing",
				"model":    "gpt-4.1-mini",
				"intent":   "pricing",
			},
			Timestamp: base.Add(-90 * time.Minute),
		},
	} {
		if err := store.CreateEvent(context.Background(), event); err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
	}

	h := &handler{ctx: ctx}
	req := httptest.NewRequest(http.MethodGet, "/api/sites/"+siteID.String()+"/ai-chatbots/export?from="+base.Add(-24*time.Hour).Format(time.RFC3339)+"&to="+base.Format(time.RFC3339)+"&scope_key=provider&scope_value=OpenAI&format=csv", nil)
	req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
	req.SetPathValue("id", siteID.String())
	rec := httptest.NewRecorder()
	h.handleExportAIChatbots().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d body=%s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != exportfmt.ContentType(exportfmt.FormatCSV) {
		t.Fatalf("expected csv content type, got %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment; filename=ai-chatbots_") {
		t.Fatalf("expected content-disposition attachment, got %q", got)
	}

	rows, err := csv.NewReader(bytes.NewReader(rec.Body.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected header plus two rows, got %d", len(rows))
	}
}

func TestHandleExportAIChatbotsRejectsInvalidScopeKey(t *testing.T) {
	_, ctx, userID, siteID, _ := setupEventHandlerTestEnv(t)

	h := &handler{ctx: ctx}
	req := httptest.NewRequest(http.MethodGet, "/api/sites/"+siteID.String()+"/ai-chatbots/export?scope_key=unknown&scope_value=test", nil)
	req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
	req.SetPathValue("id", siteID.String())
	rec := httptest.NewRecorder()
	h.handleExportAIChatbots().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
