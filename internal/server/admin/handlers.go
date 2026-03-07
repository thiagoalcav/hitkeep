package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	authcore "hitkeep/internal/auth"
	"hitkeep/internal/blocking"
	"hitkeep/internal/database"
	"hitkeep/internal/mailables"
	serverauth "hitkeep/internal/server/auth"
	"hitkeep/internal/server/shared"
)

type handler struct {
	ctx *shared.Context
}

func Register(mux *http.ServeMux, ctx *shared.Context) {
	h := &handler{ctx: ctx}
	mux.HandleFunc("GET /api/admin/users", ctx.Handler(shared.HandlerConfig{
		InstancePerm: authcore.PermInstanceManageUsers,
		RateLimiter:  ctx.ApiLimiter,
	}, h.handleListUsers()))
	mux.HandleFunc("POST /api/admin/users/{id}/role", ctx.Handler(shared.HandlerConfig{
		InstancePerm: authcore.PermInstanceManageUsers,
		RateLimiter:  ctx.ApiLimiter,
	}, h.handleUpdateUserRole()))
	mux.HandleFunc("DELETE /api/admin/users/{id}", ctx.Handler(shared.HandlerConfig{
		InstancePerm: authcore.PermInstanceManageUsers,
		RateLimiter:  ctx.ApiLimiter,
	}, h.handleDeleteUser()))
	mux.HandleFunc("GET /api/admin/sites", ctx.Handler(shared.HandlerConfig{
		InstancePerm: authcore.PermInstanceManageUsers,
		RateLimiter:  ctx.ApiLimiter,
	}, h.handleAdminListSites()))
	mux.HandleFunc("DELETE /api/admin/sites/{id}", ctx.Handler(shared.HandlerConfig{
		InstancePerm: authcore.PermInstanceManageUsers,
		RateLimiter:  ctx.ApiLimiter,
	}, h.handleAdminDeleteSite()))
	mux.HandleFunc("GET /api/admin/exclusions", ctx.Handler(shared.HandlerConfig{
		InstancePerm: authcore.PermInstanceViewAllSites,
		RateLimiter:  ctx.ApiLimiter,
	}, h.handleListInstanceExclusions()))
	mux.HandleFunc("POST /api/admin/exclusions", ctx.Handler(shared.HandlerConfig{
		InstancePerm: authcore.PermInstanceViewAllSites,
		RateLimiter:  ctx.ApiLimiter,
	}, h.handleCreateInstanceExclusion()))
	mux.HandleFunc("DELETE /api/admin/exclusions/{ruleID}", ctx.Handler(shared.HandlerConfig{
		InstancePerm: authcore.PermInstanceViewAllSites,
		RateLimiter:  ctx.ApiLimiter,
	}, h.handleDeleteInstanceExclusion()))

	mux.HandleFunc("GET /api/sites/{id}/members", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteView,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetSiteMembers()))
	mux.HandleFunc("POST /api/sites/{id}/members", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageTeam,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleAddSiteMember()))
	mux.HandleFunc("DELETE /api/sites/{id}/members/{userId}", ctx.Handler(shared.HandlerConfig{
		SitePerm:    authcore.PermSiteManageTeam,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleRemoveSiteMember()))
}

