package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSocialConfigOptionalAndValidated(t *testing.T) {
	values := map[string]string{}
	lookup := func(key string) string { return values[key] }
	configured, err := loadSocialConfig(lookup, "local", "")
	require.NoError(t, err)
	require.False(t, configured.Configured())
	require.Equal(t, "https://loomcast.prabhavalabs.com", configured.LoomcastURL)
	require.False(t, configured.IncludeLink)
	values["SNAP_SOCIAL_INCLUDE_LINK"] = "true"
	configured, err = loadSocialConfig(lookup, "local", "")
	require.NoError(t, err)
	require.True(t, configured.IncludeLink)
	values["SNAP_SOCIAL_INCLUDE_LINK"] = "sometimes"
	_, err = loadSocialConfig(lookup, "local", "")
	require.ErrorContains(t, err, "SNAP_SOCIAL_INCLUDE_LINK")
	values["SNAP_SOCIAL_INCLUDE_LINK"] = ""
	configured, err = loadSocialConfig(lookup, "production", "https://news.invalid")
	require.NoError(t, err)
	require.Equal(t, "https://news.invalid", configured.PublicBaseURL)
	values["SNAP_LOOMCAST_API_KEY"] = "lc_api_abc123"
	values["SNAP_LOOMCAST_CHANNEL_ID"] = "A3E1C0DE-1234-4ABC-9DEF-0123456789AB"
	values["SNAP_LOOMCAST_URL"] = "https://loomcast.invalid/"
	configured, err = loadSocialConfig(lookup, "production", "")
	require.NoError(t, err)
	require.True(t, configured.Configured())
	require.Equal(t, "https://loomcast.invalid", configured.LoomcastURL)
	require.Equal(t, "a3e1c0de-1234-4abc-9def-0123456789ab", configured.LoomcastChannelID)
	values["SNAP_LOOMCAST_API_KEY"] = "lc_mcp_wrongkind"
	_, err = loadSocialConfig(lookup, "local", "")
	require.ErrorContains(t, err, "SNAP_LOOMCAST_API_KEY")
	values["SNAP_LOOMCAST_API_KEY"] = "lc_api_abc123"
	values["SNAP_LOOMCAST_CHANNEL_ID"] = "facebook"
	_, err = loadSocialConfig(lookup, "local", "")
	require.ErrorContains(t, err, "SNAP_LOOMCAST_CHANNEL_ID")
	values["SNAP_LOOMCAST_CHANNEL_ID"] = ""
	values["SNAP_LOOMCAST_URL"] = "https://user:pass@loomcast.invalid"
	_, err = loadSocialConfig(lookup, "local", "")
	require.Error(t, err)
	values["SNAP_LOOMCAST_URL"] = "http://loomcast.invalid"
	_, err = loadSocialConfig(lookup, "production", "")
	require.ErrorContains(t, err, "HTTPS")
}
