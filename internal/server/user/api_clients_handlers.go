package user

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	authcore "hitkeep/internal/auth"
	"hitkeep/internal/database"
	"hitkeep/internal/server/shared"
)

type apiClientSiteRoleInput struct {
	SiteID string `json:"site_id"`
	Role   string `json:"role"`
}

type createAPIClientRequest struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	InstanceRole string                   `json:"instance_role"`
	ExpiresAt    *time.Time               `json:"expires_at"`
	SiteRoles    []apiClientSiteRoleInput `json:"site_roles"`
}

type updateAPIClientRequest struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	InstanceRole string                   `json:"instance_role"`
	ExpiresAt    *time.Time               `json:"expires_at"`
	Revoked      *bool                    `json:"revoked"`
	SiteRoles    []apiClientSiteRoleInput `json:"site_roles"`
}

func (h *handler) handleListAPIClients() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		clients, err := h.ctx.Store.ListAPIClients(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to list api clients", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(clients); err != nil {
			slog.Error("Failed to encode api clients response", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleCreateAPIClient() http.HandlerFunc {
	type response struct {
		Client any    `json:"client"`
		Token  string `json:"token"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var req createAPIClientRequest
		if err := decodeJSON(r, &req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" || len(name) > 120 {
			http.Error(w, "Name is required and must be <= 120 characters", http.StatusBadRequest)
			return
		}

		description := strings.TrimSpace(req.Description)
		if len(description) > 500 {
			http.Error(w, "Description must be <= 500 characters", http.StatusBadRequest)
			return
		}

		requestedInstanceRole := authcore.InstanceUser
		if strings.TrimSpace(req.InstanceRole) != "" {
			requestedInstanceRole = authcore.InstanceRole(strings.TrimSpace(req.InstanceRole))
		}
		if !authcore.IsValidInstanceRole(requestedInstanceRole) {
			http.Error(w, "Invalid instance role", http.StatusBadRequest)
			return
		}

		actorInstanceRole, err := h.ctx.Store.GetInstanceRole(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to get actor instance role", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !authcore.CanAssignInstanceRole(actorInstanceRole, requestedInstanceRole) {
			http.Error(w, "Cannot delegate requested instance role", http.StatusForbidden)
			return
		}

		siteRoles, err := h.validateDelegatedSiteRoles(r, userID, actorInstanceRole, req.SiteRoles)
		if err != nil {
			slog.Warn("Invalid delegated site roles", "error", err, "user_id", userID)
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		client, token, err := h.ctx.Store.CreateAPIClient(
			r.Context(),
			userID,
			name,
			description,
			requestedInstanceRole,
			siteRoles,
			req.ExpiresAt,
		)
		if err != nil {
			slog.Error("Failed to create api client", "error", err, "user_id", userID)
			http.Error(w, "Failed to create api client", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(response{Client: client, Token: token}); err != nil {
			slog.Error("Failed to encode create api client response", "error", err, "user_id", userID)
		}
	}
}

func (h *handler) handleUpdateAPIClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		clientID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid client ID", http.StatusBadRequest)
			return
		}

		existing, err := h.ctx.Store.GetAPIClient(r.Context(), userID, clientID)
		if err != nil {
			slog.Error("Failed to load api client before update", "error", err, "user_id", userID, "client_id", clientID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if existing == nil {
			http.Error(w, "API client not found", http.StatusNotFound)
			return
		}

		var req updateAPIClientRequest
		if err := decodeJSON(r, &req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" || len(name) > 120 {
			http.Error(w, "Name is required and must be <= 120 characters", http.StatusBadRequest)
			return
		}

		description := strings.TrimSpace(req.Description)
		if len(description) > 500 {
			http.Error(w, "Description must be <= 500 characters", http.StatusBadRequest)
			return
		}

		requestedInstanceRole := authcore.InstanceRole(strings.TrimSpace(req.InstanceRole))
		if !authcore.IsValidInstanceRole(requestedInstanceRole) {
			http.Error(w, "Invalid instance role", http.StatusBadRequest)
			return
		}

		actorInstanceRole, err := h.ctx.Store.GetInstanceRole(r.Context(), userID)
		if err != nil {
			slog.Error("Failed to get actor instance role", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if !authcore.CanAssignInstanceRole(actorInstanceRole, requestedInstanceRole) {
			http.Error(w, "Cannot delegate requested instance role", http.StatusForbidden)
			return
		}

		siteRoles, err := h.validateDelegatedSiteRoles(r, userID, actorInstanceRole, req.SiteRoles)
		if err != nil {
			slog.Warn("Invalid delegated site roles for update", "error", err, "user_id", userID, "client_id", clientID)
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}

		revoked := existing.RevokedAt != nil
		if req.Revoked != nil {
			revoked = *req.Revoked
		}

		updated, err := h.ctx.Store.UpdateAPIClient(
			r.Context(),
			userID,
			clientID,
			name,
			description,
			requestedInstanceRole,
			siteRoles,
			req.ExpiresAt,
			revoked,
		)
		if err != nil {
			slog.Error("Failed to update api client", "error", err, "user_id", userID, "client_id", clientID)
			http.Error(w, "Failed to update api client", http.StatusInternalServerError)
			return
		}
		if updated == nil {
			http.Error(w, "API client not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(updated); err != nil {
			slog.Error("Failed to encode api client update response", "error", err, "user_id", userID, "client_id", clientID)
		}
	}
}

func (h *handler) handleDeleteAPIClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := shared.GetUserIDFromContext(r)
		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		clientID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
		if err != nil {
			http.Error(w, "Invalid client ID", http.StatusBadRequest)
			return
		}

		err = h.ctx.Store.DeleteAPIClient(r.Context(), userID, clientID)
		if err != nil {
			if errors.Is(err, database.ErrAPIClientNotFound) {
				http.Error(w, "API client not found", http.StatusNotFound)
				return
			}
			slog.Error("Failed to delete api client", "error", err, "user_id", userID, "client_id", clientID)
			http.Error(w, "Failed to delete api client", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *handler) validateDelegatedSiteRoles(r *http.Request, userID uuid.UUID, actorInstanceRole authcore.InstanceRole, roles []apiClientSiteRoleInput) (map[uuid.UUID]authcore.SiteRole, error) {
	if len(roles) == 0 {
		return map[uuid.UUID]authcore.SiteRole{}, nil
	}
	if len(roles) > 100 {
		return nil, errors.New("too many delegated site roles")
	}

	result := make(map[uuid.UUID]authcore.SiteRole, len(roles))
	for _, item := range roles {
		siteID, err := uuid.Parse(strings.TrimSpace(item.SiteID))
		if err != nil {
			return nil, errors.New("invalid delegated site role site_id")
		}

		requestedRole := authcore.SiteRole(strings.TrimSpace(item.Role))
		if !authcore.IsValidSiteRole(requestedRole) {
			return nil, errors.New("invalid delegated site role")
		}

		maxRole, err := h.maxDelegableSiteRole(r, userID, actorInstanceRole, siteID)
		if err != nil {
			return nil, err
		}
		if !authcore.CanAssignSiteRole(maxRole, requestedRole) {
			return nil, errors.New("cannot delegate requested site role")
		}

		result[siteID] = requestedRole
	}

	return result, nil
}

func (h *handler) maxDelegableSiteRole(r *http.Request, userID uuid.UUID, actorInstanceRole authcore.InstanceRole, siteID uuid.UUID) (authcore.SiteRole, error) {
	if actorInstanceRole == authcore.InstanceOwner {
		return authcore.SiteOwner, nil
	}

	role, err := h.ctx.Store.GetSiteRole(r.Context(), userID, siteID)
	if err != nil {
		return "", errors.New("cannot delegate site role for site without access")
	}
	return role, nil
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("unexpected trailing JSON content")
	}
	return nil
}
