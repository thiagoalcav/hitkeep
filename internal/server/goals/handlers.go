package goals

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	authcore "hitkeep/internal/auth"
	"hitkeep/internal/server/shared"
)

type handler struct {
	ctx *shared.Context
}

func Register(mux *http.ServeMux, ctx *shared.Context) {
	h := &handler{ctx: ctx}
	mux.HandleFunc("GET /api/sites/{id}/goals", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetGoals()))
	mux.HandleFunc("GET /api/sites/{id}/goals/timeseries", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetGoalTimeseries()))
	mux.HandleFunc("POST /api/sites/{id}/goals", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageGoals,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleCreateGoal()))
	mux.HandleFunc("DELETE /api/sites/{id}/goals/{goalID}", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageGoals,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleDeleteGoal()))

	mux.HandleFunc("GET /api/sites/{id}/funnels", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetFunnels()))
	mux.HandleFunc("GET /api/sites/{id}/funnels/timeseries", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetFunnelTimeseries()))
	mux.HandleFunc("POST /api/sites/{id}/funnels", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageGoals,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleCreateFunnel()))
	mux.HandleFunc("DELETE /api/sites/{id}/funnels/{funnelID}", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageGoals,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleDeleteFunnel()))
	mux.HandleFunc("GET /api/sites/{id}/funnels/{funnelID}/stats", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetFunnelStats()))
}

// Helper to validate user access to a site.
// Returns the siteID and true if authorized, otherwise handles the error response and returns false.
func (h *handler) validateSiteOwnership(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID := shared.GetUserIDFromContext(r)
	if userID == uuid.Nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return uuid.Nil, false
	}

	siteIDStr := r.PathValue("id")
	siteID, err := uuid.Parse(siteIDStr)
	if err != nil {
		http.Error(w, "Invalid site_id", http.StatusBadRequest)
		return uuid.Nil, false
	}

	// Verify ownership
	site, err := h.ctx.Store.GetSite(r.Context(), siteID, userID)
	if err != nil || site == nil {
		http.Error(w, "Site not found", http.StatusNotFound)
		return uuid.Nil, false
	}
	return siteID, true
}

// Goals

func (h *handler) handleGetGoals() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteID, ok := h.validateSiteOwnership(w, r)
		if !ok {
			return
		}

		goals, err := h.ctx.Store.GetGoals(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to get goals", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(goals); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleGetGoalTimeseries() http.HandlerFunc {
	return h.handleTimeseries("goal_id", "Invalid goal_id", "Failed to get goal timeseries",
		func(ctx context.Context, params api.AnalyticsParams, ids []uuid.UUID) (any, error) {
			return h.ctx.Store.GetGoalTimeseries(ctx, params, ids)
		})
}

func (h *handler) handleCreateGoal() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteID, ok := h.validateSiteOwnership(w, r)
		if !ok {
			return
		}

		var req api.Goal
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" || req.Value == "" || (req.Type != "event" && req.Type != "path") {
			http.Error(w, "Invalid goal data", http.StatusBadRequest)
			return
		}

		req.SiteID = siteID
		req.CreatedAt = time.Now()

		if err := h.ctx.Store.CreateGoal(r.Context(), &req); err != nil {
			slog.Error("Failed to create goal", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func (h *handler) handleDeleteGoal() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteID, ok := h.validateSiteOwnership(w, r)
		if !ok {
			return
		}

		goalIDStr := r.PathValue("goalID")
		goalID, err := uuid.Parse(goalIDStr)
		if err != nil {
			http.Error(w, "Invalid goal_id", http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.DeleteGoal(r.Context(), goalID, siteID); err != nil {
			slog.Error("Failed to delete goal", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// Funnels

func (h *handler) handleGetFunnels() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteID, ok := h.validateSiteOwnership(w, r)
		if !ok {
			return
		}

		funnels, err := h.ctx.Store.GetFunnels(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to get funnels", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(funnels); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleGetFunnelTimeseries() http.HandlerFunc {
	return h.handleTimeseries("funnel_id", "Invalid funnel_id", "Failed to get funnel timeseries",
		func(ctx context.Context, params api.AnalyticsParams, ids []uuid.UUID) (any, error) {
			return h.ctx.Store.GetFunnelTimeseries(ctx, params, ids)
		})
}

func (h *handler) handleTimeseries(
	idParam string,
	invalidIDMessage string,
	logMessage string,
	fetch func(context.Context, api.AnalyticsParams, []uuid.UUID) (any, error),
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteID, ok := h.validateSiteOwnership(w, r)
		if !ok {
			return
		}

		start, end := parseTimeseriesRange(r.URL.Query())

		ids, err := parseUUIDQueryParam(r.URL.Query(), idParam)
		if err != nil {
			http.Error(w, invalidIDMessage, http.StatusBadRequest)
			return
		}

		params := api.AnalyticsParams{
			SiteID: siteID,
			Start:  start,
			End:    end,
		}

		series, err := fetch(r.Context(), params, ids)
		if err != nil {
			slog.Error(logMessage, "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(series); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func parseTimeseriesRange(q url.Values) (time.Time, time.Time) {
	now := time.Now().UTC()
	end := now.AddDate(0, 0, 1)
	start := end.AddDate(0, 0, -30)

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

	return start, end
}

func parseUUIDQueryParam(q url.Values, key string) ([]uuid.UUID, error) {
	values := q[key]
	if len(values) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, 0, len(values))
	for _, rawID := range values {
		id, err := uuid.Parse(rawID)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (h *handler) handleCreateFunnel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteID, ok := h.validateSiteOwnership(w, r)
		if !ok {
			return
		}

		var req api.Funnel
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" || len(req.Steps) < 2 {
			http.Error(w, "Invalid funnel data (need name and at least 2 steps)", http.StatusBadRequest)
			return
		}

		req.SiteID = siteID
		req.CreatedAt = time.Now()

		if err := h.ctx.Store.CreateFunnel(r.Context(), &req); err != nil {
			slog.Error("Failed to create funnel", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func (h *handler) handleDeleteFunnel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteID, ok := h.validateSiteOwnership(w, r)
		if !ok {
			return
		}

		funnelIDStr := r.PathValue("funnelID")
		funnelID, err := uuid.Parse(funnelIDStr)
		if err != nil {
			http.Error(w, "Invalid funnel_id", http.StatusBadRequest)
			return
		}

		if err := h.ctx.Store.DeleteFunnel(r.Context(), funnelID, siteID); err != nil {
			slog.Error("Failed to delete funnel", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func (h *handler) handleGetFunnelStats() http.HandlerFunc {
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

		funnelIDStr := r.PathValue("funnelID")
		funnelID, err := uuid.Parse(funnelIDStr)
		if err != nil {
			http.Error(w, "Invalid funnel_id", http.StatusBadRequest)
			return
		}

		// Verify ownership
		site, err := h.ctx.Store.GetSite(r.Context(), siteID, userID)
		if err != nil || site == nil {
			http.Error(w, "Site not found", http.StatusNotFound)
			return
		}

		// Parse time range
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

		params := api.AnalyticsParams{
			SiteID: siteID,
			UserID: userID,
			Start:  start,
			End:    end,
		}

		stats, err := h.ctx.Store.GetFunnelStats(r.Context(), funnelID, params)
		if err != nil {
			slog.Error("Failed to get funnel stats", "error", err)
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "Funnel not found", http.StatusNotFound)
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
