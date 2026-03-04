package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/pkg/version"
	"github.com/kaitwalla/swoops-control/server/internal/agentconn"
	"github.com/kaitwalla/swoops-control/server/internal/api"
	"github.com/kaitwalla/swoops-control/server/internal/certrotate"
	"github.com/kaitwalla/swoops-control/server/internal/config"
	"github.com/kaitwalla/swoops-control/server/internal/store"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	// Check for subcommands first
	if len(os.Args) > 1 && os.Args[1] == "create-user" {
		runCreateUser(os.Args[2:])
		return
	}

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

	// Configure TLS with certificate rotation
	var autocertManager *autocert.Manager
	var httpCertRotator *certrotate.CertRotator

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
		log.Printf("Certificate auto-renewal enabled (Let's Encrypt)")
	} else if cfg.Server.TLSEnabled {
		// Manual TLS with certificate rotation
		httpCertRotator, err = certrotate.NewCertRotator(cfg.Server.TLSCert, cfg.Server.TLSKey, "", logger)
		if err != nil {
			log.Fatalf("Failed to initialize certificate rotator: %v", err)
		}
		defer httpCertRotator.Stop()

		httpServer.TLSConfig = &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: httpCertRotator.GetCertificateFunc(),
		}
		log.Printf("Manual TLS configured with automatic certificate rotation (checking every 5 minutes)")
		log.Printf("Certificate files: %s, %s", cfg.Server.TLSCert, cfg.Server.TLSKey)
	}

	grpcAddr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen for gRPC on %s: %v", grpcAddr, err)
	}

	// Configure gRPC server with optional TLS and mTLS
	var grpcServer *grpc.Server
	var grpcCertRotator *certrotate.CertRotator

	if cfg.GRPC.Insecure {
		log.Printf("Warning: gRPC server running in INSECURE mode (no TLS)")
		grpcServer = grpc.NewServer()
	} else {
		// Initialize certificate rotator for gRPC
		caFile := ""
		if cfg.GRPC.RequireMTLS {
			caFile = cfg.GRPC.ClientCA
		}

		grpcCertRotator, err = certrotate.NewCertRotator(cfg.GRPC.TLSCert, cfg.GRPC.TLSKey, caFile, logger)
		if err != nil {
			log.Fatalf("Failed to initialize gRPC certificate rotator: %v", err)
		}
		defer grpcCertRotator.Stop()

		tlsConfig := &tls.Config{
			MinVersion:     tls.VersionTLS13,
			GetCertificate: grpcCertRotator.GetCertificateFunc(),
		}

		// Configure mTLS if enabled
		if cfg.GRPC.RequireMTLS {
			// Use dynamic CA pool from cert rotator
			tlsConfig.ClientCAs = grpcCertRotator.GetCACertPool()
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

			// Update GetConfigForClient to provide fresh CA pool for each connection
			tlsConfig.GetConfigForClient = func(info *tls.ClientHelloInfo) (*tls.Config, error) {
				config := tlsConfig.Clone()
				config.ClientCAs = grpcCertRotator.GetCACertPool()
				return config, nil
			}

			log.Printf("gRPC server configured with mTLS and automatic certificate rotation")
			log.Printf("Server cert: %s, Client CA: %s", cfg.GRPC.TLSCert, cfg.GRPC.ClientCA)
		} else {
			log.Printf("gRPC server configured with TLS and automatic certificate rotation")
			log.Printf("Server cert: %s", cfg.GRPC.TLSCert)
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

	// Detect architecture
	arch := os.Getenv("GOARCH")
	if arch == "" {
		// Fallback to runtime detection
		arch = "amd64" // Default, could use runtime.GOARCH
		cmd := exec.Command("uname", "-m")
		if output, err := cmd.Output(); err == nil {
			machine := string(output[:len(output)-1]) // trim newline
			if machine == "x86_64" {
				arch = "amd64"
			} else if machine == "aarch64" || machine == "arm64" {
				arch = "arm64"
			}
		}
	}

	// Detect OS
	goos := os.Getenv("GOOS")
	if goos == "" {
		goos = "linux" // Default for production
		cmd := exec.Command("uname", "-s")
		if output, err := cmd.Output(); err == nil {
			osName := string(output[:len(output)-1]) // trim newline
			if osName == "Linux" {
				goos = "linux"
			} else if osName == "Darwin" {
				goos = "darwin"
			}
		}
	}

	// Fetch latest release from GitHub
	log.Println("Fetching latest release information...")
	githubRepo := "kaitwalla/swoops-control"
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("github API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parse release info: %w", err)
	}

	log.Printf("Latest version: %s", release.TagName)

	// Download binary
	binaryName := fmt.Sprintf("swoopsd-%s-%s", goos, arch)
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, release.TagName, binaryName)

	log.Printf("Downloading from %s...", downloadURL)
	resp, err = http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Write to temporary file
	tempFile, err := os.CreateTemp("", "swoopsd-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if err := os.WriteFile(tempPath, []byte{}, 0755); err != nil {
		return fmt.Errorf("prepare temp file: %w", err)
	}

	file, err := os.OpenFile(tempPath, os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}

	_, err = file.ReadFrom(resp.Body)
	file.Close()
	if err != nil {
		return fmt.Errorf("write binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(tempPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// Replace current binary
	log.Printf("Installing to %s...", exePath)

	// Try to rename first (atomic operation)
	err = os.Rename(tempPath, exePath)
	if err != nil {
		// If rename fails, try copy
		input, err := os.ReadFile(tempPath)
		if err != nil {
			return fmt.Errorf("read temp file: %w", err)
		}
		if err := os.WriteFile(exePath, input, 0755); err != nil {
			// Check if it's a permission error
			if os.IsPermission(err) {
				return fmt.Errorf("permission denied writing to %s - try running with sudo: sudo %s --update", exePath, exePath)
			}
			return fmt.Errorf("write binary: %w", err)
		}
	}

	// On Linux, try to grant CAP_NET_BIND_SERVICE for autocert (port 443/80)
	if goos == "linux" {
		// Check if setcap exists
		if _, err := exec.LookPath("setcap"); err != nil {
			log.Printf("Warning: setcap command not found - cannot grant CAP_NET_BIND_SERVICE capability")
			log.Printf("Install libcap2-bin (Debian/Ubuntu) or libcap (RHEL/Fedora), then run:")
			log.Printf("  sudo setcap 'cap_net_bind_service=+ep' %s", exePath)
		} else {
			log.Println("Granting CAP_NET_BIND_SERVICE capability for port 443/80 binding...")
			cmd := exec.Command("setcap", "cap_net_bind_service=+ep", exePath)
			if err := cmd.Run(); err != nil {
				log.Printf("Warning: Failed to grant CAP_NET_BIND_SERVICE capability: %v", err)
				log.Printf("If using autocert, run manually: sudo setcap 'cap_net_bind_service=+ep' %s", exePath)
			} else {
				log.Println("✓ CAP_NET_BIND_SERVICE capability granted")
			}
		}
	}

	log.Println("✓ Update complete! Restart swoopsd for changes to take effect:")
	if goos == "linux" {
		log.Println("  sudo systemctl restart swoopsd")
	} else {
		log.Println("  Restart your swoopsd service")
	}
	return nil
}

// runCreateUser handles the create-user subcommand
func runCreateUser(args []string) {
	if len(args) != 2 {
		fmt.Println("Usage: swoopsd create-user <username> <email>")
		fmt.Println("Password will be read securely from stdin")
		fmt.Println("Example: swoopsd create-user admin admin@example.com")
		os.Exit(1)
	}

	username := args[0]
	email := args[1]

	// Read password securely from stdin
	var password string
	if term.IsTerminal(int(syscall.Stdin)) {
		// Interactive mode: prompt for password without echo
		fmt.Print("Enter password: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // New line after password input
		if err != nil {
			log.Fatalf("Failed to read password: %v", err)
		}
		password = string(passwordBytes)

		// Confirm password
		fmt.Print("Confirm password: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // New line after confirmation
		if err != nil {
			log.Fatalf("Failed to read password confirmation: %v", err)
		}

		if password != string(confirmBytes) {
			log.Fatal("Passwords do not match")
		}
	} else {
		// Non-interactive mode: read from pipe/stdin
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read password from stdin: %v", err)
		}
		password = strings.TrimSpace(line)
	}

	if password == "" {
		log.Fatal("Password cannot be empty")
	}

	// Get database path from environment or use default
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		// Load from config if available
		cfg, err := config.Load("")
		if err == nil && cfg.Database.Path != "" {
			dbPath = cfg.Database.Path
		} else {
			dbPath = "./swoops.db"
		}
	}

	// Open database
	s, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Create admin user
	req := &models.CreateUserRequest{
		Username: username,
		Email:    email,
		Password: password,
		FullName: "Administrator",
		IsAdmin:  true,
	}

	user, err := s.CreateUser(req)
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	fmt.Printf("✓ Admin user created successfully!\n")
	fmt.Printf("  ID:       %s\n", user.ID)
	fmt.Printf("  Username: %s\n", user.Username)
	fmt.Printf("  Email:    %s\n", user.Email)
	fmt.Printf("  Is Admin: %v\n", user.IsAdmin)
	fmt.Printf("  Created:  %s\n", user.CreatedAt.Format(time.RFC3339))
	fmt.Printf("\nYou can now login at the web interface with these credentials.\n")
}
