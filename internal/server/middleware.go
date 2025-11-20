package server

import (
	"context"
	"log/slog"
	"net/http"

	"hitkeep/internal/auth"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// requireAuth creates a middleware that checks for a valid JWT in the HTTP-only cookie.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.CookieName)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID, err := auth.ValidateToken(cookie.Value, s.conf.JWTSecret, s.conf.PublicURL)
		if err != nil {
			slog.Warn("Invalid auth token", "error", err, "remote_addr", r.RemoteAddr)
			auth.SetTokenCookie(w, "", false)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 3. Add UserID to context
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}
