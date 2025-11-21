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

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// withRateLimit applies the specific limiter to the handler
func (s *Server) withRateLimit(limiter *IPRateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If limiter is nil (shouldn't happen if setup correctly), just pass through
		if limiter == nil {
			next(w, r)
			return
		}

		ip := getRealIP(r)
		l := limiter.GetLimiter(ip)

		if !l.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
}
