package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	GRPC     GRPCConfig     `yaml:"grpc"`
	Auth     AuthConfig     `yaml:"auth"`
}

type ServerConfig struct {
	Host           string   `yaml:"host"`
	Port           int      `yaml:"port"`
	ExternalURL    string   `yaml:"external_url"` // Public URL for remote agents to connect (e.g., https://swoops.example.com:8080)
	AllowedOrigins []string `yaml:"allowed_origins"`

	// Manual TLS configuration
	TLSCert        string   `yaml:"tls_cert"`  // Path to TLS certificate file for HTTPS
	TLSKey         string   `yaml:"tls_key"`   // Path to TLS private key file for HTTPS
	TLSEnabled     bool     `yaml:"tls_enabled"` // Enable HTTPS with manual certificates (production recommended)

	// Automatic HTTPS with Let's Encrypt (recommended for production)
	AutocertEnabled bool     `yaml:"autocert_enabled"` // Enable automatic HTTPS with Let's Encrypt
	AutocertDomain  string   `yaml:"autocert_domain"`  // Domain name for Let's Encrypt (e.g., swoops.example.com)
	AutocertEmail   string   `yaml:"autocert_email"`   // Email for Let's Encrypt notifications (optional but recommended)
	AutocertCacheDir string  `yaml:"autocert_cache_dir"` // Directory to cache certificates (default: ~/.cache/swoops/autocert)
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type GRPCConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	TLSCert   string `yaml:"tls_cert"`   // Path to server TLS certificate file
	TLSKey    string `yaml:"tls_key"`    // Path to server TLS private key file
	ClientCA  string `yaml:"client_ca"`  // Path to client CA certificate for mTLS (optional)
	Insecure  bool   `yaml:"insecure"`   // Allow insecure connections (dev only)
	RequireMTLS bool `yaml:"require_mtls"` // Require client certificates (mTLS)
}

type AuthConfig struct {
	APIKey string `yaml:"api_key"`
}

