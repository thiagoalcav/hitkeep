package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"hitkeep/internal/auth"
)

type PermissionContext struct {
	UserID       uuid.UUID
	InstanceRole auth.InstanceRole
	SiteRole     auth.SiteRole // Only set if checking site permission
}

const PermissionKey contextKey = "permissions"

type HandlerConfig struct {
	RequireAuth  bool
	InstancePerm auth.Permission
	SitePerm     auth.Permission
	RateLimiter  *IPRateLimiter
}

// Handler wraps common middleware patterns
func (s *Server) Handler(config HandlerConfig, fn http.HandlerFunc) http.HandlerFunc {
	handler := fn

	// Apply site permission check if needed
	if config.SitePerm != "" {
		handler = s.requirePermission(config.SitePerm)(handler)
	}

	// Apply instance permission check if needed
	if config.InstancePerm != "" {
		handler = s.requirePermission(config.InstancePerm)(handler)
	}

	// Apply auth if needed
	if config.RequireAuth || config.InstancePerm != "" || config.SitePerm != "" {
		handler = s.requireAuth(handler)
	}

	// Apply rate limiting
	if config.RateLimiter != nil {
		handler = s.withRateLimit(config.RateLimiter, handler)
	}

	return handler
}

// requirePermission checks if user has the required permission
func (s *Server) requirePermission(perm auth.Permission) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID := getUserIDFromContext(r)
			if userID == uuid.Nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Get instance role
			instanceRole, err := s.store.GetInstanceRole(r.Context(), userID)
			if err != nil {
				slog.Error("Failed to get instance role", "error", err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}

			// Check instance-level permission
			if instanceRole.HasPermission(perm) {
				ctx := context.WithValue(r.Context(), PermissionKey, PermissionContext{
					UserID:       userID,
					InstanceRole: instanceRole,
				})
				next(w, r.WithContext(ctx))
				return
			}

			// For site-level permissions, check site role
			if strings.HasPrefix(string(perm), "site.") {
				siteIDStr := r.PathValue("id")
				if siteIDStr == "" {
					http.Error(w, "Site ID required", http.StatusBadRequest)
					return
				}

				siteID, err := uuid.Parse(siteIDStr)
				if err != nil {
					http.Error(w, "Invalid site ID", http.StatusBadRequest)
					return
				}

				siteRole, err := s.store.GetSiteRole(r.Context(), userID, siteID)
				if err != nil {
					http.Error(w, "Access denied", http.StatusForbidden)
					return
				}

				if siteRole.HasPermission(perm) {
					ctx := context.WithValue(r.Context(), PermissionKey, PermissionContext{
						UserID:       userID,
						InstanceRole: instanceRole,
						SiteRole:     siteRole,
					})
					next(w, r.WithContext(ctx))
					return
				}
			}

			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	}
}
