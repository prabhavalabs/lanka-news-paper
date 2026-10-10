package config

import (
	"encoding/base64"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"regexp"
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
	MediaLocalDirectory        string
	MigrationsPath             string
	Newsletter                 NewsletterConfig
	Social                     SocialConfig
	R2AccessKeyID              string
	R2AccountID                string
	R2Bucket                   string
	R2SecretAccessKey          string
	SessionCookieSecure        bool
	SessionSecret              string
	SessionTTL                 time.Duration
	ShutdownTimeout            time.Duration
}

type NewsletterConfig struct {
	BaseURL             string
	ConfiguredRecipient string
	Enabled             bool
	From                string
	ResendAPIKey        string
	SendHour            int
	Timezone            string
}

func (config Config) BlockInSharedDevelopment(process string) error {
	if config.Environment == "shared-development" {
		return fmt.Errorf("%s is disabled when SNAP_ENV=shared-development", process)
	}
	return nil
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
	r2AccountID := strings.TrimSpace(lookup("SNAP_R2_ACCOUNT_ID"))
	r2AccessKeyID := strings.TrimSpace(lookup("SNAP_R2_ACCESS_KEY_ID"))
	r2SecretAccessKey := strings.TrimSpace(lookup("SNAP_R2_SECRET_ACCESS_KEY"))
	r2Bucket := strings.TrimSpace(lookup("SNAP_R2_BUCKET"))
	r2ConfiguredValues := 0
	for _, value := range []string{r2AccountID, r2AccessKeyID, r2SecretAccessKey, r2Bucket} {
		if value != "" {
			r2ConfiguredValues++
		}
	}
	if r2ConfiguredValues != 0 && r2ConfiguredValues != 4 {
		return Config{}, fmt.Errorf("configure all SNAP_R2_* values or leave all of them empty")
	}
	newsletter, err := loadNewsletterConfig(lookup)
	if err != nil {
		return Config{}, err
	}

	social, err := loadSocialConfig(lookup, environment, newsletter.BaseURL)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Social:                     social,
		Address:                    valueOrDefault(lookup("SNAP_API_ADDRESS"), defaultAddress),
		AllowedOrigins:             allowedOrigins,
		BootstrapAdminEmail:        strings.TrimSpace(lookup("SNAP_BOOTSTRAP_ADMIN_EMAIL")),
		BootstrapAdminPasswordHash: strings.TrimSpace(lookup("SNAP_BOOTSTRAP_ADMIN_PASSWORD_HASH")),
		DatabaseURL:                databaseURL,
		Environment:                environment,
		MediaLocalDirectory:        valueOrDefault(lookup("SNAP_MEDIA_LOCAL_DIR"), ".data/media"),
		MigrationsPath:             valueOrDefault(lookup("SNAP_MIGRATIONS_PATH"), "migrations"),
		Newsletter:                 newsletter,
		R2AccessKeyID:              r2AccessKeyID,
		R2AccountID:                r2AccountID,
		R2Bucket:                   r2Bucket,
		R2SecretAccessKey:          r2SecretAccessKey,
		SessionCookieSecure:        secureCookie,
		SessionSecret:              valueOrDefault(lookup("SNAP_SESSION_SECRET"), "local-dev-session-secret-change"),
		SessionTTL:                 defaultSessionTTL,
		ShutdownTimeout:            shutdownTimeout,
	}, nil
}

