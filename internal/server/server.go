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

	"hitkeep/internal/cluster"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
)

type Server struct {
	httpServer *http.Server
	store      *database.Store
	cluster    *cluster.Manager
	producer   *nsq.Producer
	conf       *config.Config
}

func New(conf *config.Config, publicFS fs.FS, store *database.Store, cluster *cluster.Manager, producer *nsq.Producer) *Server {
	s := &Server{
		store:    store,
		cluster:  cluster,
		producer: producer,
		conf:     conf,
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
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) setupRoutes(mux *http.ServeMux, publicFS fs.FS) {
	// System
	mux.HandleFunc("/healthz", s.handleHealthz())
	mux.HandleFunc("/api/status", s.handleGetStatus())

	// Ingest
	mux.HandleFunc("/ingest", s.handleIngest())

	// Auth
	mux.HandleFunc("/api/initial-user", s.handleCreateInitialUser())
	mux.HandleFunc("/api/login", s.handleLogin())

	// Sites & Analytics
	mux.HandleFunc("GET /api/sites", s.requireAuth(s.handleGetSites()))
	mux.HandleFunc("POST /api/sites", s.requireAuth(s.handleCreateSite()))
	mux.HandleFunc("GET /api/sites/{id}/stats", s.requireAuth(s.handleGetSiteStats()))
	mux.HandleFunc("/api/hits", s.requireAuth(s.handleGetHits()))

	// SPA / Static
	mux.Handle("/", s.spaHandler(publicFS))
}

// spaHandler wraps the file server to support Single Page Application routing.
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
			// explicitly ignore healthz here so it falls through to 404 if not matched above
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
