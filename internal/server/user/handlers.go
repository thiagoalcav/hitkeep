package user

import (
	"crypto/md5" //nolint:gosec // Gravatar requires MD5 hashes for avatar lookups.
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/server/shared"
)

type handler struct {
	ctx *shared.Context
}

func Register(mux *http.ServeMux, ctx *shared.Context) {
	h := &handler{ctx: ctx}
	mux.HandleFunc("GET /api/user/profile", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetUserProfile()))
	mux.HandleFunc("GET /api/user/avatar", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetUserAvatar()))
	mux.HandleFunc("GET /api/user/preferences", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetUserPreferences()))
	mux.HandleFunc("PUT /api/user/preferences", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleUpdateUserPreferences()))
	mux.HandleFunc("GET /api/user/security", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetUserSecurityStatus()))
	mux.HandleFunc("POST /api/user/security/totp/setup/start", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.AuthLimiter,
	}, h.handleStartTOTPSetup()))
	mux.HandleFunc("POST /api/user/security/totp/setup/verify", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.AuthLimiter,
	}, h.handleVerifyTOTPSetup()))
	mux.HandleFunc("POST /api/user/security/totp/disable", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.AuthLimiter,
	}, h.handleDisableTOTP()))
	mux.HandleFunc("POST /api/user/security/passkeys/register/start", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.AuthLimiter,
	}, h.handleStartPasskeyRegistration()))
	mux.HandleFunc("POST /api/user/security/passkeys/register/finish", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.AuthLimiter,
	}, h.handleFinishPasskeyRegistration()))
	mux.HandleFunc("DELETE /api/user/security/passkeys/{id}", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.AuthLimiter,
	}, h.handleDeleteUserPasskey()))
}

const gravatarBaseURL = "https://www.gravatar.com/avatar/"

func (h *handler) handleGetUserProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := h.ctx.Store.GetUserByID(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load user profile", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		resp := api.UserProfile{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: displayNameFromEmail(user.Email),
			AvatarURL:   "/api/user/avatar?s=96",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("Failed to encode user profile", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleGetUserAvatar() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := h.ctx.Store.GetUserByID(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load user for avatar", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		size := clampAvatarSize(r.URL.Query().Get("s"))
		avatarURL := gravatarURL(user.Email, size)

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, avatarURL, nil)
		if err != nil {
			http.Error(w, "Failed to request avatar", http.StatusInternalServerError)
			return
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			slog.Warn("Failed to fetch gravatar", "error", err, "user_id", userID)
			http.Error(w, "Avatar unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= http.StatusBadRequest {
			http.Error(w, "Avatar unavailable", resp.StatusCode)
			return
		}

		if contentType := resp.Header.Get("Content-Type"); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			slog.Warn("Failed to proxy gravatar response", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleGetUserPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		prefs, err := h.ctx.Store.GetUserPreferences(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load user preferences", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if prefs == nil {
			fallback := defaultPreferencesFromHeader(r.Header.Get("Accept-Language"))
			prefs = &fallback
		} else {
			normalized := normalizeLocaleTag(prefs.DefaultLocale)
			if normalized == "" {
				fallback := defaultPreferencesFromHeader(r.Header.Get("Accept-Language"))
				prefs = &fallback
			} else {
				prefs.DefaultLocale = normalized
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(prefs); err != nil {
			slog.Error("Failed to encode user preferences", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleUpdateUserPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var payload api.UserPreferences
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		prefs, err := validateUserPreferences(payload.DefaultLocale)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.UpsertUserPreferences(r.Context(), userID, prefs); err != nil {
			slog.Error("Failed to update user preferences", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(prefs); err != nil {
			slog.Error("Failed to encode user preferences", "error", err, "user_id", userID)
		}
	}
}

func gravatarURL(email string, size int) string {
	hash := gravatarHash(email)
	params := url.Values{
		"s": {strconv.Itoa(size)},
		"d": {"mp"},
	}
	return gravatarBaseURL + hash + "?" + params.Encode()
}

func gravatarHash(email string) string {
	normalized := strings.TrimSpace(strings.ToLower(email))
	sum := md5.Sum([]byte(normalized)) //nolint:gosec // Gravatar requires MD5 hashes for avatar lookups.
	return hex.EncodeToString(sum[:])
}

func clampAvatarSize(raw string) int {
	if raw == "" {
		return 96
	}
	size, err := strconv.Atoi(raw)
	if err != nil {
		return 96
	}
	if size < 32 {
		return 32
	}
	if size > 256 {
		return 256
	}
	return size
}

func displayNameFromEmail(email string) string {
	local := strings.SplitN(strings.TrimSpace(email), "@", 2)[0]
	if local == "" {
		return "User"
	}
	parts := strings.FieldsFunc(local, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	name := strings.TrimSpace(strings.Join(parts, " "))
	if name == "" {
		return "User"
	}
	return name
}
