package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	authcore "hitkeep/internal/auth"
	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

type handler struct {
	ctx *shared.Context
}

func Register(mux *http.ServeMux, ctx *shared.Context) {
	h := &handler{ctx: ctx}
	mux.HandleFunc("GET /api/sites/{id}/events/names", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetEventNames()))
	mux.HandleFunc("GET /api/sites/{id}/events/properties", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetEventPropertyKeys()))
	mux.HandleFunc("GET /api/sites/{id}/events/breakdown", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetEventPropertyBreakdown()))
	mux.HandleFunc("GET /api/sites/{id}/events/timeseries", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetEventTimeseries()))
	mux.HandleFunc("GET /api/sites/{id}/events/audience", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetEventAudience()))
}

func parseSiteAndRange(w http.ResponseWriter, r *http.Request) (uuid.UUID, time.Time, time.Time, bool) {
	siteIDStr := r.PathValue("id")
	siteID, err := uuid.Parse(siteIDStr)
	if err != nil {
		http.Error(w, "Invalid site_id", http.StatusBadRequest)
		return uuid.Nil, time.Time{}, time.Time{}, false
	}

	q := r.URL.Query()
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
	return siteID, start, end, true
}

func (h *handler) handleGetEventNames() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		siteID, start, end, ok := parseSiteAndRange(w, r)
		if !ok {
			return
		}

		params := api.EventNamesParams{
			SiteID: siteID,
			Start:  start,
			End:    end,
		}

		analyticsStore, err := h.ctx.AnalyticsStore(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to resolve analytics store", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		names, err := analyticsStore.GetEventNames(r.Context(), params)
		if err != nil {
			slog.Error("Failed to get event names", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(names); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleGetEventPropertyKeys() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		siteID, start, end, ok := parseSiteAndRange(w, r)
		if !ok {
			return
		}

		eventName := r.URL.Query().Get("event_name")
		if eventName == "" {
			http.Error(w, "event_name is required", http.StatusBadRequest)
			return
		}

		params := api.EventNamesParams{
			SiteID: siteID,
			Start:  start,
			End:    end,
		}

		analyticsStore, err := h.ctx.AnalyticsStore(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to resolve analytics store", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		keys, err := analyticsStore.GetEventPropertyKeys(r.Context(), params, eventName)
		if err != nil {
			slog.Error("Failed to get event property keys", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(keys); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleGetEventPropertyBreakdown() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		siteID, start, end, ok := parseSiteAndRange(w, r)
		if !ok {
			return
		}

		q := r.URL.Query()
		eventName := q.Get("event_name")
		propertyKey := q.Get("property_key")
		if eventName == "" || propertyKey == "" {
			http.Error(w, "event_name and property_key are required", http.StatusBadRequest)
			return
		}

		params := api.EventBreakdownParams{
			SiteID:      siteID,
			Start:       start,
			End:         end,
			EventName:   eventName,
			PropertyKey: propertyKey,
		}

		analyticsStore, err := h.ctx.AnalyticsStore(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to resolve analytics store", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		breakdown, err := analyticsStore.GetEventPropertyBreakdown(r.Context(), params)
		if err != nil {
			slog.Error("Failed to get event property breakdown", "error", err, "site_id", siteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(breakdown); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

type eventQueryParams struct {
	SiteID         uuid.UUID
	Start          time.Time
	End            time.Time
	EventName      string
	PropertyKey    string
	PropertyValue  string
	DimensionKey   string
	DimensionValue string
}

func (h *handler) parseEventQueryParams(w http.ResponseWriter, r *http.Request) (eventQueryParams, bool) {
	if h.ctx.Store == nil {
		http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
		return eventQueryParams{}, false
	}

	siteID, start, end, ok := parseSiteAndRange(w, r)
	if !ok {
		return eventQueryParams{}, false
	}

	q := r.URL.Query()
	eventName := q.Get("event_name")
	if eventName == "" {
		http.Error(w, "event_name is required", http.StatusBadRequest)
		return eventQueryParams{}, false
	}

	return eventQueryParams{
		SiteID:         siteID,
		Start:          start,
		End:            end,
		EventName:      eventName,
		PropertyKey:    q.Get("property_key"),
		PropertyValue:  q.Get("property_value"),
		DimensionKey:   q.Get("dimension_key"),
		DimensionValue: q.Get("dimension_value"),
	}, true
}

func (h *handler) handleGetEventAudience() http.HandlerFunc {
	return h.eventQueryHandler("event audience", func(ctx context.Context, store *database.Store, p eventQueryParams) (any, error) {
		return store.GetEventAudience(ctx, api.EventAudienceParams(p))
	})
}

func (h *handler) handleGetEventTimeseries() http.HandlerFunc {
	return h.eventQueryHandler("event timeseries", func(ctx context.Context, store *database.Store, p eventQueryParams) (any, error) {
		return store.GetEventTimeseries(ctx, api.EventTimeseriesParams(p))
	})
}

func (h *handler) eventQueryHandler(label string, query func(context.Context, *database.Store, eventQueryParams) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := h.parseEventQueryParams(w, r)
		if !ok {
			return
		}

		analyticsStore, err := h.ctx.AnalyticsStore(r.Context(), p.SiteID)
		if err != nil {
			slog.Error("Failed to resolve analytics store", "error", err, "site_id", p.SiteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		result, err := query(r.Context(), analyticsStore, p)
		if err != nil {
			slog.Error("Failed to get "+label, "error", err, "site_id", p.SiteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}
