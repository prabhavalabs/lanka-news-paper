package content

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateOutboundURL(t *testing.T) {
	parsed, err := validateOutboundURL("https://news.example/story#section", []string{"news.example"})
	require.NoError(t, err)
	require.Empty(t, parsed.Fragment)

	for _, value := range []string{
		"http://news.example/story",
		"https://other.example/story",
		"https://127.0.0.1/story",
		"https://user@news.example/story",
		"https://news.example:8443/story",
	} {
		_, err := validateOutboundURL(value, []string{"news.example"})
		require.Error(t, err, value)
	}
}

func TestPublicAddress(t *testing.T) {
	require.True(t, publicAddress(net.ParseIP("1.1.1.1")))
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "fc00::1"} {
		require.False(t, publicAddress(net.ParseIP(value)), value)
	}
}

func TestHostAllowlistDoesNotAcceptParentDomain(t *testing.T) {
	require.True(t, hostAllowed("sub.news.example", []string{"news.example"}))
	require.False(t, hostAllowed("example", []string{"news.example"}))
	require.False(t, hostAllowed("news.example.attacker.test", []string{"news.example"}))
}
