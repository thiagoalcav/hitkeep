package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"hitkeep/internal/auth"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

func setupAuthTestEnv(t *testing.T) (*handler, *database.Store) {
	t.Helper()

	store := database.NewStore(":memory:")
	if err := store.Connect(); err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	conf := &config.Config{
		PublicURL: "http://localhost:8080",
		JWTSecret: "test-secret",
	}

	ctx := &shared.Context{
		Store:  store,
		Config: conf,
	}

	return &handler{ctx: ctx}, store
}

func TestHandleCreateInitialUser(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	body, err := json.Marshal(map[string]string{
		"email":    "admin@example.com",
		"password": "password123",
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/initial-user", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handle := h.handleCreateInitialUser()
	handle.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["token"] == "" {
		t.Fatalf("expected token in response")
	}

	count, err := store.GetUserCount(context.Background())
	if err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 user, got %d", count)
	}

	// Second call should be blocked once setup is complete.
	req2 := httptest.NewRequest(http.MethodPost, "/api/initial-user", bytes.NewReader(body))
	w2 := httptest.NewRecorder()
	handle.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w2.Code)
	}
}

func TestHandleLogin(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "user@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if userID == uuid.Nil {
		t.Fatalf("expected valid user ID")
	}

	t.Run("invalid credentials", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"email":    "user@example.com",
			"password": "wrongpass",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.handleLogin().ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"email":    "user@example.com",
			"password": "password123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.handleLogin().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		cookies := w.Header().Values("Set-Cookie")
		found := false
		for _, cookie := range cookies {
			if bytes.Contains([]byte(cookie), []byte(auth.CookieName+"=")) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected auth cookie to be set")
		}
	})
}

func TestHandleChangePassword(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "user@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	t.Run("wrong current password", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"current_password": "wrongpass",
			"new_password":     "newpassword123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/user/password", bytes.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
		w := httptest.NewRecorder()
		h.handleChangePassword().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"current_password": "password123",
			"new_password":     "newpassword123",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/user/password", bytes.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), shared.UserIDKey, userID))
		w := httptest.NewRecorder()
		h.handleChangePassword().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		user, err := store.GetUserByID(context.Background(), userID)
		if err != nil {
			t.Fatalf("failed to load user: %v", err)
		}
		if user == nil {
			t.Fatalf("expected user")
		}
		match, err := verifyPassword("newpassword123", user.Password)
		if err != nil {
			t.Fatalf("failed to verify password: %v", err)
		}
		if !match {
			t.Fatalf("expected password to be updated")
		}
	})
}

func TestHandleLogout(t *testing.T) {
	h, store := setupAuthTestEnv(t)
	defer store.Close()

	hashed, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	userID, err := store.CreateUser(context.Background(), "user@example.com", hashed)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	token, err := store.CreateRememberMeToken(context.Background(), userID)
	if err != nil {
		t.Fatalf("failed to create remember me token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.RememberMeCookieName, Value: token})
	w := httptest.NewRecorder()
	h.handleLogout().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Token should be deleted after logout.
	validatedUser, err := store.ValidateRememberMeToken(context.Background(), token)
	if err != nil {
		t.Fatalf("failed to validate remember me token: %v", err)
	}
	if validatedUser != uuid.Nil {
		t.Fatalf("expected remember me token to be deleted")
	}
}
