package server

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nsqio/go-nsq"
	"golang.org/x/time/rate"

	"hitkeep/internal/auth"
	"hitkeep/internal/cluster"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
	"hitkeep/internal/mailer"
	"hitkeep/internal/takeout"
)

type contextKey string

const UserIDKey contextKey = "user_id"

type Server struct {
	httpServer    *http.Server
	store         *database.Store
	cluster       *cluster.Manager
	producer      *nsq.Producer
	mailer        *mailer.Mailer
	conf          *config.Config
	ingestLimiter *IPRateLimiter
	apiLimiter    *IPRateLimiter
	authLimiter   *IPRateLimiter
	takeout       *takeout.TakeoutService
}

func New(conf *config.Config, publicFS fs.FS, store *database.Store, cluster *cluster.Manager, producer *nsq.Producer) *Server {
	ingestLim := NewIPRateLimiter(rate.Limit(conf.IngestRateLimit), conf.IngestBurst)
	apiLim := NewIPRateLimiter(rate.Limit(conf.ApiRateLimit), conf.ApiBurst)
	authLim := NewIPRateLimiter(rate.Limit(conf.AuthRateLimit), conf.AuthBurst)
	mailService, err := mailer.New(conf)
	if err != nil {
		slog.Warn("Failed to initialize mailer. Email features will not work.", "error", err)
	}

	takeoutService := takeout.NewTakeoutService(store, "archive/takeout")

	s := &Server{
		store:         store,
		cluster:       cluster,
		producer:      producer,
		mailer:        mailService,
		conf:          conf,
		ingestLimiter: ingestLim,
		apiLimiter:    apiLim,
		authLimiter:   authLim,
		takeout:       takeoutService,
	}

	mux := http.NewServeMux()
	s.setupRoutes(mux, publicFS)

	s.httpServer = &http.Server{
		Addr:              conf.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) ListenAndServe() error {
	slog.Info("HTTP server starting", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("HTTP server shutting down.")

	s.ingestLimiter.Stop()
	s.apiLimiter.Stop()
	s.authLimiter.Stop()

	return s.httpServer.Shutdown(ctx)
}

func (s *Server) setupRoutes(mux *http.ServeMux, publicFS fs.FS) {
	mux.HandleFunc("GET /healthz", s.handleHealthz())
	mux.HandleFunc("GET /api/status", s.handleGetStatus())

	// Ingest (rate limited, no auth)
	mux.HandleFunc("POST /ingest", s.Handler(HandlerConfig{
		RateLimiter: s.ingestLimiter,
	}, s.handleIngest()))

	mux.HandleFunc("OPTIONS /ingest", s.Handler(HandlerConfig{
		RateLimiter: s.ingestLimiter,
	}, s.handleIngest()))

	mux.HandleFunc("POST /ingest/event", s.Handler(HandlerConfig{
		RateLimiter: s.ingestLimiter,
	}, s.handleIngestEvent()))

	mux.HandleFunc("OPTIONS /ingest/event", s.Handler(HandlerConfig{
		RateLimiter: s.ingestLimiter,
	}, s.handleIngestEvent()))

	// Auth endpoints (rate limited, no auth required)
	mux.HandleFunc("POST /api/initial-user", s.Handler(HandlerConfig{
		RateLimiter: s.authLimiter,
	}, s.handleCreateInitialUser()))

	mux.HandleFunc("POST /api/login", s.Handler(HandlerConfig{
		RateLimiter: s.authLimiter,
	}, s.handleLogin()))

	mux.HandleFunc("POST /api/logout", s.Handler(HandlerConfig{
		RateLimiter: s.authLimiter,
	}, s.handleLogout()))

	mux.HandleFunc("POST /api/auth/forgot-password", s.Handler(HandlerConfig{
		RateLimiter: s.authLimiter,
	}, s.handleForgotPassword()))

	mux.HandleFunc("POST /api/auth/reset-password", s.Handler(HandlerConfig{
		RateLimiter: s.authLimiter,
	}, s.handleResetPassword()))

	mux.HandleFunc("POST /api/auth/accept-invite", s.Handler(HandlerConfig{
		RateLimiter: s.authLimiter,
	}, s.handleAcceptInvite()))

	// User endpoints (auth required)
	mux.HandleFunc("POST /api/user/password", s.Handler(HandlerConfig{
		RequireAuth: true,
		RateLimiter: s.authLimiter,
	}, s.handleChangePassword()))

	mux.HandleFunc("GET /api/user/permissions", s.Handler(HandlerConfig{
		RequireAuth: true,
		RateLimiter: s.apiLimiter,
	}, s.handleGetUserPermissions()))

	// Instance Admin Routes
	mux.HandleFunc("GET /api/admin/users", s.Handler(HandlerConfig{
		InstancePerm: auth.PermInstanceManageUsers,
		RateLimiter:  s.apiLimiter,
	}, s.handleListUsers()))

	mux.HandleFunc("POST /api/admin/users/{id}/role", s.Handler(HandlerConfig{
		InstancePerm: auth.PermInstanceManageUsers,
		RateLimiter:  s.apiLimiter,
	}, s.handleUpdateUserRole()))

	mux.HandleFunc("DELETE /api/admin/users/{id}", s.Handler(HandlerConfig{
		InstancePerm: auth.PermInstanceManageUsers,
		RateLimiter:  s.apiLimiter,
	}, s.handleDeleteUser()))

	mux.HandleFunc("GET /api/admin/sites", s.Handler(HandlerConfig{
		InstancePerm: auth.PermInstanceManageUsers,
		RateLimiter:  s.apiLimiter,
	}, s.handleAdminListSites()))

	mux.HandleFunc("DELETE /api/admin/sites/{id}", s.Handler(HandlerConfig{
		InstancePerm: auth.PermInstanceManageUsers,
		RateLimiter:  s.apiLimiter,
	}, s.handleAdminDeleteSite()))

	// Site Management (basic auth)
	mux.HandleFunc("GET /api/sites", s.Handler(HandlerConfig{
		RequireAuth: true,
		RateLimiter: s.apiLimiter,
	}, s.handleGetSites()))

	mux.HandleFunc("POST /api/sites", s.Handler(HandlerConfig{
		RequireAuth: true,
		RateLimiter: s.apiLimiter,
	}, s.handleCreateSite()))

	// Site Team Management
	mux.HandleFunc("GET /api/sites/{id}/members", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteView,
		RateLimiter: s.apiLimiter,
	}, s.handleGetSiteMembers()))

	mux.HandleFunc("POST /api/sites/{id}/members", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteManageTeam,
		RateLimiter: s.apiLimiter,
	}, s.handleAddSiteMember()))

	mux.HandleFunc("DELETE /api/sites/{id}/members/{userId}", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteManageTeam,
		RateLimiter: s.apiLimiter,
	}, s.handleRemoveSiteMember()))

	// Site Analytics & Data
	mux.HandleFunc("GET /api/sites/{id}/stats", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteView,
		RateLimiter: s.apiLimiter,
	}, s.handleGetSiteStats()))

	mux.HandleFunc("GET /api/sites/{id}/hits", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteView,
		RateLimiter: s.apiLimiter,
	}, s.handleGetSiteHits()))

	mux.HandleFunc("GET /api/sites/{id}/favicon", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteView,
		RateLimiter: s.apiLimiter,
	}, s.handleGetSiteFavicon()))

	mux.HandleFunc("PUT /api/sites/{id}/retention", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteManageData,
		RateLimiter: s.apiLimiter,
	}, s.handleUpdateSiteRetention()))

	// Goals
	mux.HandleFunc("GET /api/sites/{id}/goals", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteView,
		RateLimiter: s.apiLimiter,
	}, s.handleGetGoals()))

	mux.HandleFunc("POST /api/sites/{id}/goals", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteManageGoals,
		RateLimiter: s.apiLimiter,
	}, s.handleCreateGoal()))

	mux.HandleFunc("DELETE /api/sites/{id}/goals/{goalID}", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteManageGoals,
		RateLimiter: s.apiLimiter,
	}, s.handleDeleteGoal()))

	// Funnels
	mux.HandleFunc("GET /api/sites/{id}/funnels", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteView,
		RateLimiter: s.apiLimiter,
	}, s.handleGetFunnels()))

	mux.HandleFunc("POST /api/sites/{id}/funnels", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteManageGoals,
		RateLimiter: s.apiLimiter,
	}, s.handleCreateFunnel()))

	mux.HandleFunc("DELETE /api/sites/{id}/funnels/{funnelID}", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteManageGoals,
		RateLimiter: s.apiLimiter,
	}, s.handleDeleteFunnel()))

	mux.HandleFunc("GET /api/sites/{id}/funnels/{funnelID}/stats", s.Handler(HandlerConfig{
		SitePerm:    auth.PermSiteView,
		RateLimiter: s.apiLimiter,
	}, s.handleGetFunnelStats()))

	// Takeout
	takeoutHandler := NewTakeoutHandler(s.takeout)
	mux.HandleFunc("GET /api/user/takeout", s.withRateLimit(s.apiLimiter, s.requireAuth(takeoutHandler.handleUserTakeout())))
	mux.HandleFunc("GET /api/sites/{id}/takeout", s.withRateLimit(s.apiLimiter, s.requireAuth(s.requirePermission(auth.PermSiteView)(takeoutHandler.handleSiteTakeout()))))

	// Static
	mux.Handle("/", s.spaHandler(publicFS))
}

