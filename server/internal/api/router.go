package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/swoopsh/swoops/server/internal/config"
	"github.com/swoopsh/swoops/server/internal/frontend"
	"github.com/swoopsh/swoops/server/internal/store"
)

type Server struct {
	store  *store.Store
	config *config.Config
	router chi.Router
}

func NewServer(s *store.Store, cfg *config.Config) *Server {
	srv := &Server{store: s, config: cfg}
	srv.setupRoutes()
	return srv
}

func (s *Server) setupRoutes() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// CORS: use configured origins, or same-origin only (no wildcard with credentials)
	allowedOrigins := s.config.Server.AllowedOrigins
	if len(allowedOrigins) == 0 {
		// Default: same-origin only — construct from listen address
		allowedOrigins = []string{
			fmt.Sprintf("http://localhost:%d", s.config.Server.Port),
			fmt.Sprintf("http://127.0.0.1:%d", s.config.Server.Port),
		}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api/v1", func(r chi.Router) {
		// Health check is unauthenticated
		r.Get("/health", s.handleHealth)

		// All other API routes require authentication
		r.Group(func(r chi.Router) {
			r.Use(APIKeyAuth(s.config.Auth.APIKey))

			r.Get("/stats", s.handleStats)

			r.Route("/hosts", func(r chi.Router) {
				r.Get("/", s.handleListHosts)
				r.Post("/", s.handleCreateHost)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", s.handleGetHost)
					r.Put("/", s.handleUpdateHost)
					r.Delete("/", s.handleDeleteHost)
					r.Get("/sessions", s.handleListHostSessions)
				})
			})

			r.Route("/sessions", func(r chi.Router) {
				r.Get("/", s.handleListSessions)
				r.Post("/", s.handleCreateSession)
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", s.handleGetSession)
					r.Delete("/", s.handleDeleteSession)
					r.Post("/stop", s.handleStopSession)
					r.Post("/input", s.handleSendInput)
					r.Get("/output", s.handleGetOutput)
				})
			})
		})
	})

	// Serve embedded frontend for non-API routes
	r.Handle("/*", frontend.Handler())

	s.router = r
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
