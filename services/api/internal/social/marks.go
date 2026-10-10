package social

import (
	"context"
	"embed"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/card"
)

// Publisher logos, copied from apps/admin/public/source-logos (see SOURCES.md for their origins).
// They identify sources on the card; they are never altered or used as our own branding.
//
//go:embed assets/source-logos/*
var sourceLogos embed.FS

const maxLogoBytes = 512 << 10

// sourceMarks returns one mark per credited source, in credit order, with its logo and
// average political lean (right minus left probability) across the post's evidence articles.
func (service *Service) sourceMarks(ctx context.Context, p Post) ([]card.SourceMark, error) {
	rows, err := service.store.pool.Query(ctx, `
		SELECT s.name, COALESCE(s.icon_url, ''),
		       AVG(apa.right_probability - apa.left_probability) FILTER (WHERE apa.relevant)::float8
		FROM social_posts post
		CROSS JOIN LATERAL unnest(post.evidence_article_ids) AS evidence(article_id)
		JOIN articles a ON a.id = evidence.article_id
		JOIN sources s ON s.id = a.source_id
		LEFT JOIN article_political_analysis apa ON apa.article_id = a.id
		WHERE post.id = $1
		GROUP BY s.id, s.name, s.icon_url`, p.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type info struct {
		icon string
		lean *float64
	}
	byName := map[string]info{}
	for rows.Next() {
		var name, icon string
		var lean *float64
		if err := rows.Scan(&name, &icon, &lean); err != nil {
			return nil, err
		}
		if lean != nil {
			clamped := max(-1, min(1, *lean))
			lean = &clamped
		}
		byName[name] = info{icon, lean}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	marks := make([]card.SourceMark, 0, len(p.SourceNames))
	for _, name := range p.SourceNames {
		found := byName[name]
		mark := card.SourceMark{Name: name, Lean: found.lean}
		mark.Logo, mark.LogoType = service.sourceLogo(ctx, found.icon)
		marks = append(marks, mark)
	}
	return marks, nil
}

// sourceLogo loads a logo from the bundled copies or local media storage. Remote URLs are
// not fetched at render time; those sources show their initial instead.
func (service *Service) sourceLogo(ctx context.Context, icon string) ([]byte, string) {
	var data []byte
	switch {
	case strings.HasPrefix(icon, "/source-logos/"):
		name := path.Base(icon)
		read, err := sourceLogos.ReadFile("assets/source-logos/" + name)
		if err != nil {
			return nil, ""
		}
		data = read
	case strings.HasPrefix(icon, "/api/admin/media/") && service.media != nil:
		file, _, err := service.media.Open(ctx, strings.TrimPrefix(icon, "/api/admin/media/"))
		if err != nil {
			return nil, ""
		}
		defer file.Close()
		read, err := io.ReadAll(io.LimitReader(file, maxLogoBytes+1))
		if err != nil {
			return nil, ""
		}
		data = read
	default:
		return nil, ""
	}
	if len(data) == 0 || len(data) > maxLogoBytes {
		return nil, ""
	}
	if strings.HasSuffix(strings.ToLower(icon), ".svg") {
		return data, "image/svg+xml"
	}
	return data, http.DetectContentType(data)
}
