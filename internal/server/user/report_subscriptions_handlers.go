package user

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

func (h *handler) handleGetReportSubscriptions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		subs, err := h.ctx.Store.GetReportSubscriptions(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to load report subscriptions", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(subs); err != nil {
			slog.Error("Failed to encode report subscriptions", "error", err)
		}
	}
}

func (h *handler) handleUpdateSiteReportSubscription() http.HandlerFunc {
	type request struct {
		Daily   bool `json:"daily"`
		Weekly  bool `json:"weekly"`
		Monthly bool `json:"monthly"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		siteIDStr := r.PathValue("site_id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site ID", http.StatusBadRequest)
			return
		}

		var req request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		freqs := []struct {
			freq    api.ReportFrequency
			enabled bool
		}{
			{api.ReportFrequencyDaily, req.Daily},
			{api.ReportFrequencyWeekly, req.Weekly},
			{api.ReportFrequencyMonthly, req.Monthly},
		}

		for _, f := range freqs {
			if err := h.ctx.Store.UpsertSiteReportSubscription(r.Context(), userID, siteID, f.freq, f.enabled); err != nil {
				if errors.Is(err, database.ErrSiteAccessRequired) {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				slog.Error("Failed to upsert site report subscription", "error", err, "user_id", userID, "site_id", siteID, "freq", f.freq)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *handler) handleUpdateDigestSubscription() http.HandlerFunc {
	type request struct {
		Daily   bool `json:"daily"`
		Weekly  bool `json:"weekly"`
		Monthly bool `json:"monthly"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		freqs := []struct {
			freq    api.ReportFrequency
			enabled bool
		}{
			{api.ReportFrequencyDaily, req.Daily},
			{api.ReportFrequencyWeekly, req.Weekly},
			{api.ReportFrequencyMonthly, req.Monthly},
		}

		for _, f := range freqs {
			if err := h.ctx.Store.UpsertDigestSubscription(r.Context(), userID, f.freq, f.enabled); err != nil {
				slog.Error("Failed to upsert digest subscription", "error", err, "user_id", userID, "freq", f.freq)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
