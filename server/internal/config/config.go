package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"

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
	AllowedOrigins []string `yaml:"allowed_origins"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

type GRPCConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type AuthConfig struct {
	APIKey string `yaml:"api_key"`
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
			Host: "0.0.0.0",
			Port: 9090,
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
