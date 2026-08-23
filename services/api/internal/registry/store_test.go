package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateIconURL(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "https://publisher.example/icon.png"} {
		if err := validateIconURL(value); err != nil {
			t.Fatalf("validateIconURL(%q) returned %v", value, err)
		}
	}
	for _, value := range []string{"/icon.png", "http://publisher.example/icon.png", "https://user@publisher.example/icon.png"} {
		if err := validateIconURL(value); err == nil {
			t.Fatalf("validateIconURL(%q) accepted an unsafe URL", value)
		}
	}
}

func TestValidateCollectionProfile(t *testing.T) {
	profile := CollectionProfile{
		DiscoveryMethod:       "rss",
		ArticleMethod:         "html_static",
		MinDelaySeconds:       5,
		MaxRequestsPerRun:     25,
		MaxPages:              3,
		RequestTimeoutSeconds: 15,
		Config: CollectionConfig{
			DiscoveryURLs:      []string{"https://www.publisher.example/feed"},
			AllowedHosts:       []string{"publisher.example"},
			ArticleURLPatterns: []string{`^https://www\.publisher\.example/news/`},
			ContentSelector:    "article .body",
		},
	}
	require.NoError(t, validateCollectionProfile(&profile, "https://www.publisher.example/feed", "https://publisher.example"))
	require.Equal(t, 200, profile.Config.MinContentCharacters)

	profile.Config.AllowedHosts = []string{"internal.example"}
	require.Error(t, validateCollectionProfile(&profile, "https://www.publisher.example/feed", "https://publisher.example"))

	profile.Config.AllowedHosts = []string{"example"}
	require.Error(t, validateCollectionProfile(&profile, "https://www.publisher.example/feed", ""), "a source subdomain must not authorize its parent domain")
}

func TestValidateComplianceDependencies(t *testing.T) {
	pending := ComplianceReview{Status: "pending", AllowDiscovery: true}
	require.Error(t, validateComplianceReview(&pending, "https://publisher.example"))

	review := ComplianceReview{Status: "approved", AllowPublicFullText: true}
	require.Error(t, validateComplianceReview(&review, "https://publisher.example"))

	review.AllowFullTextStorage = true
	require.NoError(t, validateComplianceReview(&review, "https://publisher.example"))
	require.Equal(t, "https://publisher.example/robots.txt", review.RobotsURL)
}
