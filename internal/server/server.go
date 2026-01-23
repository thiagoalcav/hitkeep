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
	"hitkeep/internal/mailer"
	"hitkeep/internal/server/admin"
	serverauth "hitkeep/internal/server/auth"
	"hitkeep/internal/server/goals"
	"hitkeep/internal/server/ingest"
	"hitkeep/internal/server/permissions"
	"hitkeep/internal/server/shared"
	"hitkeep/internal/server/sites"
	"hitkeep/internal/server/system"
	takeouthandlers "hitkeep/internal/server/takeout"
	"hitkeep/internal/server/user"
	"hitkeep/internal/takeout"
)

type Server struct {
	httpServer    *http.Server
	store         *database.Store
	cluster       *cluster.Manager
	producer      *nsq.Producer
	mailer        *mailer.Mailer
	conf          *config.Config
	ingestLimiter *shared.IPRateLimiter
	apiLimiter    *shared.IPRateLimiter
	authLimiter   *shared.IPRateLimiter
	takeout       *takeout.TakeoutService
	ctx           *shared.Context
}

func New(conf *config.Config, publicFS fs.FS, store *database.Store, cluster *cluster.Manager, producer *nsq.Producer) *Server {
	ingestLim := shared.NewIPRateLimiter(rate.Limit(conf.IngestRateLimit), conf.IngestBurst)
	apiLim := shared.NewIPRateLimiter(rate.Limit(conf.ApiRateLimit), conf.ApiBurst)
	authLim := shared.NewIPRateLimiter(rate.Limit(conf.AuthRateLimit), conf.AuthBurst)
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
	s.ctx = &shared.Context{
		Store:         store,
		Cluster:       cluster,
		Producer:      producer,
		Mailer:        mailService,
		Config:        conf,
		Takeout:       takeoutService,
		IngestLimiter: ingestLim,
		ApiLimiter:    apiLim,
		AuthLimiter:   authLim,
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
	ctx := s.ctx
	system.Register(mux, ctx)
	ingest.Register(mux, ctx)
	serverauth.Register(mux, ctx)
	user.Register(mux, ctx)
	permissions.Register(mux, ctx)
	admin.Register(mux, ctx)
	sites.Register(mux, ctx)
	goals.Register(mux, ctx)
	takeouthandlers.Register(mux, ctx)

	// Static
	mux.Handle("/", s.spaHandler(publicFS))
}

func (s *Server) spaHandler(publicFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(publicFS))

	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		if strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".svg") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		if path == "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		f, err := publicFS.Open(path)
		if os.IsNotExist(err) {
			if strings.HasPrefix(path, "api/") || strings.HasPrefix(path, "ingest") {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
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
