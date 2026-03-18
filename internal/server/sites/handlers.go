package sites

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	authcore "hitkeep/internal/auth"
	"hitkeep/internal/blocking"
	"hitkeep/internal/database"
	"hitkeep/internal/exportfmt"
	"hitkeep/internal/server/shared"
)

type handler struct {
	ctx *shared.Context
}

var faviconProxyTransport = newFaviconProxyTransport(5 * time.Second)

func Register(mux *http.ServeMux, ctx *shared.Context) {
	h := &handler{ctx: ctx}
	mux.HandleFunc("GET /api/sites", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		AllowAPIKey: true,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetSites()))
	mux.HandleFunc("POST /api/sites", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleCreateSite()))
	mux.HandleFunc("DELETE /api/sites/{id}", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteDelete,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleDeleteSite()))
	mux.HandleFunc("GET /api/sites/{id}/stats", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetSiteStats()))
	mux.HandleFunc("GET /api/sites/{id}/hits", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetSiteHits()))
	mux.HandleFunc("GET /api/sites/{id}/hits/export", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleExportSiteHits()))
	mux.HandleFunc("GET /api/sites/{id}/ecommerce", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetSiteEcommerceSummary()))
	mux.HandleFunc("GET /api/sites/{id}/ecommerce/timeseries", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetSiteEcommerceTimeseries()))
	mux.HandleFunc("GET /api/sites/{id}/ecommerce/products", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetSiteEcommerceProducts()))
	mux.HandleFunc("GET /api/sites/{id}/ecommerce/sources", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetSiteEcommerceSources()))
	mux.HandleFunc("GET /api/favicon/{domain}", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetFavicon()))
	mux.HandleFunc("PUT /api/sites/{id}/retention", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageData,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleUpdateSiteRetention()))
	mux.HandleFunc("POST /api/sites/{id}/transfer-team", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageTeam,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleTransferSiteTeam()))
	mux.HandleFunc("GET /api/sites/{id}/exclusions", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageData,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleListSiteExclusions()))
	mux.HandleFunc("POST /api/sites/{id}/exclusions", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageData,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleCreateSiteExclusion()))
	mux.HandleFunc("DELETE /api/sites/{id}/exclusions/{ruleID}", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageData,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleDeleteSiteExclusion()))
}

var domainRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

func canManageTenantRole(role string) bool {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case database.TenantRoleOwner, database.TenantRoleAdmin:
		return true
	default:
		return false
	}
}

