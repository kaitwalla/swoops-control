package api

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
	"github.com/swoopsh/swoops/server/internal/config"
	"github.com/swoopsh/swoops/server/internal/frontend"
	"github.com/swoopsh/swoops/server/internal/sessionmgr"
	"github.com/swoopsh/swoops/server/internal/store"
)

type Server struct {
	store      *store.Store
	config     *config.Config
	sessionMgr *sessionmgr.Manager
	wsUpgrader websocket.Upgrader
	router     chi.Router

	// launchFunc is called asynchronously after session creation.
	// Defaults to sessionMgr.LaunchSession. Override in tests to disable SSH.
	launchFunc func(sessionID, hostID string) error
}

func NewServer(s *store.Store, cfg *config.Config) *Server {
	mgr := sessionmgr.New(s)
	srv := &Server{store: s, config: cfg, sessionMgr: mgr}
	srv.launchFunc = mgr.LaunchSession
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

	// WebSocket upgrader with origin allowlist matching CORS config
	s.wsUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		CheckOrigin:     buildOriginChecker(allowedOrigins),
	}

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

			// WebSocket endpoints
			r.Get("/ws/sessions/{id}/output", s.handleSessionOutputWS)
		})
	})

	// Serve embedded frontend for non-API routes
	r.Handle("/*", frontend.Handler())

	s.router = r
}

// buildOriginChecker returns a function that checks WebSocket upgrade requests
// against the configured allowed origins.
func buildOriginChecker(allowedOrigins []string) func(r *http.Request) bool {
	// Build set of allowed origin hosts for fast lookup
	allowed := make(map[string]bool)
	for _, origin := range allowedOrigins {
		if u, err := url.Parse(origin); err == nil {
			allowed[u.Host] = true
		}
	}

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// No Origin header — same-origin request (non-browser or same page)
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return allowed[u.Host]
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Close cleans up the server resources (session manager, SSH connections).
func (s *Server) Close() {
	if s.sessionMgr != nil {
		s.sessionMgr.Close()
	}
}
