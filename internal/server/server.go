package server

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nsqio/go-nsq"
	"golang.org/x/time/rate"

	"hitkeep/internal/cluster"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
)

type Server struct {
	httpServer    *http.Server
	store         *database.Store
	cluster       *cluster.Manager
	producer      *nsq.Producer
	conf          *config.Config
	ingestLimiter *IPRateLimiter
	apiLimiter    *IPRateLimiter
	authLimiter   *IPRateLimiter
}

func New(conf *config.Config, publicFS fs.FS, store *database.Store, cluster *cluster.Manager, producer *nsq.Producer) *Server {
	ingestLim := NewIPRateLimiter(rate.Limit(conf.IngestRateLimit), conf.IngestBurst)
	apiLim := NewIPRateLimiter(rate.Limit(conf.ApiRateLimit), conf.ApiBurst)
	authLim := NewIPRateLimiter(rate.Limit(conf.AuthRateLimit), conf.AuthBurst)

	s := &Server{
		store:         store,
		cluster:       cluster,
		producer:      producer,
		conf:          conf,
		ingestLimiter: ingestLim,
		apiLimiter:    apiLim,
		authLimiter:   authLim,
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
	// System
	mux.HandleFunc("GET /healthz", s.handleHealthz())
	mux.HandleFunc("GET /api/status", s.handleGetStatus())

	// Ingest
	mux.HandleFunc("POST /ingest", s.withRateLimit(s.ingestLimiter, s.handleIngest()))
	mux.HandleFunc("OPTIONS /ingest", s.withRateLimit(s.ingestLimiter, s.handleIngest()))

	// Auth
	mux.HandleFunc("POST /api/initial-user", s.withRateLimit(s.authLimiter, s.handleCreateInitialUser()))
	mux.HandleFunc("POST /api/login", s.withRateLimit(s.authLimiter, s.handleLogin()))

	// API
	mux.HandleFunc("GET /api/sites", s.withRateLimit(s.apiLimiter, s.requireAuth(s.handleGetSites())))
	mux.HandleFunc("POST /api/sites", s.withRateLimit(s.apiLimiter, s.requireAuth(s.handleCreateSite())))
	mux.HandleFunc("GET /api/sites/{id}/stats", s.withRateLimit(s.apiLimiter, s.requireAuth(s.handleGetSiteStats())))
	mux.HandleFunc("GET /api/sites/{id}/hits", s.withRateLimit(s.apiLimiter, s.requireAuth(s.handleGetSiteHits())))

	// Static
	mux.Handle("/", s.spaHandler(publicFS))
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
