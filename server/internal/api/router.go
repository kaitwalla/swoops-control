package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
	"github.com/kaitwalla/swoops-control/server/internal/config"
	"github.com/kaitwalla/swoops-control/server/internal/frontend"
	"github.com/kaitwalla/swoops-control/server/internal/metrics"
	"github.com/kaitwalla/swoops-control/server/internal/sessionmgr"
	"github.com/kaitwalla/swoops-control/server/internal/store"
)

type Server struct {
	store      *store.Store
	config     *config.Config
	configMu   sync.RWMutex // Protects concurrent access to config
	sessionMgr *sessionmgr.Manager
	agentOut   AgentOutputSource
	wsUpgrader websocket.Upgrader
	router     chi.Router
	waf        *WAFMiddleware

	// launchFunc is called asynchronously after session creation.
	// Defaults to sessionMgr.LaunchSession. Override in tests to disable SSH.
	launchFunc func(sessionID, hostID string) error
}

func NewServer(s *store.Store, cfg *config.Config) *Server {
	mgr := sessionmgr.New(s)
	mgr.SetConfig(cfg) // Pass config for MCP config generation
	srv := &Server{store: s, config: cfg, sessionMgr: mgr}
	srv.launchFunc = mgr.LaunchSession
	srv.setupRoutes()
	return srv
}

type AgentOutputSource interface {
	SubscribeSessionOutput(sessionID string) chan string
	UnsubscribeSessionOutput(sessionID string, ch chan string)
}

func (s *Server) SetAgentOutputSource(src AgentOutputSource) {
	s.agentOut = src
}

func (s *Server) SetAgentController(controller sessionmgr.AgentController) {
	if s.sessionMgr != nil {
		s.sessionMgr.SetAgentController(controller)
	}
}

func (s *Server) setupRoutes() {
	r := chi.NewRouter()

	// WAF should be first to block malicious requests early
	if s.config.WAF.Enabled {
		wafConfig := WAFConfig{
			RateLimitEnabled:   s.config.WAF.RateLimitEnabled,
			RequestsPerMinute:  s.config.WAF.RequestsPerMinute,
			BurstSize:          s.config.WAF.BurstSize,
			FilterEnabled:      s.config.WAF.FilterEnabled,
			MaxRequestBodySize: s.config.WAF.MaxRequestBodySize,
			BlockSuspiciousUA:  s.config.WAF.BlockSuspiciousUA,
			LogBlockedRequests: s.config.WAF.LogBlockedRequests,
			BlockedIPs:         s.config.WAF.BlockedIPs,
			AllowedIPs:         s.config.WAF.AllowedIPs,
		}
		s.waf = NewWAFMiddleware(wafConfig, slog.Default())
		r.Use(s.waf.Middleware)
	}

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(metrics.HTTPMiddleware)

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

	// Prometheus metrics endpoint (unauthenticated for scraping)
	r.Handle("/metrics", metrics.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		// Health check, version, and CA cert are unauthenticated
		r.Get("/health", s.handleHealth)
		r.Get("/version", s.handleVersion)
		r.Get("/ca-cert", s.handleGetCACert)

		// Auth endpoints (unauthenticated)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/logout", s.handleLogout)

		// Client cert download (uses auth token for authentication, POST to avoid token in URL)
		r.Post("/hosts/{id}/client-cert", s.handleGetClientCert)

		// All other API routes require authentication
		r.Group(func(r chi.Router) {
			r.Use(s.HybridAuth())

			r.Get("/stats", s.handleStats)
			r.Get("/server-info", s.handleGetServerInfo)

			// User management (current user - available to all authenticated users)
			r.Get("/auth/me", s.handleGetCurrentUser)

			// Admin-only endpoints
			r.Group(func(r chi.Router) {
				r.Use(s.RequireAdmin())

				// WAF management endpoints
				r.Get("/waf/stats", s.handleWAFStats)
				r.Get("/waf/config", s.handleGetWAFConfig)
				r.Put("/waf/config", s.handleUpdateWAFConfig)
				r.Post("/waf/block-ip", s.handleBlockIP)
				r.Post("/waf/unblock-ip", s.handleUnblockIP)

				// User management
				r.Get("/users", s.handleListUsers)
				r.Post("/users", s.handleCreateUser)
			})

			r.Route("/hosts", func(r chi.Router) {
				r.Get("/", s.handleListHosts)
				r.Post("/", s.handleCreateHost)
				r.Post("/agent", s.handleCreateAgentHost)
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

					// MCP endpoints
					r.Post("/status", s.handleReportStatus)
					r.Get("/status", s.handleListStatusUpdates)
					r.Post("/tasks", s.handleCreateTask)
					r.Get("/tasks", s.handleListTasks)
					r.Get("/tasks/next", s.handleGetNextTask)
					r.Post("/reviews", s.handleCreateReviewRequest)
					r.Post("/messages", s.handleCreateSessionMessage)
					r.Get("/messages", s.handleListSessionMessages)
				})
			})

			// Global endpoints (not session-specific)
			r.Get("/reviews", s.handleListReviewRequests)
			r.Route("/reviews/{review_id}", func(r chi.Router) {
				r.Get("/", s.handleGetReviewRequest)
				r.Put("/", s.handleUpdateReviewRequest)
			})
			r.Put("/tasks/{task_id}", s.handleUpdateTaskStatus)
			r.Put("/messages/{message_id}/read", s.handleMarkMessageRead)

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