func loadNewsletterConfig(lookup LookupFunc) (NewsletterConfig, error) {
	configured := NewsletterConfig{
		BaseURL:             strings.TrimRight(strings.TrimSpace(lookup("SNAP_NEWSLETTER_BASE_URL")), "/"),
		ConfiguredRecipient: strings.ToLower(strings.TrimSpace(lookup("SNAP_NEWSLETTER_RECIPIENT"))),
		From:                strings.TrimSpace(lookup("SNAP_NEWSLETTER_FROM")),
		ResendAPIKey:        strings.TrimSpace(lookup("RESEND_API_KEY")),
		SendHour:            8,
		Timezone:            valueOrDefault(lookup("SNAP_NEWSLETTER_TIMEZONE"), "Asia/Colombo"),
	}
	if enabled := strings.TrimSpace(lookup("SNAP_NEWSLETTER_ENABLED")); enabled != "" {
		parsed, err := strconv.ParseBool(enabled)
		if err != nil {
			return NewsletterConfig{}, fmt.Errorf("parse SNAP_NEWSLETTER_ENABLED: expected true or false")
		}
		configured.Enabled = parsed
	}
	if hour := strings.TrimSpace(lookup("SNAP_NEWSLETTER_SEND_HOUR")); hour != "" {
		parsed, err := strconv.Atoi(hour)
		if err != nil || parsed < 0 || parsed > 23 {
			return NewsletterConfig{}, fmt.Errorf("parse SNAP_NEWSLETTER_SEND_HOUR: expected an hour from 0 to 23")
		}
		configured.SendHour = parsed
	}
	if _, err := time.LoadLocation(configured.Timezone); err != nil {
		return NewsletterConfig{}, fmt.Errorf("parse SNAP_NEWSLETTER_TIMEZONE: unknown timezone")
	}
	if !configured.Enabled {
		return configured, nil
	}
	if configured.ResendAPIKey == "" {
		return NewsletterConfig{}, fmt.Errorf("RESEND_API_KEY is required when the newsletter is enabled")
	}
	if address, err := mail.ParseAddress(configured.From); err != nil || address.Address == "" {
		return NewsletterConfig{}, fmt.Errorf("SNAP_NEWSLETTER_FROM must be a valid sender address")
	}
	parsedBaseURL, err := url.Parse(configured.BaseURL)
	if err != nil || parsedBaseURL.Scheme != "https" || parsedBaseURL.Host == "" {
		return NewsletterConfig{}, fmt.Errorf("SNAP_NEWSLETTER_BASE_URL must be an HTTPS URL")
	}
	if configured.ConfiguredRecipient != "" {
		address, err := mail.ParseAddress(configured.ConfiguredRecipient)
		if err != nil || address.Address != configured.ConfiguredRecipient {
			return NewsletterConfig{}, fmt.Errorf("SNAP_NEWSLETTER_RECIPIENT must be a valid email")
		}
	}
	return configured, nil
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

type SocialConfig struct {
	AppID         string
	AppSecret     string
	RedirectURL   string
	LoginConfigID string
	GraphVersion  string
	TokenKey      []byte
	PublicBaseURL string
	ChromePath    string
	// IncludeLink adds the public story link to captions and cards. Off until the public domain is ready.
	IncludeLink bool
}

func (config SocialConfig) Configured() bool {
	return config.AppID != "" && config.AppSecret != "" && len(config.TokenKey) == 32
}

func loadSocialConfig(lookup LookupFunc, environment, newsletterBaseURL string) (SocialConfig, error) {
	redirect := "http://127.0.0.1:5174/api/admin/social/facebook/callback"
	if environment == "production" {
		redirect = "https://admin.thulawanews.com/api/admin/social/facebook/callback"
	}
	base := newsletterBaseURL
	if base == "" {
		base = "https://thulawanews.com"
	}
	configured := SocialConfig{
		AppID: strings.TrimSpace(lookup("SNAP_META_APP_ID")), AppSecret: strings.TrimSpace(lookup("SNAP_META_APP_SECRET")),
		RedirectURL:   valueOrDefault(lookup("SNAP_META_REDIRECT_URL"), redirect),
		LoginConfigID: strings.TrimSpace(lookup("SNAP_META_LOGIN_CONFIG_ID")),
		GraphVersion:  valueOrDefault(lookup("SNAP_META_GRAPH_VERSION"), "v26.0"),
		PublicBaseURL: strings.TrimRight(valueOrDefault(lookup("SNAP_SOCIAL_PUBLIC_BASE_URL"), base), "/"),
		ChromePath:    strings.TrimSpace(lookup("SNAP_SOCIAL_CHROME_PATH")),
	}
	if value := strings.TrimSpace(lookup("SNAP_SOCIAL_INCLUDE_LINK")); value != "" {
		include, err := strconv.ParseBool(value)
		if err != nil {
			return SocialConfig{}, fmt.Errorf("SNAP_SOCIAL_INCLUDE_LINK must be a boolean")
		}
		configured.IncludeLink = include
	}
	if value := strings.TrimSpace(lookup("SNAP_SOCIAL_TOKEN_KEY")); value != "" {
		key, err := base64.StdEncoding.DecodeString(value)
		if err != nil || len(key) != 32 {
			return SocialConfig{}, fmt.Errorf("SNAP_SOCIAL_TOKEN_KEY must be base64-encoded 32 bytes")
		}
		configured.TokenKey = key
	}
	for _, item := range []struct{ name, value string }{{"SNAP_META_REDIRECT_URL", configured.RedirectURL}, {"SNAP_SOCIAL_PUBLIC_BASE_URL", configured.PublicBaseURL}} {
		parsed, err := url.Parse(item.value)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return SocialConfig{}, fmt.Errorf("%s must be an absolute HTTP(S) URL without credentials, query or fragment", item.name)
		}
		if environment == "production" && parsed.Scheme != "https" {
			return SocialConfig{}, fmt.Errorf("%s must use HTTPS in production", item.name)
		}
	}
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+$`).MatchString(configured.GraphVersion) {
		return SocialConfig{}, fmt.Errorf("SNAP_META_GRAPH_VERSION must have the form v26.0")
	}
	return configured, nil
}
