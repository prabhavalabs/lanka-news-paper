package social

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/card"
)

func editable(status string) bool {
	return status == "pending_review" || status == "approved" || status == "failed"
}

func (service *Service) Edit(ctx context.Context, id string, input EditInput, actor uuid.UUID) (Post, error) {
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)
	p, err := lockPost(ctx, tx, id)
	if err != nil {
		return p, err
	}
	if !editable(p.Status) {
		return p, ErrConflict
	}
	if input.HeadlineSI == nil && input.SummarySI == nil && input.Caption == nil && input.Hashtags == nil {
		return p, ErrInvalid
	}
	if input.HeadlineSI != nil {
		p.HeadlineSI = strings.TrimSpace(*input.HeadlineSI)
	}
	if input.SummarySI != nil {
		p.SummarySI = strings.TrimSpace(*input.SummarySI)
	}
	if input.Caption != nil {
		p.Caption = strings.TrimSpace(*input.Caption)
	}
	if input.Hashtags != nil {
		normalized := normalizeHashtags(*input.Hashtags)
		if len(normalized) != len(*input.Hashtags) {
			return p, ErrInvalid
		}
		if input.Caption == nil {
			oldTags := normalizeHashtags(p.Hashtags)
			if len(oldTags) > 0 {
				p.Caption = strings.TrimSuffix(p.Caption, "\n\n#"+strings.Join(oldTags, " #"))
			}
			if len(normalized) > 0 {
				p.Caption += "\n\n#" + strings.Join(normalized, " #")
			}
		}
		p.Hashtags = normalized
	}
	if p.HeadlineSI == "" || utf8.RuneCountInString(p.HeadlineSI) > 90 || p.SummarySI == "" || utf8.RuneCountInString(p.SummarySI) > 280 || p.Caption == "" || utf8.RuneCountInString(p.Caption) > 5000 {
		return p, ErrInvalid
	}
	if !strings.Contains(p.Caption, "මූලාශ්‍ර: "+strings.Join(p.SourceNames, ", ")) {
		return p, fmt.Errorf("%w: caption must retain the source credits", ErrInvalid)
	}
	if _, err = tx.Exec(ctx, `UPDATE social_posts SET headline_si=$2,summary_si=$3,caption=$4,hashtags=$5,image_key=NULL,image_rendered_at=NULL,last_error=NULL,
 review_status='not_reviewed',review_score=NULL,review_summary='',review_issues='[]',review_llm_call_id=NULL,reviewed_at=NULL WHERE id=$1`, id, p.HeadlineSI, p.SummarySI, p.Caption, p.Hashtags); err != nil {
		return p, err
	}
	if err = audit(ctx, tx, actor, "social_post_edit", id); err != nil {
		return p, err
	}
	if err = tx.Commit(ctx); err != nil {
		return p, err
	}
	_ = service.render(ctx, id, uuid.Nil)
	return service.store.GetPost(ctx, id)
}

func (service *Service) Render(ctx context.Context, id string, actor uuid.UUID) (Post, error) {
	err := service.render(ctx, id, actor)
	p, loadErr := service.store.GetPost(ctx, id)
	if loadErr != nil {
		return p, loadErr
	}
	// A renderer failure is saved on the post so administrators can retry it.
	if err != nil && (errors.Is(err, ErrInvalid) || errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound)) {
		return p, err
	}
	return p, nil
}

func (service *Service) render(ctx context.Context, id string, actor uuid.UUID) error {
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	p, err := lockPost(ctx, tx, id)
	if err != nil {
		return err
	}
	if !editable(p.Status) {
		return ErrConflict
	}
	var renderError error
	var png []byte
	if service.renderer == nil || service.media == nil {
		renderError = errors.New("social image renderer is unavailable; configure Chrome and render again")
	} else {
		var spectrum *card.Spectrum
		if p.Spectrum != nil {
			spectrum = &card.Spectrum{Left: p.Spectrum.Left, Center: p.Spectrum.Center, Right: p.Spectrum.Right}
		}
		siteURL := ""
		if service.config.IncludeLink {
			siteURL = siteName(service.config.PublicBaseURL)
		}
		marks, marksErr := service.sourceMarks(ctx, p)
		if marksErr != nil {
			marks = nil // Fall back to names only; the card stays renderable.
		}
		kicker := p.Category
		if p.Kind == "update" && p.Storyline != nil {
			kicker = p.Storyline.Title
		}
		png, renderError = service.renderer.Render(ctx, card.Card{Kind: card.Kind(p.Kind), Kicker: kicker, Headline: p.HeadlineSI, Summary: p.SummarySI, Sources: p.SourceNames, Marks: marks, SourceCount: p.SourceCount, Spectrum: spectrum, PublishedAt: service.now(), SiteURL: siteURL})
		if renderError == nil && len(png) == 0 {
			renderError = errors.New("social renderer returned an empty image")
		}
	}
	key := "social/posts/" + id + ".png"
	if renderError == nil {
		renderError = service.media.Put(ctx, key, "image/png", png)
	}
	if renderError != nil {
		message := renderError.Error()
		if _, err = tx.Exec(ctx, `UPDATE social_posts SET image_key=NULL,image_rendered_at=NULL,last_error=$2 WHERE id=$1`, id, truncate(message, 1000)); err != nil {
			return err
		}
	} else {
		if _, err = tx.Exec(ctx, `UPDATE social_posts SET image_key=$2,image_rendered_at=$3,last_error=NULL WHERE id=$1`, id, key, service.now()); err != nil {
			return err
		}
	}
	if actor != uuid.Nil {
		if err = audit(ctx, tx, actor, "social_post_render", id); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if renderError != nil {
		return fmt.Errorf("render social card: %w", renderError)
	}
	return nil
}

// siteName turns the public base URL into the bare website name shown on cards.
func siteName(baseURL string) string {
	name := strings.TrimPrefix(strings.TrimPrefix(baseURL, "https://"), "http://")
	name = strings.TrimPrefix(name, "www.")
	if i := strings.IndexAny(name, "/?#"); i >= 0 {
		name = name[:i]
	}
	return name
}
