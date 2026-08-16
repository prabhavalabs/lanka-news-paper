package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

func CanonicalURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	query := parsed.Query()
	for _, key := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "fbclid", "gclid", "at_campaign", "at_medium", "at_format"} {
		query.Del(key)
	}
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""
	return parsed.String()
}

func Fingerprint(sourceID, itemID, canonicalURL, headline, published string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{sourceID, itemID, canonicalURL, headline, published}, "\n")))
	return hex.EncodeToString(sum[:])
}
