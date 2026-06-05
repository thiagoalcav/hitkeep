package shared

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	hitai "hitkeep/internal/ai"
	"hitkeep/internal/auth"
	"hitkeep/internal/blocking"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/entitlements"
	"hitkeep/internal/mailer"
	"hitkeep/internal/realtime"
	"hitkeep/internal/searchconsole"
	"hitkeep/internal/takeout"
)

type contextKey string

const UserIDKey contextKey = "user_id"
const PermissionKey contextKey = "permissions"
const APIClientAuthKey contextKey = "api_client_auth"
const AuthSessionKey contextKey = "auth_session"

type PermissionContext struct {
	UserID       uuid.UUID
	InstanceRole auth.InstanceRole
	SiteRole     auth.SiteRole // Only set if checking site permission.
}

type AuthSessionContext struct {
	ExpiresAt time.Time
	IssuedAt  time.Time
}

type HandlerConfig struct {
	RequireAuth   bool
	InstancePerm  auth.Permission
	SitePerm      auth.Permission
	TeamCap       auth.Capability
	AllowAPIKey   bool
	APIClientOnly bool
	RateLimiter   *IPRateLimiter
}

type MessageProducer interface {
	Publish(topic string, body []byte) error
	Ping() error
}

type ClusterState interface {
	IsLeader() bool
	GetLeaderAddr() string
}

type Context struct {
	Store          *database.Store
	TenantStores   *database.TenantStoreManager
	Cluster        ClusterState
	Producer       MessageProducer
	Mailer         *mailer.Mailer
	Config         *config.Config
	Takeout        *takeout.TakeoutService
	Entitlements   entitlements.Provider
	IngestLimiter  *IPRateLimiter
	ApiLimiter     *IPRateLimiter
	AuthLimiter    *IPRateLimiter
	WebhookLimiter *IPRateLimiter
	AuthState      *AuthStateStore
	SearchConsole  searchconsole.Client
	AI             hitai.Client
	Realtime       *realtime.Broker
	IPFilter       *blocking.IPFilter
	SpamFilter     *blocking.SpamFilter

	// Runtime system monitoring
	StartedAt                time.Time
	SystemCounters           *database.SystemCounter
	BackupStatus             *database.BackupStatusTracker
	ImportStageCleanupStatus *database.ImportStageCleanupStatusTracker
	MailTestTracker          *database.MailTestTracker
}

// AnalyticsStore resolves the tenant-specific store that holds analytics data for the given site.
// It falls back to the shared store if TenantStores is nil (single-tenant / follower node).
func (c *Context) AnalyticsStore(ctx context.Context, siteID uuid.UUID) (*database.Store, error) {
	if c.TenantStores == nil {
		return c.Store, nil
	}

	store, _, err := c.TenantStores.ResolveSiteStore(ctx, siteID)
	if err != nil {
		return nil, fmt.Errorf("resolve analytics store for site %s: %w", siteID, err)
	}

	return store, nil
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
	handler := c.applyAccessChecks(config, fn)
	handler = c.applyAuthentication(config, handler)

	// Apply rate limiting.
	if config.RateLimiter != nil {
		handler = c.WithRateLimit(config.RateLimiter, handler)
	}

	return handler
}

func (c *Context) applyAccessChecks(config HandlerConfig, handler http.HandlerFunc) http.HandlerFunc {
	if config.SitePerm != "" {
		handler = c.RequirePermission(config.SitePerm)(handler)
	}

	if config.TeamCap != "" {
		handler = c.RequireTeamCapability(config.TeamCap)(handler)
	}

	// Apply instance permission check if needed.
	if config.InstancePerm != "" {
		handler = c.RequirePermission(config.InstancePerm)(handler)
	}

	return handler
}

func (c *Context) applyAuthentication(config HandlerConfig, handler http.HandlerFunc) http.HandlerFunc {
	if config.APIClientOnly {
		return c.RequireAPIClientAuth(handler)
	}
	if config.requiresUserAuth() {
		return c.RequireAuth(config.allowsAPIKey(), handler)
	}
	return handler
}

func (config HandlerConfig) requiresUserAuth() bool {
	return config.RequireAuth || config.InstancePerm != "" || config.SitePerm != "" || config.TeamCap != ""
}

func (config HandlerConfig) allowsAPIKey() bool {
	return config.AllowAPIKey || config.InstancePerm != "" || config.SitePerm != ""
}

