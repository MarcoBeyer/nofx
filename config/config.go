package config

import (
	"os"
	"strconv"
	"strings"
)

// Global configuration instance
var global *Config

// Config is the global configuration (loaded from .env)
// Only contains truly global config, trading related config is at trader/strategy level
type Config struct {
	// Service configuration
	APIServerPort       int
	JWTSecret           string
	RegistrationEnabled bool
	MaxUsers            int // Maximum number of users allowed (0 = unlimited, default = 1)
	AllowedEmails       []string // Whitelist of allowed emails (empty = allow all)

	// Security configuration
	// TransportEncryption enables browser-side encryption for API keys
	// Requires HTTPS or localhost. Set to false for HTTP access via IP.
	TransportEncryption bool
}

// Init initializes global configuration (from .env)
func Init() {
	cfg := &Config{
		APIServerPort:       8080,
		RegistrationEnabled: true,
		MaxUsers:            1, // Default: only 1 user allowed
	}

	// Load from environment variables
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = strings.TrimSpace(v)
	}
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "default-jwt-secret-change-in-production"
	}

	if v := os.Getenv("REGISTRATION_ENABLED"); v != "" {
		cfg.RegistrationEnabled = strings.ToLower(v) == "true"
	}

	if v := os.Getenv("MAX_USERS"); v != "" {
		if maxUsers, err := strconv.Atoi(v); err == nil && maxUsers >= 0 {
			cfg.MaxUsers = maxUsers
		}
	}

	if v := os.Getenv("ALLOWED_EMAILS"); v != "" {
		emails := strings.Split(v, ",")
		for _, email := range emails {
			trimmed := strings.TrimSpace(email)
			if trimmed != "" {
				cfg.AllowedEmails = append(cfg.AllowedEmails, trimmed)
			}
		}
	}

	if v := os.Getenv("API_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.APIServerPort = port
		}
	}

	// Transport encryption: default false for easier deployment
	// Set TRANSPORT_ENCRYPTION=true to enable (requires HTTPS or localhost)
	if v := os.Getenv("TRANSPORT_ENCRYPTION"); v != "" {
		cfg.TransportEncryption = strings.ToLower(v) == "true"
	}

	global = cfg
}

// Get returns the global configuration
func Get() *Config {
	if global == nil {
		Init()
	}
	return global
}
