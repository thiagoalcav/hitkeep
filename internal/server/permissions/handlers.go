package permissions

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	authcore "hitkeep/internal/auth"
	"hitkeep/internal/server/shared"
)

type handler struct {
	ctx *shared.Context
}

func Register(mux *http.ServeMux, ctx *shared.Context) {
	h := &handler{ctx: ctx}
	mux.HandleFunc("GET /api/user/permissions", ctx.Handler(shared.HandlerConfig{
		RequireAuth: true,
		RateLimiter: ctx.ApiLimiter,
	}, h.handleGetUserPermissions()))
}

func (h *handler) handleGetUserPermissions() http.HandlerFunc {
	type response struct {
		InstanceRole authcore.InstanceRole `json:"instance_role"`
		Permissions  []authcore.Permission `json:"permissions"`
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

		// Get instance role
		instanceRole, err := h.ctx.Store.GetInstanceRole(r.Context(), userID)
		if err != nil {
			//nolint:gosec // user_id is sourced from authenticated context; structured logging is intentional.
			slog.Error("Failed to get instance role", "error", err, "user_id", userID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Calculate permissions based on role
		// We can expose a helper in auth package to get permissions for a role,
		// or just manually construct it here if it's simple.
		// Looking at auth/permissions.go, the map is private (instancePermissions).
		// We should probably expose a way to get permissions for a role.
		// But for now, let's just return the role, and the frontend can infer permissions?
		// The task says "api/user/permissions", so it probably expects a list of permissions.

		// Let's check if we can access the permissions map or add a method to get them.
		// In auth/permissions.go:
		// func (r InstanceRole) HasPermission(perm Permission) bool
		// It doesn't expose the list.

		// However, for the frontend, it might be useful to know the role AND the permissions.
		// Since we can't easily list all permissions for a role without modifying auth package (which is fine),
		// let's modify auth package to expose GetPermissions() for a role.

		// Wait, I can't modify auth package in this step easily without context switching.
		// Let's see if I can just return the role for now, or if I should modify auth first.
		// The user asked to create the endpoint.
		// Let's modify auth/permissions.go to expose the permissions list first.
		// But I'll do that in a separate step if needed.
		// Actually, I can just iterate over all known permissions and check HasPermission.

		allInstancePermissions := []authcore.Permission{
			authcore.PermInstanceManageUsers,
			authcore.PermInstanceViewAllSites,
			authcore.PermInstanceManageSettings,
		}

		var perms []authcore.Permission
		for _, p := range allInstancePermissions {
			if instanceRole.HasPermission(p) {
				perms = append(perms, p)
			}
		}

		resp := response{
			InstanceRole: instanceRole,
			Permissions:  perms,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}
