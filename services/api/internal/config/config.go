package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddress         = "127.0.0.1:8090"
	defaultEnvironment     = "local"
	defaultShutdownTimeout = 10 * time.Second
	defaultSessionTTL      = 12 * time.Hour
)

type Config struct {
	Address                    string
	AllowedOrigins             []string
	BootstrapAdminEmail        string
	BootstrapAdminPasswordHash string
	DatabaseURL                string
	Environment                string
	MigrationsPath             string
	SessionCookieSecure        bool
	SessionSecret              string
	SessionTTL                 time.Duration
	ShutdownTimeout            time.Duration
}

var defaultAllowedOrigins = []string{
	"http://127.0.0.1:5173",
	"http://127.0.0.1:5174",
}

type LookupFunc func(key string) string

func FromEnvironment() (Config, error) {
	return Load(os.Getenv)
}

func Load(lookup LookupFunc) (Config, error) {
	databaseURL := strings.TrimSpace(lookup("SNAP_DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("SNAP_DATABASE_URL is required")
	}

	shutdownTimeout := defaultShutdownTimeout
	if configured := strings.TrimSpace(lookup("SNAP_SHUTDOWN_TIMEOUT")); configured != "" {
		parsed, err := time.ParseDuration(configured)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("parse SNAP_SHUTDOWN_TIMEOUT: expected a positive duration")
		}
		shutdownTimeout = parsed
	}

	allowedOrigins, err := parseAllowedOrigins(lookup("SNAP_ALLOWED_ORIGINS"))
	if err != nil {
		return Config{}, err
	}
	environment := valueOrDefault(lookup("SNAP_ENV"), defaultEnvironment)
	secureCookie := environment != "local"
	if configured := strings.TrimSpace(lookup("SNAP_SESSION_COOKIE_SECURE")); configured != "" {
		secureCookie, err = strconv.ParseBool(configured)
		if err != nil {
			return Config{}, fmt.Errorf("parse SNAP_SESSION_COOKIE_SECURE: expected true or false")
		}
	}

	return Config{
		Address:                    valueOrDefault(lookup("SNAP_API_ADDRESS"), defaultAddress),
		AllowedOrigins:             allowedOrigins,
		BootstrapAdminEmail:        strings.TrimSpace(lookup("SNAP_BOOTSTRAP_ADMIN_EMAIL")),
		BootstrapAdminPasswordHash: strings.TrimSpace(lookup("SNAP_BOOTSTRAP_ADMIN_PASSWORD_HASH")),
		DatabaseURL:                databaseURL,
		Environment:                environment,
		MigrationsPath:             valueOrDefault(lookup("SNAP_MIGRATIONS_PATH"), "migrations"),
		SessionCookieSecure:        secureCookie,
		SessionSecret:              valueOrDefault(lookup("SNAP_SESSION_SECRET"), "local-dev-session-secret-change"),
		SessionTTL:                 defaultSessionTTL,
		ShutdownTimeout:            shutdownTimeout,
	}, nil
}

func parseAllowedOrigins(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return append([]string(nil), defaultAllowedOrigins...), nil
	}

	origins := make([]string, 0)
	seen := make(map[string]struct{})
	for entry := range strings.SplitSeq(value, ",") {
		origin := strings.TrimSpace(entry)
		if origin == "" {
			continue
		}
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.Path != "" {
			return nil, fmt.Errorf("parse SNAP_ALLOWED_ORIGINS: invalid origin %q", origin)
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}
	if len(origins) == 0 {
		return nil, fmt.Errorf("parse SNAP_ALLOWED_ORIGINS: at least one origin is required")
	}
	return origins, nil
}

func valueOrDefault(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}