func (h *handler) handleGetSites() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		apiClientAuth, _ := r.Context().Value(shared.APIClientAuthKey).(*database.APIClientAuth)
		if userID == uuid.Nil && apiClientAuth == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		var (
			sites []api.Site
			err   error
		)
		switch {
		case userID != uuid.Nil:
			sites, err = h.ctx.Store.GetSites(r.Context(), userID)
		case apiClientAuth != nil && apiClientAuth.TenantID != uuid.Nil:
			sites, err = h.ctx.Store.ListSitesForTenant(r.Context(), apiClientAuth.TenantID)
		default:
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if err != nil {
			slog.Error("Failed to get sites", "error", err, "user_id", userID, "tenant_id", apiClientAuthTenantID(apiClientAuth))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if apiClientAuth != nil {
			filtered := make([]api.Site, 0, len(sites))
			for _, site := range sites {
				if _, allowed := apiClientAuth.SiteRoles[site.ID]; allowed {
					filtered = append(filtered, site)
				}
			}
			sites = filtered
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(sites); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func apiClientAuthTenantID(authz *database.APIClientAuth) uuid.UUID {
	if authz == nil {
		return uuid.Nil
	}
	return authz.TenantID
}

func (h *handler) handleCreateSite() http.HandlerFunc {
	type request struct {
		Domain string `json:"domain"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if h.ctx.Store == nil {
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

		site, err := h.ctx.Store.CreateSite(r.Context(), userID, domain)
		if err != nil {
			slog.Error("Failed to create site", "error", err, "domain", domain)
			http.Error(w, "Failed to create site (domain might already exist)", http.StatusConflict)
			return
		}

		if h.ctx.Config.DataRetentionDays > 0 {
			if err := h.ctx.Store.UpdateSiteRetention(r.Context(), site.ID, userID, h.ctx.Config.DataRetentionDays); err != nil {
				slog.Warn("Failed to set default data retention policy", "site_id", site.ID, "error", err)
			}
		}

		if h.ctx.TenantStores != nil {
			if err := h.ctx.TenantStores.SyncSite(r.Context(), site.ID); err != nil {
				slog.Error("Failed to sync tenant site mirror after create", "error", err, "site_id", site.ID)
				http.Error(w, "Failed to create site", http.StatusInternalServerError)
				return
			}
		}

		slog.Info("Site created", "id", site.ID, "domain", domain, "user_id", userID)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(site); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleDeleteSite() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site_id", http.StatusBadRequest)
			return
		}

		if h.ctx.TenantStores != nil {
			if err := h.ctx.TenantStores.DeleteSite(r.Context(), siteID); err != nil {
				slog.Error("Failed to delete site", "error", err, "site_id", siteID)
				http.Error(w, "Failed to delete site", http.StatusInternalServerError)
				return
			}
		} else if err := h.ctx.Store.DeleteSite(r.Context(), siteID); err != nil {
			slog.Error("Failed to delete site", "error", err, "site_id", siteID)
			http.Error(w, "Failed to delete site", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleUpdateSiteRetention() http.HandlerFunc {
	type request struct {
		Days int `json:"days"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
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
		site, err := h.ctx.Store.GetSite(r.Context(), siteID, userID)
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

		if err := h.ctx.Store.UpdateSiteRetention(r.Context(), siteID, userID, req.Days); err != nil {
			slog.Error("Failed to update site retention", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if h.ctx.TenantStores != nil {
			if err := h.ctx.TenantStores.SyncSite(r.Context(), siteID); err != nil {
				slog.Error("Failed to sync tenant site mirror after retention update", "error", err, "site_id", siteID)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (h *handler) handleTransferSiteTeam() http.HandlerFunc {
	type request struct {
		TeamID string `json:"team_id"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil || h.ctx.TenantStores == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		siteID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid site_id", http.StatusBadRequest)
			return
		}

		var req request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		destinationTeamID, err := uuid.Parse(strings.TrimSpace(req.TeamID))
		if err != nil {
			http.Error(w, "Invalid team_id", http.StatusBadRequest)
			return
		}

		destinationRole, err := h.ctx.Store.GetTenantRole(r.Context(), destinationTeamID, userID)
		if err != nil || !canManageTenantRole(destinationRole) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		sourceTeamID, err := h.ctx.Store.GetSiteTenantID(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to resolve source team for site transfer", "error", err, "site_id", siteID, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if err := h.ctx.TenantStores.TransferSite(r.Context(), siteID, destinationTeamID); err != nil {
			slog.Error("Failed to transfer site to team", "error", err, "site_id", siteID, "source_team_id", sourceTeamID, "destination_team_id", destinationTeamID, "user_id", userID)
			http.Error(w, "Failed to transfer site", http.StatusInternalServerError)
			return
		}

		site, _ := h.ctx.Store.GetSiteByID(r.Context(), siteID)
		siteLabel := siteID.String()
		if site != nil && site.Domain != "" {
			siteLabel = site.Domain
		}
		if err := h.ctx.Store.AppendTeamAuditEntry(r.Context(), sourceTeamID, userID, "site.transferred_out", fmt.Sprintf("Site %s moved to team %s", siteLabel, destinationTeamID), nil); err != nil {
			slog.Warn("Failed to append source team site transfer audit entry", "error", err, "site_id", siteID, "team_id", sourceTeamID)
		}
		if err := h.ctx.Store.AppendTeamAuditEntry(r.Context(), destinationTeamID, userID, "site.transferred_in", fmt.Sprintf("Site %s moved into this team", siteLabel), nil); err != nil {
			slog.Warn("Failed to append destination team site transfer audit entry", "error", err, "site_id", siteID, "team_id", destinationTeamID)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":              "ok",
			"site_id":             siteID,
			"source_team_id":      sourceTeamID,
			"destination_team_id": destinationTeamID,
		}); err != nil {
			slog.Error("Failed to encode site transfer response", "error", err, "site_id", siteID, "user_id", userID)
		}
	}
}

func (h *handler) handleListSiteExclusions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		siteID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid site_id", http.StatusBadRequest)
			return
		}

		rules, err := h.ctx.Store.ListSiteExclusions(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to list site exclusions", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(rules); err != nil {
			slog.Error("Failed to encode site exclusions response", "error", err, "site_id", siteID)
		}
	}
}

func (h *handler) handleCreateSiteExclusion() http.HandlerFunc {
	type request struct {
		CIDR        string `json:"cidr"`
		Description string `json:"description"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		siteID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid site_id", http.StatusBadRequest)
			return
		}

		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		normalizedCIDR, _, err := blocking.NormalizeCIDR(req.CIDR)
		if err != nil {
			http.Error(w, "Invalid IP or CIDR", http.StatusBadRequest)
			return
		}

		description := strings.TrimSpace(req.Description)
		if len(description) > 255 {
			http.Error(w, "Description must be 255 characters or fewer", http.StatusBadRequest)
			return
		}

		rule, err := h.ctx.Store.CreateSiteExclusion(r.Context(), siteID, normalizedCIDR, description, userID)
		if err != nil {
			slog.Error("Failed to create site exclusion", "error", err, "site_id", siteID, "cidr", normalizedCIDR)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		h.refreshIPFilter(r.Context())

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(rule); err != nil {
			slog.Error("Failed to encode site exclusion response", "error", err, "site_id", siteID)
		}
	}
}

func (h *handler) handleDeleteSiteExclusion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		siteID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid site_id", http.StatusBadRequest)
			return
		}

		ruleID, err := uuid.Parse(strings.TrimSpace(r.PathValue("ruleID")))
		if err != nil {
			http.Error(w, "Invalid rule_id", http.StatusBadRequest)
			return
		}

		deleted, err := h.ctx.Store.DeleteSiteExclusion(r.Context(), siteID, ruleID)
		if err != nil {
			slog.Error("Failed to delete site exclusion", "error", err, "site_id", siteID, "rule_id", ruleID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !deleted {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		h.refreshIPFilter(r.Context())

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *handler) refreshIPFilter(ctx context.Context) {
	if h.ctx.IPFilter == nil {
		return
	}
	if err := h.ctx.IPFilter.Refresh(ctx); err != nil {
		slog.Warn("Failed to refresh IP filter after exclusion write", "error", err)
	}
}

// handleGetSiteHits retrieves raw hits for a specific site.
// Path: GET /api/sites/{id}/hits
func (h *handler) handleGetSiteHits() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if h.ctx.Store == nil {
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

		filters, err := parseFilters(q)
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
			SiteID:    siteID,
			UserID:    userID,
			Start:     start,
			End:       end,
			Query:     q.Get("q"),
			SortField: q.Get("sort"),
			SortOrder: q.Get("order"), // asc/desc
			Limit:     limit,
			Offset:    offset,
			Filters:   filters,
		}

		analyticsStore, err := h.ctx.AnalyticsStore(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to resolve analytics store", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		result, err := analyticsStore.GetHits(r.Context(), params)
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

// handleExportSiteHits streams filtered hits in the requested export format.
// Path: GET /api/sites/{id}/hits/export
func (h *handler) handleExportSiteHits() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if h.ctx.Store == nil {
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

		filters, err := parseFilters(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		format := exportfmt.Normalize(q.Get("format"), exportfmt.FormatCSV)

		params := api.HitQueryParams{
			SiteID:  siteID,
			UserID:  userID,
			Start:   start,
			End:     end,
			Query:   q.Get("q"),
			Filters: filters,
		}

		analyticsStore, err := h.ctx.AnalyticsStore(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to resolve analytics store", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if format == exportfmt.FormatCSV {
			filename := fmt.Sprintf("hits_%s_%d.csv", siteID, time.Now().Unix())
			w.Header().Set("Content-Type", exportfmt.ContentType(exportfmt.FormatCSV))
			w.Header().Set("Content-Disposition", "attachment; filename="+filename)

			if err := analyticsStore.ExportHitsCSV(r.Context(), params, w); err != nil {
				slog.Error("Failed to export hits", "error", err, "site_id", siteID, "user_id", userID)
			}
			return
		}

		filename, err := analyticsStore.ExportHitsFile(r.Context(), params, format)
		if err != nil {
			slog.Error("Failed to export hits", "error", err, "site_id", siteID, "user_id", userID)
			http.Error(w, "Failed to export hits", http.StatusInternalServerError)
			return
		}
		downloadName := fmt.Sprintf("hits_%s_%d.%s", siteID, time.Now().Unix(), format)
		w.Header().Set("Content-Disposition", "attachment; filename="+downloadName)
		w.Header().Set("Content-Type", exportfmt.ContentType(format))
		http.ServeFile(w, r, filename)

		go func() {
			cleanupSiteHitsExportFile(filename)
		}()
	}
}

func cleanupSiteHitsExportFile(filename string) {
	if filename == "" {
		return
	}

	cleaned := filepath.Clean(filename)
	base := filepath.Base(cleaned)
	if !strings.HasPrefix(base, "hitkeep_hits_") {
		return
	}

	tempDir := filepath.Clean(os.TempDir())
	rel, err := filepath.Rel(tempDir, cleaned)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return
	}

	//nolint:gosec // cleaned path is constrained to an app-owned temp export under os.TempDir.
	_ = os.Remove(cleaned)
}

func (h *handler) handleGetSiteStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if h.ctx.Store == nil {
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

		filters, err := parseFilters(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var goalIDs []uuid.UUID
		for _, rawID := range q["goal_id"] {
			id, err := uuid.Parse(rawID)
			if err != nil {
				http.Error(w, "Invalid goal_id", http.StatusBadRequest)
				return
			}
			goalIDs = append(goalIDs, id)
		}

		var funnelIDs []uuid.UUID
		for _, rawID := range q["funnel_id"] {
			id, err := uuid.Parse(rawID)
			if err != nil {
				http.Error(w, "Invalid funnel_id", http.StatusBadRequest)
				return
			}
			funnelIDs = append(funnelIDs, id)
		}

		params := api.AnalyticsParams{
			SiteID:    siteID,
			UserID:    userID,
			Start:     start,
			End:       end,
			Filters:   filters,
			GoalIDs:   goalIDs,
			FunnelIDs: funnelIDs,
		}

		if compareFromStr := q.Get("compare_from"); compareFromStr != "" {
			if parsed, err := time.Parse(time.RFC3339, compareFromStr); err == nil {
				params.CompareStart = parsed
			}
		}
		if compareToStr := q.Get("compare_to"); compareToStr != "" {
			if parsed, err := time.Parse(time.RFC3339, compareToStr); err == nil {
				params.CompareEnd = parsed
			}
		}

		analyticsStore, err := h.ctx.AnalyticsStore(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to resolve analytics store", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		stats, err := analyticsStore.GetSiteStats(r.Context(), params)
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

func (h *handler) parseEcommerceParams(w http.ResponseWriter, r *http.Request, defaultLimit int) (api.EcommerceParams, bool) {
	if h.ctx.Store == nil {
		http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
		return api.EcommerceParams{}, false
	}

	siteIDStr := r.PathValue("id")
	siteID, err := uuid.Parse(siteIDStr)
	if err != nil {
		http.Error(w, "Invalid site_id", http.StatusBadRequest)
		return api.EcommerceParams{}, false
	}

	now := time.Now().UTC()
	end := now.AddDate(0, 0, 1)
	start := end.AddDate(0, 0, -30)
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

	filters, err := parseFilters(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return api.EcommerceParams{}, false
	}

	limit := defaultLimit
	if rawLimit := q.Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			http.Error(w, "Invalid limit", http.StatusBadRequest)
			return api.EcommerceParams{}, false
		}
		limit = parsed
	}

	return api.EcommerceParams{
		SiteID:   siteID,
		Start:    start,
		End:      end,
		Filters:  filters,
		ItemID:   strings.TrimSpace(q.Get("item_id")),
		ItemName: strings.TrimSpace(q.Get("item_name")),
		Limit:    limit,
	}, true
}

func (h *handler) handleGetSiteEcommerceSummary() http.HandlerFunc {
	return h.handleGetSiteEcommerce(func(ctx context.Context, store *database.Store, params api.EcommerceParams) (any, error) {
		return store.GetEcommerceSummary(ctx, params)
	}, "summary")
}

func (h *handler) handleGetSiteEcommerceTimeseries() http.HandlerFunc {
	return h.handleGetSiteEcommerce(func(ctx context.Context, store *database.Store, params api.EcommerceParams) (any, error) {
		return store.GetEcommerceTimeSeries(ctx, params)
	}, "timeseries")
}

func (h *handler) handleGetSiteEcommerceProducts() http.HandlerFunc {
	return h.handleGetSiteEcommerce(func(ctx context.Context, store *database.Store, params api.EcommerceParams) (any, error) {
		return store.GetEcommerceTopProducts(ctx, params)
	}, "products")
}

func (h *handler) handleGetSiteEcommerceSources() http.HandlerFunc {
	return h.handleGetSiteEcommerce(func(ctx context.Context, store *database.Store, params api.EcommerceParams) (any, error) {
		return store.GetEcommerceSources(ctx, params)
	}, "sources")
}

func (h *handler) handleGetSiteEcommerce(load func(context.Context, *database.Store, api.EcommerceParams) (any, error), label string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params, ok := h.parseEcommerceParams(w, r, 10)
		if !ok {
			return
		}

		analyticsStore, err := h.ctx.AnalyticsStore(r.Context(), params.SiteID)
		if err != nil {
			slog.Error("Failed to resolve analytics store", "error", err, "site_id", params.SiteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		payload, err := load(r.Context(), analyticsStore, params)
		if err != nil {
			slog.Error("Failed to get ecommerce "+label, "error", err, "site_id", params.SiteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func parseFilters(q url.Values) ([]api.Filter, error) {
	var filters []api.Filter

	for _, raw := range q["filter"] {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid filter format")
		}
		filterType := strings.ToLower(strings.TrimSpace(parts[0]))
		filterValue := strings.TrimSpace(parts[1])
		if err := validateFilter(filterType, filterValue); err != nil {
			return nil, err
		}
		filters = append(filters, api.Filter{Type: filterType, Value: filterValue})
	}

	filterType := strings.ToLower(strings.TrimSpace(q.Get("filter_type")))
	filterValue := strings.TrimSpace(q.Get("filter_value"))
	if filterType != "" || filterValue != "" {
		if err := validateFilter(filterType, filterValue); err != nil {
			return nil, err
		}
		filters = append(filters, api.Filter{Type: filterType, Value: filterValue})
	}

	return filters, nil
}

func validateFilter(filterType, filterValue string) error {
	if filterType == "" || filterValue == "" {
		return fmt.Errorf("filter_type and filter_value are required together")
	}

	switch filterType {
	case "path", "referrer", "device", "country", "browser", "language", "utm_campaign", "utm_content", "utm_medium", "utm_source", "utm_term":
		return nil
	default:
		return fmt.Errorf("invalid filter_type")
	}
}

// handleGetFavicon proxies the favicon request to DuckDuckGo to avoid CORS and privacy leaks.
// GET /api/favicon/{domain}
func (h *handler) handleGetFavicon() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := normalizeFaviconDomain(r.PathValue("domain"))
		if !isValidFaviconDomain(domain) {
			http.Error(w, "Invalid domain", http.StatusBadRequest)
			return
		}

		ddgURL := (&url.URL{
			Scheme: "https",
			Host:   "icons.duckduckgo.com",
			Path:   fmt.Sprintf("/ip3/%s.ico", domain),
		}).String()

		target, err := url.Parse(ddgURL)
		if err != nil {
			http.Error(w, "Upstream error", http.StatusBadGateway)
			return
		}

		proxy := &httputil.ReverseProxy{
			Rewrite: func(proxyReq *httputil.ProxyRequest) {
				rewrittenURL := *target
				proxyReq.Out.URL = &rewrittenURL
				proxyReq.Out.Host = ""
				proxyReq.Out.Method = http.MethodGet
				proxyReq.Out.Body = nil
				proxyReq.Out.ContentLength = 0
				proxyReq.Out.Header.Del("Authorization")
				proxyReq.Out.Header.Del("Cookie")
				proxyReq.Out.Header.Del("X-API-Key")
			},
			Transport: faviconProxyTransport,
			ModifyResponse: func(resp *http.Response) error {
				resp.Header.Set("Cache-Control", "public, max-age=86400")
				return nil
			},
			ErrorHandler: func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
				slog.Warn("Failed to fetch favicon upstream", "domain", domain, "error", proxyErr)
				http.Error(rw, "Upstream error", http.StatusBadGateway)
			},
		}

		proxy.ServeHTTP(w, r)
	}
}

func normalizeFaviconDomain(domain string) string {
	trimmed := strings.TrimSpace(domain)
	return strings.TrimSuffix(strings.ToLower(trimmed), ".")
}

func isValidFaviconDomain(domain string) bool {
	if domain == "" {
		return false
	}
	if strings.ContainsAny(domain, `/\?#`) {
		return false
	}
	return domainRegex.MatchString(domain)
}

func newFaviconProxyTransport(timeout time.Duration) http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = timeout
	return transport
}
