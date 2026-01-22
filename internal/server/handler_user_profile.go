package server

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
)

const gravatarBaseURL = "https://www.gravatar.com/avatar/"

func (s *Server) handleGetUserProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := s.store.GetUserByID(r.Context(), userID)
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

func (s *Server) handleGetUserAvatar() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := s.store.GetUserByID(r.Context(), userID)
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
