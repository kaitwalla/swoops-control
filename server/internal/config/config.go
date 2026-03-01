package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
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
	ExternalURL    string   `yaml:"external_url"` // Public URL for remote agents to connect (e.g., http://swoops.example.com:8080)
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type GRPCConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	TLSCert  string `yaml:"tls_cert"`  // Path to TLS certificate file
	TLSKey   string `yaml:"tls_key"`   // Path to TLS private key file
	Insecure bool   `yaml:"insecure"`  // Allow insecure connections (dev only)
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

	// Validate TLS configuration
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

	// Environment variable overrides
	if v := os.Getenv("SWOOPS_API_KEY"); v != "" {
		cfg.Auth.APIKey = v
	}
	if v := os.Getenv("SWOOPS_DB_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("SWOOPS_EXTERNAL_URL"); v != "" {
		cfg.Server.ExternalURL = v
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
	if v := os.Getenv("SWOOPS_GRPC_INSECURE"); v != "" {
		cfg.GRPC.Insecure = v == "true" || v == "1"
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
		log.Printf("No API key configured — generated ephemeral key: %s", cfg.Auth.APIKey)
		log.Printf("Set SWOOPS_API_KEY or auth.api_key in config to persist it")
	}

	return cfg, nil
}
