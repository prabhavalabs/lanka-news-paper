package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	_, err := Load(func(string) string { return "" })
	require.Error(t, err)
	require.Contains(t, err.Error(), "SNAP_DATABASE_URL")
}

func TestLoadUsesDefaults(t *testing.T) {
	loaded, err := Load(func(key string) string {
		if key == "SNAP_DATABASE_URL" {
			return "postgres://snap:snap@127.0.0.1:55432/snap?sslmode=disable"
		}
		return ""
	})
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:8090", loaded.Address)
	require.Equal(t, "local", loaded.Environment)
	require.Equal(t, ".data/media", loaded.MediaLocalDirectory)
	require.False(t, loaded.SessionCookieSecure)
	require.Equal(t, 10*time.Second, loaded.ShutdownTimeout)
	require.Equal(t, []string{"http://127.0.0.1:5173", "http://127.0.0.1:5174"}, loaded.AllowedOrigins)
}

func TestLoadRejectsPartialR2Configuration(t *testing.T) {
	_, err := Load(func(key string) string {
		switch key {
		case "SNAP_DATABASE_URL":
			return "postgres://snap:snap@127.0.0.1:55432/snap?sslmode=disable"
		case "SNAP_R2_BUCKET":
			return "lanka-news-media"
		default:
			return ""
		}
	})
	require.ErrorContains(t, err, "configure all SNAP_R2_*")
}

func TestLoadSecureCookieOverride(t *testing.T) {
	loaded, err := Load(func(key string) string {
		switch key {
		case "SNAP_DATABASE_URL":
			return "postgres://snap:snap@127.0.0.1:55432/snap?sslmode=disable"
		case "SNAP_SESSION_COOKIE_SECURE":
			return "true"
		default:
			return ""
		}
	})
	require.NoError(t, err)
	require.True(t, loaded.SessionCookieSecure)
}
