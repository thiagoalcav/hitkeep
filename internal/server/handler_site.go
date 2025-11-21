package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"hitkeep/internal/api"

	"github.com/google/uuid"
)

func (s *Server) handleGetSites() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if s.store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		sites, err := s.store.GetSites(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to get sites", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(sites); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleCreateSite() http.HandlerFunc {
	type request struct {
		Domain string `json:"domain"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if s.store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		domain := strings.TrimSpace(req.Domain)
		if domain == "" {
			http.Error(w, "Domain is required", http.StatusBadRequest)
			return
		}

		site, err := s.store.CreateSite(r.Context(), userID, domain)
		if err != nil {
			slog.Error("Failed to create site", "error", err, "domain", domain)
			http.Error(w, "Failed to create site (domain might already exist)", http.StatusConflict)
			return
		}

		slog.Info("Site created", "id", site.ID, "domain", domain, "user_id", userID)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(site); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleGetHits() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if s.store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}
		siteIDStr := r.URL.Query().Get("site_id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site_id", http.StatusBadRequest)
			return
		}

		hits, err := s.store.GetHits(r.Context(), siteID, userID)
		if err != nil {
			slog.Error("Failed to get hits", "error", err, "site_id", siteID, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(hits); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleGetSiteStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if s.store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site_id", http.StatusBadRequest)
			return
		}

		// Default to last 30 days
		now := time.Now().UTC()
		end := now.AddDate(0, 0, 1) // Tomorrow (to cover full today)
		start := end.AddDate(0, 0, -30)

		// Allow overriding via query params (RFC3339)
		// Example: ?from=2023-10-01T00:00:00Z&to=2023-10-05T00:00:00Z
		q := r.URL.Query()
		if fromStr := q.Get("from"); fromStr != "" {
			if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
				start = parsed
			}
		}
		if toStr := q.Get("to"); toStr != "" {
			if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
				end = parsed
			}
		}

		params := api.AnalyticsParams{
			SiteID: siteID,
			UserID: userID,
			Start:  start,
			End:    end,
		}

		stats, err := s.store.GetSiteStats(r.Context(), params)
		if err != nil {
			slog.Error("Failed to get site stats", "error", err, "site_id", siteID)
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "Site not found", http.StatusNotFound)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

// helper to extract UserID from context (set by auth middleware)
func getUserIDFromContext(r *http.Request) uuid.UUID {
	val := r.Context().Value(UserIDKey)
	if val == nil {
		return uuid.Nil
	}
	id, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return id
}
