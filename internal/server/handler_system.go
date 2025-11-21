package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// handleHealthz checks the health of the node.
func (s *Server) handleHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.store != nil {
			if err := s.store.DB().Ping(); err != nil {
				slog.Error("Healthcheck failed: database unreachable", "error", err)
				http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			slog.Error("Failed to write healthcheck response", "error", err)
		}
	}
}

func (s *Server) handleGetStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		userCount, err := s.store.GetUserCount(r.Context())
		if err != nil {
			slog.Error("Failed to get user count", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		response := map[string]any{
			"needs_setup": userCount == 0,
			"version":     s.conf.Version,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}
