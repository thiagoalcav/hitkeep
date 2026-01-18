package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

var domainRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

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

		domain := strings.ToLower(strings.TrimSpace(req.Domain))
		if domain == "" {
			http.Error(w, "Domain is required", http.StatusBadRequest)
			return
		}

		if strings.Contains(domain, "://") {
			http.Error(w, "Domain must not contain protocol (http:// or https://)", http.StatusBadRequest)
			return
		}

		if strings.HasPrefix(domain, "www.") {
			http.Error(w, "Domain must not start with 'www.' (we track subdomains automatically)", http.StatusBadRequest)
			return
		}

		if len(domain) > 253 || !domainRegex.MatchString(domain) {
			http.Error(w, "Invalid domain format (e.g. example.com)", http.StatusBadRequest)
			return
		}

		site, err := s.store.CreateSite(r.Context(), userID, domain)
		if err != nil {
			slog.Error("Failed to create site", "error", err, "domain", domain)
			http.Error(w, "Failed to create site (domain might already exist)", http.StatusConflict)
			return
		}

		if s.conf.DataRetentionDays > 0 {
			if err := s.store.UpdateSiteRetention(r.Context(), site.ID, userID, s.conf.DataRetentionDays); err != nil {
				slog.Warn("Failed to set default data retention policy", "site_id", site.ID, "error", err)
			}
		}

		slog.Info("Site created", "id", site.ID, "domain", domain, "user_id", userID)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(site); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (s *Server) handleUpdateSiteRetention() http.HandlerFunc {
	type request struct {
		Days int `json:"days"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site_id", http.StatusBadRequest)
			return
		}

		// Verify ownership
		site, err := s.store.GetSite(r.Context(), siteID, userID)
		if err != nil || site == nil {
			http.Error(w, "Site not found", http.StatusNotFound)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Days < 0 {
			http.Error(w, "Retention days must be non-negative", http.StatusBadRequest)
			return
		}

		if err := s.store.UpdateSiteRetention(r.Context(), siteID, userID, req.Days); err != nil {
			slog.Error("Failed to update site retention", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// handleGetSiteHits retrieves raw hits for a specific site.
// Path: GET /api/sites/{id}/hits
func (s *Server) handleGetSiteHits() http.HandlerFunc {
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

		q := r.URL.Query()

		now := time.Now().UTC()
		start := now.Add(-24 * time.Hour)
		end := now

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

		filterType, filterValue, err := parseFilterParams(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		limit := 10
		offset := 0
		if l := q.Get("limit"); l != "" {
			if val, err := strconv.Atoi(l); err == nil {
				limit = val
			}
		}
		if o := q.Get("offset"); o != "" {
			if val, err := strconv.Atoi(o); err == nil {
				offset = val
			}
		}
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 10
		}

		params := api.HitQueryParams{
			SiteID:      siteID,
			UserID:      userID,
			Start:       start,
			End:         end,
			Query:       q.Get("q"),
			SortField:   q.Get("sort"),
			SortOrder:   q.Get("order"), // asc/desc
			Limit:       limit,
			Offset:      offset,
			FilterType:  filterType,
			FilterValue: filterValue,
		}

		result, err := s.store.GetHits(r.Context(), params)
		if err != nil {
			slog.Error("Failed to get hits", "error", err, "site_id", siteID, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

// handleExportSiteHits streams filtered hits as CSV.
// Path: GET /api/sites/{id}/hits/export
func (s *Server) handleExportSiteHits() http.HandlerFunc {
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

		q := r.URL.Query()

		now := time.Now().UTC()
		start := now.Add(-24 * time.Hour)
		end := now

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

		filterType, filterValue, err := parseFilterParams(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		params := api.HitQueryParams{
			SiteID:      siteID,
			UserID:      userID,
			Start:       start,
			End:         end,
			Query:       q.Get("q"),
			FilterType:  filterType,
			FilterValue: filterValue,
		}

		filename := fmt.Sprintf("hits_%s_%d.csv", siteID, time.Now().Unix())
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)

		if err := s.store.ExportHitsCSV(r.Context(), params, w); err != nil {
			slog.Error("Failed to export hits", "error", err, "site_id", siteID, "user_id", userID)
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

		filterType, filterValue, err := parseFilterParams(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		params := api.AnalyticsParams{
			SiteID:      siteID,
			UserID:      userID,
			Start:       start,
			End:         end,
			FilterType:  filterType,
			FilterValue: filterValue,
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
	// First check PermissionContext (new RBAC)
	if val := r.Context().Value(PermissionKey); val != nil {
		if perms, ok := val.(PermissionContext); ok {
			return perms.UserID
		}
	}

	// Fallback to legacy UserIDKey
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

func parseFilterParams(q url.Values) (string, string, error) {
	filterType := strings.ToLower(strings.TrimSpace(q.Get("filter_type")))
	filterValue := strings.TrimSpace(q.Get("filter_value"))

	if filterType == "" && filterValue == "" {
		return "", "", nil
	}
	if filterType == "" || filterValue == "" {
		return "", "", fmt.Errorf("filter_type and filter_value are required together")
	}

	switch filterType {
	case "path", "referrer", "device":
		return filterType, filterValue, nil
	default:
		return "", "", fmt.Errorf("invalid filter_type")
	}
}

// handleGetFavicon proxies the favicon request to DuckDuckGo to avoid CORS and privacy leaks.
// GET /api/favicon/{domain}
func (s *Server) handleGetFavicon() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := strings.TrimSpace(r.PathValue("domain"))
		if domain == "" || strings.Contains(domain, "/") {
			http.Error(w, "Invalid domain", http.StatusBadRequest)
			return
		}

		// Use DuckDuckGo's favicon service
		ddgURL := fmt.Sprintf("https://icons.duckduckgo.com/ip3/%s.ico", domain)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(ddgURL)
		if err != nil {
			slog.Warn("Failed to fetch favicon upstream", "domain", domain, "error", err)
			http.Error(w, "Upstream error", http.StatusBadGateway)
			return
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				slog.Warn("Failed to close response body", "error", err)
			}
		}()

		// Cache for 24 hours in the browser to reduce load
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))

		if _, err := io.Copy(w, resp.Body); err != nil {
			slog.Warn("Failed to write favicon response", "error", err)
		}
	}
}
