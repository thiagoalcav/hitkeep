package shared

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/nsqio/go-nsq"

	"hitkeep/internal/auth"
	"hitkeep/internal/cluster"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/mailer"
	"hitkeep/internal/takeout"
)

type contextKey string

const UserIDKey contextKey = "user_id"
const PermissionKey contextKey = "permissions"

type PermissionContext struct {
	UserID       uuid.UUID
	InstanceRole auth.InstanceRole
	SiteRole     auth.SiteRole // Only set if checking site permission.
}

type HandlerConfig struct {
	RequireAuth  bool
	InstancePerm auth.Permission
	SitePerm     auth.Permission
	RateLimiter  *IPRateLimiter
}

type Context struct {
	Store         *database.Store
	Cluster       *cluster.Manager
	Producer      *nsq.Producer
	Mailer        *mailer.Mailer
	Config        *config.Config
	Takeout       *takeout.TakeoutService
	IngestLimiter *IPRateLimiter
	ApiLimiter    *IPRateLimiter
	AuthLimiter   *IPRateLimiter
}

// GetUserIDFromContext extracts the user ID from context (set by auth middleware).
func GetUserIDFromContext(r *http.Request) uuid.UUID {
	// First check PermissionContext (new RBAC).
	if val := r.Context().Value(PermissionKey); val != nil {
		if perms, ok := val.(PermissionContext); ok {
			return perms.UserID
		}
	}

	// Fallback to legacy UserIDKey.
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

// Handler wraps common middleware patterns.
func (c *Context) Handler(config HandlerConfig, fn http.HandlerFunc) http.HandlerFunc {
	handler := fn

	// Apply site permission check if needed.
	if config.SitePerm != "" {
		handler = c.RequirePermission(config.SitePerm)(handler)
	}

	// Apply instance permission check if needed.
	if config.InstancePerm != "" {
		handler = c.RequirePermission(config.InstancePerm)(handler)
	}

	// Apply auth if needed.
	if config.RequireAuth || config.InstancePerm != "" || config.SitePerm != "" {
		handler = c.RequireAuth(handler)
	}

	// Apply rate limiting.
	if config.RateLimiter != nil {
		handler = c.WithRateLimit(config.RateLimiter, handler)
	}

	return handler
}

// RequirePermission checks if user has the required permission.
func (c *Context) RequirePermission(perm auth.Permission) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserIDFromContext(r)
			if userID == uuid.Nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Get instance role.
			instanceRole, err := c.Store.GetInstanceRole(r.Context(), userID)
			if err != nil {
				slog.Error("Failed to get instance role", "error", err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}

			// Check instance-level permission.
			if instanceRole.HasPermission(perm) {
				ctx := context.WithValue(r.Context(), PermissionKey, PermissionContext{
					UserID:       userID,
					InstanceRole: instanceRole,
				})
				next(w, r.WithContext(ctx))
				return
			}

			// For site-level permissions, check site role.
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

				siteRole, err := c.Store.GetSiteRole(r.Context(), userID, siteID)
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

// RequireAuth wraps a handler and ensures the user is authenticated.
// It sets the UserIDKey in the context.
func (c *Context) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userID uuid.UUID
		var err error

		// 1. Try to validate the short-lived JWT.
		cookie, err := r.Cookie(auth.CookieName)
		if err == nil {
			userID, err = auth.ValidateToken(cookie.Value, c.Config.JWTSecret, c.Config.PublicURL)
		}

		// 2. If JWT is missing or invalid, try the Remember Me token.
		if err != nil || userID == uuid.Nil {
			rememberCookie, err := r.Cookie(auth.RememberMeCookieName)
			if err == nil {
				userID, err = c.Store.ValidateRememberMeToken(r.Context(), rememberCookie.Value)
				if err == nil && userID != uuid.Nil {
					// Valid remember me token! Issue a new JWT.
					newToken, err := auth.GenerateToken(c.Config.JWTSecret, c.Config.PublicURL, userID)
					if err == nil {
						isSecure := strings.HasPrefix(c.Config.PublicURL, "https://")
						auth.SetTokenCookie(w, newToken, isSecure)
					}
				}
			}
		}

		if userID == uuid.Nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next(w, r.WithContext(ctx))
	}
}

// WithRateLimit wraps a handler with IP-based rate limiting.
func (c *Context) WithRateLimit(limiter *IPRateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := GetRealIP(r, c.Config.GetTrustedProxyNetworks())
		l := limiter.GetLimiter(ip)
		if !l.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
