package config

import (
	"encoding/base64"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSocialConfigOptionalAndValidated(t *testing.T) {
	values := map[string]string{}
	lookup := func(key string) string { return values[key] }
	configured, err := loadSocialConfig(lookup, "local", "")
	require.NoError(t, err)
	require.False(t, configured.Configured())
	require.Equal(t, "v26.0", configured.GraphVersion)
	require.Equal(t, "http://127.0.0.1:5174/api/admin/social/facebook/callback", configured.RedirectURL)
	configured, err = loadSocialConfig(lookup, "production", "https://news.invalid")
	require.NoError(t, err)
	require.Equal(t, "https://news.invalid", configured.PublicBaseURL)
	values["SNAP_META_APP_ID"] = "app"
	values["SNAP_META_APP_SECRET"] = "secret"
	values["SNAP_SOCIAL_TOKEN_KEY"] = base64.StdEncoding.EncodeToString(make([]byte, 32))
	configured, err = loadSocialConfig(lookup, "local", "")
	require.NoError(t, err)
	require.True(t, configured.Configured())
	values["SNAP_SOCIAL_TOKEN_KEY"] = base64.StdEncoding.EncodeToString(make([]byte, 31))
	_, err = loadSocialConfig(lookup, "local", "")
	require.ErrorContains(t, err, "32 bytes")
	values["SNAP_SOCIAL_TOKEN_KEY"] = ""
	values["SNAP_META_REDIRECT_URL"] = "https://user:pass@admin.invalid/callback"
	_, err = loadSocialConfig(lookup, "local", "")
	require.Error(t, err)
	values["SNAP_META_REDIRECT_URL"] = "http://admin.invalid/callback"
	_, err = loadSocialConfig(lookup, "production", "")
	require.Error(t, err)
	values["SNAP_META_REDIRECT_URL"] = ""
	values["SNAP_META_GRAPH_VERSION"] = "v26.0/evil"
	_, err = loadSocialConfig(lookup, "local", "")
	require.Error(t, err)
}