func (h *handler) handleListUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := h.ctx.Store.ListUsers(r.Context())
		if err != nil {
			slog.Error("Failed to list users", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(users); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleListInstanceExclusions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		rules, err := h.ctx.Store.ListInstanceExclusions(r.Context())
		if err != nil {
			slog.Error("Failed to list instance exclusions", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(rules); err != nil {
			slog.Error("Failed to encode instance exclusions response", "error", err)
		}
	}
}

func (h *handler) handleCreateInstanceExclusion() http.HandlerFunc {
	type request struct {
		CIDR        string `json:"cidr"`
		Description string `json:"description"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
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

		rule, err := h.ctx.Store.CreateInstanceExclusion(r.Context(), normalizedCIDR, description, userID)
		if err != nil {
			slog.Error("Failed to create instance exclusion", "error", err, "cidr", normalizedCIDR)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		h.refreshIPFilter(r.Context())

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(rule); err != nil {
			slog.Error("Failed to encode instance exclusion response", "error", err)
		}
	}
}

func (h *handler) handleDeleteInstanceExclusion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.ctx.Store == nil {
			http.Error(w, "Service not available on this node", http.StatusServiceUnavailable)
			return
		}

		ruleID, err := uuid.Parse(strings.TrimSpace(r.PathValue("ruleID")))
		if err != nil {
			http.Error(w, "Invalid rule ID", http.StatusBadRequest)
			return
		}

		deleted, err := h.ctx.Store.DeleteInstanceExclusion(r.Context(), ruleID)
		if err != nil {
			slog.Error("Failed to delete instance exclusion", "error", err, "rule_id", ruleID)
			http.Error(w, "Internal error", http.StatusInternalServerError)
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

func (h *handler) handleUpdateUserRole() http.HandlerFunc {
	type request struct {
		Role string `json:"role"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		targetUserIDStr := r.PathValue("id")
		targetUserID, err := uuid.Parse(targetUserIDStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		actorID := shared.GetUserIDFromContext(r)

		err = h.ctx.Store.UpdateInstanceRole(r.Context(), targetUserID, authcore.InstanceRole(req.Role), actorID)
		if err != nil {
			slog.Error("Failed to update role", "error", err)
			http.Error(w, "Failed to update role", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleDeleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetUserIDStr := r.PathValue("id")
		targetUserID, err := uuid.Parse(targetUserIDStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		actorID := shared.GetUserIDFromContext(r)

		// Prevent deleting yourself
		if actorID == targetUserID {
			http.Error(w, "Cannot delete yourself", http.StatusBadRequest)
			return
		}

		// Check if target user is an owner (optional safety check, though role check handles permissions)
		// Ideally, only owners can delete other owners, etc.
		// For now, we rely on the route permission check (PermInstanceManageUsers).

		err = h.ctx.Store.DeleteUser(r.Context(), targetUserID)
		if err != nil {
			var ownsTeamsErr *database.UserOwnsTeamsError
			if errors.As(err, &ownsTeamsErr) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				if encodeErr := json.NewEncoder(w).Encode(api.AdminDeleteUserBlockedResponse{
					Status:  "error",
					Code:    "user_owns_teams",
					Message: "Transfer ownership before deleting this user.",
					Teams:   ownsTeamsErr.Teams,
				}); encodeErr != nil {
					slog.Error("Failed to encode delete user blocked response", "error", encodeErr, "target_user_id", targetUserID)
				}
				return
			}
			slog.Error("Failed to delete user", "error", err)
			http.Error(w, "Failed to delete user", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleAdminListSites() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sites, err := h.ctx.Store.ListAllSites(r.Context())
		if err != nil {
			slog.Error("Failed to list all sites", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(sites); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleAdminDeleteSite() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site ID", http.StatusBadRequest)
			return
		}

		if h.ctx.TenantStores != nil {
			err = h.ctx.TenantStores.DeleteSite(r.Context(), siteID)
		} else {
			err = h.ctx.Store.DeleteSite(r.Context(), siteID)
		}
		if err != nil {
			slog.Error("Failed to delete site", "error", err)
			http.Error(w, "Failed to delete site", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleGetSiteMembers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site ID", http.StatusBadRequest)
			return
		}

		members, err := h.ctx.Store.GetSiteMembers(r.Context(), siteID)
		if err != nil {
			slog.Error("Failed to get members", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(members); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleAddSiteMember() http.HandlerFunc {
	type request struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site ID", http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Find user by email
		user, err := h.ctx.Store.GetUserByEmail(r.Context(), req.Email)
		if err != nil {
			slog.Error("Database error checking user", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// If user doesn't exist, create them (simplified flow)
		var userID uuid.UUID
		var isNewUser bool
		var inviteToken string

		if user == nil {
			// Create a placeholder user. In a real system, this would trigger an invite email.
			// For now, we create a user with a random password that they can reset later.
			// Or better, we just create the user record.
			// Since CreateUser requires a password, we'll generate a random one.
			tempPassword := uuid.New().String() // Temporary
			hashedPassword, err := serverauth.HashPassword(tempPassword)
			if err != nil {
				slog.Error("Failed to hash password", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			userID, err = h.ctx.Store.CreateUser(r.Context(), req.Email, hashedPassword)
			if err != nil {
				slog.Error("Failed to create user", "error", err)
				http.Error(w, "Failed to create user", http.StatusInternalServerError)
				return
			}
			isNewUser = true

			// Generate invite token (reusing password reset token mechanism)
			inviteToken, err = h.ctx.Store.CreatePasswordResetToken(r.Context(), req.Email)
			if err != nil {
				slog.Error("Failed to create invite token", "error", err)
				// Continue anyway, user is created but won't get email.
				// They can use forgot password later.
			}
		} else {
			userID = user.ID
		}

		actorID := shared.GetUserIDFromContext(r)

		err = h.ctx.Store.AddSiteMember(r.Context(), siteID, userID, authcore.SiteRole(req.Role), actorID)
		if err != nil {
			slog.Error("Failed to add member", "error", err)
			http.Error(w, "Failed to add member", http.StatusInternalServerError)
			return
		}

		// Send invite email if new user
		if isNewUser && inviteToken != "" {
			// Get site details for email
			site, err := h.ctx.Store.GetSite(r.Context(), siteID, actorID)
			siteName := "Unknown Site"
			if err == nil && site != nil {
				siteName = site.Domain
			}

			// Get inviter details
			inviter, err := h.ctx.Store.GetUserByID(r.Context(), actorID)
			inviterName := "Someone"
			if err == nil && inviter != nil {
				inviterName = inviter.Email
			}

			inviteLink := h.ctx.Config.PublicURL + "/accept-invite?token=" + inviteToken
			err = h.ctx.Mailer.Send(req.Email, mailables.NewUserInvite(inviteLink, siteName, inviterName))
			if err != nil {
				slog.Warn("Failed to send invite email", "error", err, "email", req.Email)
				// Don't fail the request, just log warning
			}
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

func (h *handler) handleRemoveSiteMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		siteIDStr := r.PathValue("id")
		siteID, err := uuid.Parse(siteIDStr)
		if err != nil {
			http.Error(w, "Invalid site ID", http.StatusBadRequest)
			return
		}

		userIDStr := r.PathValue("userId")
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		actorID := shared.GetUserIDFromContext(r)

		// Can't remove yourself if you're the only owner
		if actorID == userID {
			role, _ := h.ctx.Store.GetSiteRole(r.Context(), userID, siteID)
			if role == authcore.SiteOwner {
				owners, _ := h.ctx.Store.CountSiteOwners(r.Context(), siteID)
				if owners <= 1 {
					http.Error(w, "Cannot remove the last owner", http.StatusBadRequest)
					return
				}
			}
		}

		err = h.ctx.Store.RemoveSiteMember(r.Context(), siteID, userID, actorID)
		if err != nil {
			slog.Error("Failed to remove member", "error", err)
			http.Error(w, "Failed to remove member", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}