// withRateLimit wraps a handler with IP-based rate limiting.
func (s *Server) withRateLimit(limiter *IPRateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := getRealIP(r, s.conf.GetTrustedProxyNetworks())
		l := limiter.GetLimiter(ip)
		if !l.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// requireAuth wraps a handler and ensures the user is authenticated.
// It sets the UserIDKey in the context.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var userID uuid.UUID
		var err error

		// 1. Try to validate the short-lived JWT
		cookie, err := r.Cookie(auth.CookieName)
		if err == nil {
			userID, err = auth.ValidateToken(cookie.Value, s.conf.JWTSecret, s.conf.PublicURL)
		}

		// 2. If JWT is missing or invalid, try the Remember Me token
		if err != nil || userID == uuid.Nil {
			rememberCookie, err := r.Cookie(auth.RememberMeCookieName)
			if err == nil {
				userID, err = s.store.ValidateRememberMeToken(r.Context(), rememberCookie.Value)
				if err == nil && userID != uuid.Nil {
					// Valid remember me token! Issue a new JWT.
					newToken, err := auth.GenerateToken(s.conf.JWTSecret, s.conf.PublicURL, userID)
					if err == nil {
						isSecure := strings.HasPrefix(s.conf.PublicURL, "https://")
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

func (s *Server) spaHandler(publicFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(publicFS))

	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		if path == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		f, err := publicFS.Open(path)
		if os.IsNotExist(err) {
			if strings.HasPrefix(path, "api/") || strings.HasPrefix(path, "ingest") {
				http.NotFound(w, r)
				return
			}
			r.URL.Path = "/"
		} else if err == nil {
			f.Close()
		}

		fileServer.ServeHTTP(w, r)
	}
}
