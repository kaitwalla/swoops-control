package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
	"github.com/kaitwalla/swoops-control/pkg/version"
	"github.com/kaitwalla/swoops-control/server/internal/agentconn"
	"github.com/kaitwalla/swoops-control/server/internal/api"
	"github.com/kaitwalla/swoops-control/server/internal/config"
	"github.com/kaitwalla/swoops-control/server/internal/store"
	"golang.org/x/crypto/acme/autocert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	showVersion := flag.Bool("version", false, "show version information and exit")
	doUpdate := flag.Bool("update", false, "update swoopsd from git and rebuild")
	flag.Parse()

	if *showVersion {
		versionInfo := version.Get()
		fmt.Println(versionInfo.String())
		os.Exit(0)
	}

	if *doUpdate {
		if err := performUpdate(); err != nil {
			log.Fatalf("Update failed: %v", err)
		}
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	db, err := store.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	srv := api.NewServer(db, cfg)
	defer srv.Close()

	agentSvc := agentconn.NewService(db, cfg, logger)
	defer agentSvc.Close()
	srv.SetAgentOutputSource(agentSvc)
	srv.SetAgentController(agentSvc)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Configure autocert if enabled
	var autocertManager *autocert.Manager
	if cfg.Server.AutocertEnabled {
		// Create cache directory if it doesn't exist
		if err := os.MkdirAll(cfg.Server.AutocertCacheDir, 0700); err != nil {
			log.Fatalf("Failed to create autocert cache directory: %v", err)
		}

		autocertManager = &autocert.Manager{
			Prompt:      autocert.AcceptTOS,
			HostPolicy:  autocert.HostWhitelist(cfg.Server.AutocertDomain),
			Cache:       autocert.DirCache(cfg.Server.AutocertCacheDir),
		}
		if cfg.Server.AutocertEmail != "" {
			autocertManager.Email = cfg.Server.AutocertEmail
		}

		httpServer.TLSConfig = autocertManager.TLSConfig()
		log.Printf("Autocert configured for domain: %s (cache: %s)", cfg.Server.AutocertDomain, cfg.Server.AutocertCacheDir)
	}

	grpcAddr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen for gRPC on %s: %v", grpcAddr, err)
	}

	// Configure gRPC server with optional TLS and mTLS
	var grpcServer *grpc.Server
	if cfg.GRPC.Insecure {
		log.Printf("Warning: gRPC server running in INSECURE mode (no TLS)")
		grpcServer = grpc.NewServer()
	} else {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS13,
		}

		// Load server certificate
		cert, err := tls.LoadX509KeyPair(cfg.GRPC.TLSCert, cfg.GRPC.TLSKey)
		if err != nil {
			log.Fatalf("Failed to load server certificate: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}

		// Configure mTLS if enabled
		if cfg.GRPC.RequireMTLS {
			// Load CA certificate for client verification
			caCert, err := os.ReadFile(cfg.GRPC.ClientCA)
			if err != nil {
				log.Fatalf("Failed to read client CA certificate: %v", err)
			}

			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				log.Fatalf("Failed to parse client CA certificate")
			}

			tlsConfig.ClientCAs = certPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
			log.Printf("gRPC server configured with mTLS (server cert: %s, client CA: %s)", cfg.GRPC.TLSCert, cfg.GRPC.ClientCA)
		} else {
			log.Printf("gRPC server configured with TLS (cert: %s)", cfg.GRPC.TLSCert)
		}

		creds := credentials.NewTLS(tlsConfig)
		grpcServer = grpc.NewServer(grpc.Creds(creds))
	}
	agentrpc.RegisterAgentServiceServer(grpcServer, agentSvc)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Log version and check for updates
	versionInfo := version.Get()
	log.Printf("Starting %s", versionInfo.String())

	// Check for updates in background
	go func() {
		checkCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		updateInfo, err := version.CheckForUpdates(checkCtx, "anthropics", "swoops-control")
		if err != nil {
			log.Printf("version check: failed to check for updates: %v", err)
			return
		}
		if updateInfo.UpdateAvailable {
			log.Printf("⚠️  Update available: v%s → v%s", updateInfo.CurrentVersion, updateInfo.LatestVersion)
			log.Printf("   Download: %s", updateInfo.UpdateURL)
		}
	}()

	// Start HTTP redirect server for autocert (handles ACME challenges on port 80)
	var httpRedirectServer *http.Server
	if cfg.Server.AutocertEnabled {
		httpRedirectServer = &http.Server{
			Addr:    ":80",
			Handler: autocertManager.HTTPHandler(http.HandlerFunc(redirectToHTTPS)),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
		go func() {
			log.Printf("HTTP redirect server starting on :80 (for ACME challenges and HTTPS redirect)")
			if err := httpRedirectServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTP redirect server error: %v", err)
			}
		}()
	}

	// Start main HTTP(S) server
	go func() {
		if cfg.Server.AutocertEnabled {
			log.Printf("Swoops control plane starting on https://%s (automatic HTTPS via Let's Encrypt)", addr)
			if err := httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTPS server error: %v", err)
			}
		} else if cfg.Server.TLSEnabled {
			log.Printf("Swoops control plane starting on https://%s (manual TLS)", addr)
			if err := httpServer.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTPS server error: %v", err)
			}
		} else {
			log.Printf("Swoops control plane starting on http://%s (TLS disabled - development only)", addr)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server error: %v", err)
			}
		}
	}()

	go func() {
		log.Printf("Agent gRPC server starting on %s", grpcAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	if httpRedirectServer != nil {
		if err := httpRedirectServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP redirect server shutdown error: %v", err)
		}
	}
	grpcServer.GracefulStop()

	log.Println("Swoops control plane stopped")
}

// redirectToHTTPS redirects HTTP requests to HTTPS
func redirectToHTTPS(w http.ResponseWriter, r *http.Request) {
	target := "https://" + r.Host + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func performUpdate() error {
	log.Println("Starting update process...")

	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	log.Printf("Current executable: %s", exePath)

	// Find git repository root by walking up from executable
	gitRoot, err := findGitRoot(exePath)
	if err != nil {
		return fmt.Errorf("find git root: %w", err)
	}
	log.Printf("Git repository: %s", gitRoot)

	// Pull latest changes
	log.Println("Pulling latest changes from git...")
	cmd := exec.Command("git", "pull")
	cmd.Dir = gitRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}

	// Rebuild the binary
	log.Println("Rebuilding swoopsd...")
	buildCmd := exec.Command("go", "build", "-o", exePath, "./server/cmd/swoopsd")
	buildCmd.Dir = gitRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("build: %w", err)
	}

	log.Println("✓ Update complete! Please restart swoopsd for changes to take effect.")
	log.Println("  sudo systemctl restart swoopsd")
	return nil
}

func findGitRoot(startPath string) (string, error) {
	dir := startPath
	if stat, err := os.Stat(dir); err == nil && !stat.IsDir() {
		dir = filepath.Dir(dir)
	}

	for {
		gitDir := filepath.Join(dir, ".git")
		if stat, err := os.Stat(gitDir); err == nil && stat.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("git repository not found")
		}
		dir = parent
	}
}
