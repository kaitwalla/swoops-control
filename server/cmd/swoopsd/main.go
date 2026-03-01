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
	"os/signal"
	"syscall"
	"time"

	"github.com/swoopsh/swoops/pkg/agentrpc"
	"github.com/swoopsh/swoops/pkg/version"
	"github.com/swoopsh/swoops/server/internal/agentconn"
	"github.com/swoopsh/swoops/server/internal/api"
	"github.com/swoopsh/swoops/server/internal/config"
	"github.com/swoopsh/swoops/server/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	showVersion := flag.Bool("version", false, "show version information and exit")
	flag.Parse()

	if *showVersion {
		versionInfo := version.Get()
		fmt.Println(versionInfo.String())
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

	go func() {
		if cfg.Server.TLSEnabled {
			log.Printf("Swoops control plane starting on https://%s (TLS enabled)", addr)
			if err := httpServer.ListenAndServeTLS(cfg.Server.TLSCert, cfg.Server.TLSKey); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTPS server error: %v", err)
			}
		} else {
			log.Printf("Swoops control plane starting on http://%s (TLS disabled)", addr)
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
	grpcServer.GracefulStop()

	log.Println("Swoops control plane stopped")
}
