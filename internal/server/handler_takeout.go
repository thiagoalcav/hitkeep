package server

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"hitkeep/internal/takeout"
)

type TakeoutHandler struct {
	service *takeout.TakeoutService
}

func NewTakeoutHandler(service *takeout.TakeoutService) *TakeoutHandler {
	return &TakeoutHandler{
		service: service,
	}
}
func (h *TakeoutHandler) handleUserTakeout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(UserIDKey).(uuid.UUID)

		filename, err := h.service.ExportUserData(r.Context(), userID)
		if err != nil {
			http.Error(w, "Failed to export user data", http.StatusInternalServerError)
			return
		}

		// Serve the file
		w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filename))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, filename)

		go func() {
			_ = os.Remove(filename)
		}()
	}
}
func (h *TakeoutHandler) handleSiteTakeout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site ID", http.StatusBadRequest)
			return
		}

		// Validate format
		format := r.URL.Query().Get("format")
		switch format {
		case "csv", "parquet":
			// allowed
		default:
			// Default to xlsx for better compatibility with mixed schema exports (hits + events)
			format = "xlsx"
		}

		filename, err := h.service.ExportSiteData(r.Context(), siteID, format)
		if err != nil {
			http.Error(w, "Failed to export site data", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filename))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, filename)

		go func() {
			_ = os.Remove(filename)
		}()
	}
}
