package social

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// resumeBatch caps how many waiting automatic posts one selection run settles, so new stories
// are still drafted within the job's time limit.
const resumeBatch = 4

// settle takes an automatic post through the AI quality gate with no editor involved. It
// reviews the post and rewrites it to fix what the reviewer found, up to maxAutoRevisions
// times. A post that passes is queued for publishing when auto-publish is on; a repeat of a
// recent post, or one that still fails, is dropped with the reason. When the model is
// unavailable the post waits and a later run settles it.
func (service *Service) settle(ctx context.Context, id string, settings Settings) error {
	p, err := service.store.GetPost(ctx, id)
	if err != nil {
		return err
	}
	if p.Status != "pending_review" {
		return nil
	}
	if p.ReviewStatus != "passed" {
		if p.ImageKey == nil {
			_ = service.render(ctx, id, uuid.Nil)
		}
		// A stored failed review is acted on directly; only a missing one is run again.
		result := reviewResult{Summary: p.ReviewSummary, Issues: p.ReviewIssues}
		if p.ReviewScore != nil {
			result.Score = *p.ReviewScore
		}
		if p.ReviewStatus != "failed" {
			if _, result, err = service.review(ctx, id); errors.Is(err, ErrModelUnavailable) {
				return nil
			} else if err != nil {
				return err
			}
		}
		for !approvable(result, settings.MinReviewScore) && revisable(result, settings.MinReviewScore) && p.ReviewRevisions < maxAutoRevisions {
			revised, err := service.revise(ctx, id, result)
			if errors.Is(err, ErrModelUnavailable) {
				return nil
			}
			if err != nil || !revised {
				break
			}
			_ = service.render(ctx, id, uuid.Nil)
			if _, result, err = service.review(ctx, id); errors.Is(err, ErrModelUnavailable) {
				return nil
			} else if err != nil {
				return err
			}
			if p, err = service.store.GetPost(ctx, id); err != nil {
				return err
			}
		}
		switch {
		case result.Repeats:
			return service.store.skip(ctx, id, "Not published: it repeats a recent post. "+result.Summary)
		case !approvable(result, settings.MinReviewScore):
			return service.store.skip(ctx, id, "Not published: it did not pass the AI review after "+strconv.Itoa(p.ReviewRevisions)+" automatic rewrites. "+result.Summary)
		}
	}
	if !settings.AutoPublish {
		return nil
	}
	approved, err := service.store.autoApprove(ctx, id, service.now())
	if err != nil {
		return err
	}
	if approved && service.enqueuePublisher != nil {
		_ = service.enqueuePublisher(ctx)
	}
	return nil
}

// resume settles automatic posts an earlier run left waiting: reviews the model could not run,
// posts held under the old topic rule, or posts that passed while auto-publish was off. Stories
// too old to post are dropped instead.
func (service *Service) resume(ctx context.Context, settings Settings) error {
	rows, err := service.store.pool.Query(ctx, `SELECT id::text,created_at FROM social_posts
 WHERE status='pending_review' AND trigger<>'manual' AND ($1 OR review_status<>'passed')
 ORDER BY kind='breaking' DESC,score DESC,created_at LIMIT $2`, settings.AutoPublish, resumeBatch)
	if err != nil {
		return err
	}
	type waiting struct {
		id      string
		created time.Time
	}
	posts := make([]waiting, 0, resumeBatch)
	for rows.Next() {
		var item waiting
		if err := rows.Scan(&item.id, &item.created); err != nil {
			rows.Close()
			return err
		}
		posts = append(posts, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, post := range posts {
		if post.created.Before(service.now().Add(-autoPublishMaxAge)) {
			err = service.store.skip(ctx, post.id, "Not published: the story was more than 12 hours old before it cleared review")
		} else {
			err = service.settle(ctx, post.id, settings)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// skip drops an automatic post that is still waiting, recording why.
func (store *Store) skip(ctx context.Context, id, reason string) error {
	_, err := store.pool.Exec(ctx, `UPDATE social_posts SET status='skipped',last_error=$2 WHERE id=$1 AND status='pending_review'`, id, truncate(reason, 1000))
	return err
}
