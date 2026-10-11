package publish

import (
	"strings"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
)

// PublicIcon turns a stored source logo URL into one readers can load: bundled logos and
// external URLs pass through; uploaded logos (admin media) are served from the public
// source-logo route.
func PublicIcon(raw string) string {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return ""
	case strings.HasPrefix(raw, media.URLPrefix+"source-logos/"):
		return "/api/v1/source-logos/" + strings.TrimPrefix(raw, media.URLPrefix+"source-logos/")
	case strings.HasPrefix(raw, "/source-logos/"), strings.HasPrefix(raw, "https://"), strings.HasPrefix(raw, "http://"):
		return raw
	default:
		return ""
	}
}