// Validate checks the configuration for errors
func (c *Config) Validate() error {
	// Validate server port
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}

	// Validate gRPC port
	if c.GRPC.Port < 1 || c.GRPC.Port > 65535 {
		return fmt.Errorf("grpc.port must be between 1 and 65535, got %d", c.GRPC.Port)
	}

	// Validate HTTP server TLS configuration
	if c.Server.TLSEnabled && c.Server.AutocertEnabled {
		return fmt.Errorf("cannot enable both manual TLS (tls_enabled) and automatic HTTPS (autocert_enabled)")
	}

	if c.Server.AutocertEnabled {
		if c.Server.AutocertDomain == "" {
			return fmt.Errorf("server.autocert_domain is required when autocert_enabled=true")
		}
		// Autocert requires binding to port 443 or using a reverse proxy
		if c.Server.Port != 443 && c.Server.Port != 80 {
			log.Printf("Warning: autocert is enabled but port is %d. For Let's Encrypt to work, you typically need port 443 (HTTPS) and 80 (HTTP challenge)", c.Server.Port)
		}
		// Set default cache directory if not specified
		if c.Server.AutocertCacheDir == "" {
			// Use /var/cache for system services, ~/.cache for user installations
			// Check if we're running as root or a system user (no HOME or HOME=/nonexistent)
			homeDir := os.Getenv("HOME")
			if homeDir == "" || homeDir == "/" || homeDir == "/nonexistent" || homeDir == "/var/empty" {
				// System service - use /var/cache
				c.Server.AutocertCacheDir = "/var/cache/swoops/autocert"
			} else {
				// User installation - use ~/.cache
				c.Server.AutocertCacheDir = filepath.Join(homeDir, ".cache", "swoops", "autocert")
			}
		}
		log.Printf("Automatic HTTPS enabled for domain: %s", c.Server.AutocertDomain)
		log.Printf("Certificates will be cached in: %s", c.Server.AutocertCacheDir)
	} else if c.Server.TLSEnabled {
		if c.Server.TLSCert == "" {
			return fmt.Errorf("server.tls_cert is required when tls_enabled=true")
		}
		if c.Server.TLSKey == "" {
			return fmt.Errorf("server.tls_key is required when tls_enabled=true")
		}
		// Check if files exist
		if _, err := os.Stat(c.Server.TLSCert); err != nil {
			return fmt.Errorf("server.tls_cert file not found: %w", err)
		}
		if _, err := os.Stat(c.Server.TLSKey); err != nil {
			return fmt.Errorf("server.tls_key file not found: %w", err)
		}
	} else {
		log.Printf("Warning: HTTP server is running without TLS (HTTP only). This should only be used for development.")
	}

	// Validate gRPC TLS configuration
	if !c.GRPC.Insecure {
		if c.GRPC.TLSCert == "" {
			return fmt.Errorf("grpc.tls_cert is required when insecure=false")
		}
		if c.GRPC.TLSKey == "" {
			return fmt.Errorf("grpc.tls_key is required when insecure=false")
		}
		// Check if files exist
		if _, err := os.Stat(c.GRPC.TLSCert); err != nil {
			return fmt.Errorf("grpc.tls_cert file not found: %w", err)
		}
		if _, err := os.Stat(c.GRPC.TLSKey); err != nil {
			return fmt.Errorf("grpc.tls_key file not found: %w", err)
		}
		// Validate mTLS configuration
		if c.GRPC.RequireMTLS {
			if c.GRPC.ClientCA == "" {
				return fmt.Errorf("grpc.client_ca is required when require_mtls=true")
			}
			if _, err := os.Stat(c.GRPC.ClientCA); err != nil {
				return fmt.Errorf("grpc.client_ca file not found: %w", err)
			}
			log.Printf("gRPC mTLS enabled: client certificates will be required and validated")
		}
	} else {
		log.Printf("Warning: gRPC is running in INSECURE mode (no TLS). This should only be used for development.")
	}

	return nil
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:           "0.0.0.0",
			Port:           8080,
			AllowedOrigins: []string{},
		},
		Database: DatabaseConfig{
			Path: "swoops.db",
		},
		GRPC: GRPCConfig{
			Host:     "0.0.0.0",
			Port:     9090,
			Insecure: true, // Default to insecure for dev, should be false in production
		},
		Auth: AuthConfig{},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read config: %w", err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse config: %w", err)
			}
		}
	}

	// Try to read saved API key from file if not set in config
	if cfg.Auth.APIKey == "" {
		if savedKey, err := readAPIKeyFromFile(); err == nil && savedKey != "" {
			cfg.Auth.APIKey = savedKey
		}
	}

	// Environment variable overrides (takes precedence over saved file)
	if v := os.Getenv("SWOOPS_API_KEY"); v != "" {
		cfg.Auth.APIKey = v
	}
	if v := os.Getenv("SWOOPS_DB_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("SWOOPS_EXTERNAL_URL"); v != "" {
		cfg.Server.ExternalURL = v
	}
	if v := os.Getenv("SWOOPS_TLS_CERT"); v != "" {
		cfg.Server.TLSCert = v
	}
	if v := os.Getenv("SWOOPS_TLS_KEY"); v != "" {
		cfg.Server.TLSKey = v
	}
	if v := os.Getenv("SWOOPS_TLS_ENABLED"); v != "" {
		cfg.Server.TLSEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("SWOOPS_AUTOCERT_ENABLED"); v != "" {
		cfg.Server.AutocertEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("SWOOPS_AUTOCERT_DOMAIN"); v != "" {
		cfg.Server.AutocertDomain = v
	}
	if v := os.Getenv("SWOOPS_AUTOCERT_EMAIL"); v != "" {
		cfg.Server.AutocertEmail = v
	}
	if v := os.Getenv("SWOOPS_AUTOCERT_CACHE_DIR"); v != "" {
		cfg.Server.AutocertCacheDir = v
	}
	if v := os.Getenv("SWOOPS_GRPC_HOST"); v != "" {
		cfg.GRPC.Host = v
	}
	if v := os.Getenv("SWOOPS_GRPC_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 65535 {
			cfg.GRPC.Port = p
		} else {
			log.Printf("Warning: invalid SWOOPS_GRPC_PORT value: %s (must be 1-65535)", v)
		}
	}
	if v := os.Getenv("SWOOPS_GRPC_TLS_CERT"); v != "" {
		cfg.GRPC.TLSCert = v
	}
	if v := os.Getenv("SWOOPS_GRPC_TLS_KEY"); v != "" {
		cfg.GRPC.TLSKey = v
	}
	if v := os.Getenv("SWOOPS_GRPC_CLIENT_CA"); v != "" {
		cfg.GRPC.ClientCA = v
	}
	if v := os.Getenv("SWOOPS_GRPC_INSECURE"); v != "" {
		cfg.GRPC.Insecure = v == "true" || v == "1"
	}
	if v := os.Getenv("SWOOPS_GRPC_REQUIRE_MTLS"); v != "" {
		cfg.GRPC.RequireMTLS = v == "true" || v == "1"
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Auto-generate API key if not set
	if cfg.Auth.APIKey == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate API key: %w", err)
		}
		cfg.Auth.APIKey = hex.EncodeToString(key)

		// Write API key to file instead of logging (avoid credential leakage)
		if err := writeAPIKeyToFile(cfg.Auth.APIKey); err != nil {
			log.Printf("Warning: failed to write API key to file: %v", err)
			log.Printf("Generated API key - set SWOOPS_API_KEY or auth.api_key in config to persist it")
		}
	}

	return cfg, nil
}

// readAPIKeyFromFile reads a previously saved API key from ~/.config/swoops/api_key
func readAPIKeyFromFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	keyPath := filepath.Join(homeDir, ".config", "swoops", "api_key")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Not an error, just no saved key
		}
		return "", fmt.Errorf("read API key file: %w", err)
	}

	// Trim whitespace and newlines, validate hex format
	apiKey := string(data)
	// Remove leading/trailing whitespace
	for len(apiKey) > 0 && (apiKey[0] == ' ' || apiKey[0] == '\t' || apiKey[0] == '\n' || apiKey[0] == '\r') {
		apiKey = apiKey[1:]
	}
	for len(apiKey) > 0 && (apiKey[len(apiKey)-1] == ' ' || apiKey[len(apiKey)-1] == '\t' || apiKey[len(apiKey)-1] == '\n' || apiKey[len(apiKey)-1] == '\r') {
		apiKey = apiKey[:len(apiKey)-1]
	}

	if len(apiKey) == 0 {
		return "", fmt.Errorf("empty API key in file")
	}

	return apiKey, nil
}

// writeAPIKeyToFile writes the API key to ~/.config/swoops/api_key with secure permissions
func writeAPIKeyToFile(apiKey string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "swoops")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	keyPath := filepath.Join(configDir, "api_key")
	if err := os.WriteFile(keyPath, []byte(apiKey), 0600); err != nil {
		return fmt.Errorf("write API key file: %w", err)
	}

	log.Printf("No API key configured — generated new key and saved to: %s", keyPath)
	log.Printf("Set SWOOPS_API_KEY or auth.api_key in config to use a custom key")
	return nil
}