// RequirePermission checks if user has the required permission.
func (c *Context) RequirePermission(perm auth.Permission) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserIDFromContext(r)
			apiClientAuth, _ := r.Context().Value(APIClientAuthKey).(*database.APIClientAuth)
			if userID == uuid.Nil && apiClientAuth == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			instanceRole, err := c.resolveInstanceRole(r.Context(), userID, apiClientAuth)
			if err != nil {
				slog.Error("Failed to get instance role", "error", err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}

			// API clients need an explicit site grant for every site-scoped route.
			if isSitePermission(perm) && apiClientAuth != nil {
				siteID, err := siteIDFromRequest(r)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				siteRole, err := c.resolveSiteRole(r.Context(), userID, apiClientAuth, siteID)
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

			// Check instance-level permission for human sessions and instance-scoped API routes.
			if instanceRole.HasPermission(perm) {
				ctx := context.WithValue(r.Context(), PermissionKey, PermissionContext{
					UserID:       userID,
					InstanceRole: instanceRole,
				})
				next(w, r.WithContext(ctx))
				return
			}

			// For site-level human-session permissions, check site role after instance role.
			if isSitePermission(perm) {
				siteID, err := siteIDFromRequest(r)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				siteRole, err := c.resolveSiteRole(r.Context(), userID, apiClientAuth, siteID)
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

func (c *Context) RequireTeamCapability(capability auth.Capability) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserIDFromContext(r)
			if userID == uuid.Nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			teamID, ok := teamIDFromRequest(r, w)
			if !ok {
				return
			}

			if !c.userHasTeamCapability(r.Context(), teamID, userID, capability) {
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}

func teamIDFromRequest(r *http.Request, w http.ResponseWriter) (uuid.UUID, bool) {
	teamID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		http.Error(w, "Invalid team ID", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return teamID, true
}

func (c *Context) userHasTeamCapability(ctx context.Context, teamID, userID uuid.UUID, capability auth.Capability) bool {
	role, err := c.Store.GetTenantRole(ctx, teamID, userID)
	return err == nil && auth.TeamRoleHasCapability(role, capability)
}

func (c *Context) sitePermissionContext(
	ctx context.Context,
	userID uuid.UUID,
	apiClientAuth *database.APIClientAuth,
	instanceRole auth.InstanceRole,
	siteID uuid.UUID,
	perm auth.Permission,
) (context.Context, bool, error) {
	siteRole, err := c.resolveSiteRole(ctx, userID, apiClientAuth, siteID)
	if err != nil {
		return nil, false, err
	}
	if !siteRole.HasPermission(perm) {
		return nil, false, nil
	}
	return context.WithValue(ctx, PermissionKey, PermissionContext{
		UserID:       userID,
		InstanceRole: instanceRole,
		SiteRole:     siteRole,
	}), true, nil
}

// RequireSiteOrInstancePermission allows route-specific exceptions where a site
// permission can also be satisfied by a narrow instance-level permission.
func (c *Context) RequireSiteOrInstancePermission(sitePerm, instancePerm auth.Permission) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserIDFromContext(r)
			apiClientAuth, _ := r.Context().Value(APIClientAuthKey).(*database.APIClientAuth)
			if userID == uuid.Nil && apiClientAuth == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			instanceRole, err := c.resolveInstanceRole(r.Context(), userID, apiClientAuth)
			if err != nil {
				slog.Error("Failed to get instance role", "error", err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}

			if sitePerm != "" {
				siteID, err := siteIDFromRequest(r)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				siteCtx, ok, err := c.sitePermissionContext(r.Context(), userID, apiClientAuth, instanceRole, siteID, sitePerm)
				if err != nil {
					if apiClientAuth != nil {
						http.Error(w, "Access denied", http.StatusForbidden)
						return
					}
				}
				if ok {
					next(w, r.WithContext(siteCtx))
					return
				}
				if apiClientAuth != nil {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
			}

			if instancePerm != "" && instanceRole.HasPermission(instancePerm) {
				ctx := context.WithValue(r.Context(), PermissionKey, PermissionContext{
					UserID:       userID,
					InstanceRole: instanceRole,
				})
				next(w, r.WithContext(ctx))
				return
			}

			http.Error(w, "Forbidden", http.StatusForbidden)
		}
	}
}

func isSitePermission(perm auth.Permission) bool {
	return strings.HasPrefix(string(perm), "site.")
}

func (c *Context) resolveInstanceRole(ctx context.Context, userID uuid.UUID, apiClientAuth *database.APIClientAuth) (auth.InstanceRole, error) {
	instanceRole := auth.InstanceUser
	if userID != uuid.Nil {
		var err error
		instanceRole, err = c.Store.GetInstanceRole(ctx, userID)
		if err != nil {
			return auth.InstanceUser, err
		}
	}
	if apiClientAuth != nil {
		instanceRole = auth.MinInstanceRole(instanceRole, apiClientAuth.InstanceRole)
	}
	return instanceRole, nil
}

func (c *Context) resolveSiteRole(ctx context.Context, userID uuid.UUID, apiClientAuth *database.APIClientAuth, siteID uuid.UUID) (auth.SiteRole, error) {
	var siteRole auth.SiteRole
	if userID != uuid.Nil {
		var err error
		siteRole, err = c.Store.GetSiteRole(ctx, userID, siteID)
		if err != nil {
			return "", err
		}
	}

	if apiClientAuth != nil {
		delegatedRole, ok := apiClientAuth.SiteRoles[siteID]
		if !ok {
			return "", fmt.Errorf("api client is not allowed for site %s", siteID)
		}
		if userID == uuid.Nil {
			siteRole = delegatedRole
		} else {
			siteRole = auth.MinSiteRole(siteRole, delegatedRole)
		}
	}

	return siteRole, nil
}

func siteIDFromRequest(r *http.Request) (uuid.UUID, error) {
	siteIDStr := r.PathValue("id")
	if siteIDStr == "" {
		return uuid.Nil, fmt.Errorf("site ID required")
	}

	siteID, err := uuid.Parse(siteIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid site ID")
	}

	return siteID, nil
}

// RequireAPIClientAuth wraps a handler and accepts only API client tokens.
func (c *Context) RequireAPIClientAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractAPIClientToken(r)
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		apiClientAuth, err := c.Store.GetAPIClientAuth(r.Context(), token)
		if err != nil {
			slog.Error("Failed to validate api client token", "error", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if apiClientAuth == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, apiClientAuth.UserID)
		ctx = context.WithValue(ctx, APIClientAuthKey, apiClientAuth)
		next(w, r.WithContext(ctx))
	}
}

// RequireAuth wraps a handler and ensures the user is authenticated.
// It sets the UserIDKey in the context.
func (c *Context) RequireAuth(allowAPIKey bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userID uuid.UUID
		var err error
		var apiClientAuth *database.APIClientAuth
		var sessionCtx *AuthSessionContext

		// 1. Try to validate the short-lived JWT.
		cookie, err := r.Cookie(auth.CookieName)
		if err == nil {
			var claims *auth.Claims
			claims, err = auth.ValidateTokenClaims(cookie.Value, c.Config.JWTSecret, c.Config.PublicURL)
			if err == nil {
				userID = claims.UserID
				sessionCtx = authSessionContextFromClaims(claims)
			}
		}

		// 2. If JWT is missing or invalid, try the Remember Me token.
		if err != nil || userID == uuid.Nil {
			rememberCookie, err := r.Cookie(auth.RememberMeCookieName)
			if err == nil {
				userID, err = c.Store.ValidateRememberMeToken(r.Context(), rememberCookie.Value)
				if err == nil && userID != uuid.Nil {
					// Valid remember me token! Issue a new JWT.
					duration := c.Config.AuthSessionDuration()
					newToken, expiresAt, err := auth.GenerateTokenWithDuration(c.Config.JWTSecret, c.Config.PublicURL, userID, duration)
					if err == nil {
						isSecure := strings.HasPrefix(c.Config.PublicURL, "https://")
						auth.SetTokenCookieWithDuration(w, newToken, isSecure, duration)
						sessionCtx = &AuthSessionContext{ExpiresAt: expiresAt.UTC(), IssuedAt: time.Now().UTC()}
					}
				}
			}
		}

		if allowAPIKey && userID == uuid.Nil {
			token := extractAPIClientToken(r)
			if token != "" {
				apiClientAuth, err = c.Store.GetAPIClientAuth(r.Context(), token)
				if err != nil {
					slog.Error("Failed to validate api client token", "error", err)
				} else if apiClientAuth != nil {
					userID = apiClientAuth.UserID
				}
			}
		}

		if userID == uuid.Nil && apiClientAuth == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		if apiClientAuth != nil {
			ctx = context.WithValue(ctx, APIClientAuthKey, apiClientAuth)
		}
		if sessionCtx != nil {
			ctx = context.WithValue(ctx, AuthSessionKey, *sessionCtx)
		}
		next(w, r.WithContext(ctx))
	}
}

func authSessionContextFromClaims(claims *auth.Claims) *AuthSessionContext {
	if claims == nil || claims.ExpiresAt == nil {
		return nil
	}

	session := &AuthSessionContext{ExpiresAt: claims.ExpiresAt.UTC()}
	if claims.IssuedAt != nil {
		session.IssuedAt = claims.IssuedAt.UTC()
	}
	return session
}

func extractAPIClientToken(r *http.Request) string {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if authorization != "" {
		parts := strings.SplitN(authorization, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token := strings.TrimSpace(parts[1])
			if token != "" {
				return token
			}
		}
	}

	return strings.TrimSpace(r.Header.Get("X-Api-Key"))
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
