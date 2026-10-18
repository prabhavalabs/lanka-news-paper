package social

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/card"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/loomcast"
)

type Service struct {
	store            *Store
	config           config.SocialConfig
	loomcast         loomcastClient
	channelMu        sync.Mutex
	channelCache     channelLookup
	model            llm.Completer
	renderer         card.Renderer
	media            *media.Store
	now              func() time.Time
	enqueueSelection func(context.Context) error
	enqueuePublisher func(context.Context) error
}

func NewService(store *Store, configuration config.SocialConfig, model llm.Completer, renderer card.Renderer, mediaStore *media.Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	client := loomcast.NewClient(loomcast.Config{BaseURL: configuration.LoomcastURL, APIKey: configuration.LoomcastAPIKey}, nil)
	return &Service{store: store, config: configuration, loomcast: client, model: model, renderer: renderer, media: mediaStore, now: now}
}

func (service *Service) Store() *Store { return service.store }
func (service *Service) SetEnqueuers(selection, publisher func(context.Context) error) {
	service.enqueueSelection = selection
	service.enqueuePublisher = publisher
}

func (service *Service) RunSelection(ctx context.Context, actor uuid.UUID) error {
	if service.enqueueSelection == nil {
		return ErrNotConfigured
	}
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = audit(ctx, tx, actor, "social_run_selection", "queue"); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return service.enqueueSelection(ctx)
}

// Triggers record why a selection ran: the periodic schedule or new coverage arriving.
const (
	TriggerScheduled = "scheduled"
	TriggerRealtime  = "realtime"
	TriggerManual    = "manual"
)

func (service *Service) Select(ctx context.Context, trigger string) error {
	if trigger != TriggerRealtime {
		trigger = TriggerScheduled
	}
	settings, err := service.store.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled || (trigger == TriggerRealtime && !settings.RealtimeEnabled) {
		return nil
	}
	if err := service.resume(ctx, settings); err != nil {
		return err
	}
	candidates, err := service.store.Candidates(ctx, settings, service.now(), service.config.PublicBaseURL, "", "")
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	_, err = service.createFromCandidates(ctx, candidates, settings, trigger, uuid.Nil)
	if errors.Is(err, ErrModelUnavailable) {
		return nil
	}
	return err
}

func (service *Service) ManualPost(ctx context.Context, eventID, articleID string, actor uuid.UUID) (Post, error) {
	if (eventID == "") == (articleID == "") {
		return Post{}, ErrInvalid
	}
	settings, err := service.store.GetSettings(ctx)
	if err != nil {
		return Post{}, err
	}
	candidates, err := service.store.Candidates(ctx, settings, service.now(), service.config.PublicBaseURL, eventID, articleID)
	if err != nil {
		return Post{}, err
	}
	if len(candidates) == 0 {
		return Post{}, fmt.Errorf("%w: subject is unavailable, has no approved summary, or already has a queued post", ErrConflict)
	}
	posts, err := service.createFromCandidates(ctx, candidates, settings, TriggerManual, actor)
	if err != nil {
		return Post{}, err
	}
	if len(posts) == 0 {
		return Post{}, ErrModelUnavailable
	}
	return posts[0], nil
}

// createFromCandidates drafts posts and runs each through the AI quality gate. Nothing is
// approved at insert time. Automatic posts are settled with no editor: reviewed, rewritten
// until they pass, then queued (when auto-publish is on) or dropped. Manual posts wait in
// pending_review for the editor who asked for them.
func (service *Service) createFromCandidates(ctx context.Context, candidates []Candidate, settings Settings, trigger string, actor uuid.UUID) ([]Post, error) {
	manual := trigger == TriggerManual
	recent, err := service.store.recentPosts(ctx, service.now())
	if err != nil {
		return nil, err
	}
	items, callID, err := service.editorial(ctx, candidates, recent, activityOf(recent, service.now()))
	if err != nil {
		return nil, err
	}
	byID := map[string]Candidate{}
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	posts := make([]Post, 0, len(items))
	for _, item := range items {
		candidate := byID[item.ID]
		item.Storyline = candidate.Storyline
		item.Breaking = candidate.Breaking
		item.At = service.now()
		status := "skipped"
		if manual {
			if item.HeadlineSI == "" || item.SummarySI == "" || item.CaptionSI == "" || !containsSinhala(item.HeadlineSI) || !containsSinhala(item.SummarySI) || !containsSinhala(item.CaptionSI) {
				continue
			}
			status = "pending_review"
		} else if item.Publish && item.Score >= settings.MinScore {
			status = "pending_review"
			// The model is told about recent posts; this catches repeats it still lets through.
			if repeated := duplicateOf(item, recent); repeated != nil {
				status = "skipped"
				item.Reason = truncate("Repeats a recent post ("+repeated.Headline+"). "+item.Reason, 500)
			}
		}
		p, err := service.insertPost(ctx, candidate, item, status, callID, trigger, actor)
		if errors.Is(err, ErrConflict) {
			continue
		}
		if err != nil {
			return posts, err
		}
		if status != "skipped" {
			if manual {
				// An editor's post gets one review and, if needed, one rewrite; the editor decides.
				_ = service.render(ctx, p.ID, uuid.Nil)
				_, result, reviewErr := service.review(ctx, p.ID)
				if reviewErr == nil && !approvable(result, settings.MinReviewScore) && revisable(result, settings.MinReviewScore) {
					if revised, err := service.revise(ctx, p.ID, result); err == nil && revised {
						_ = service.render(ctx, p.ID, uuid.Nil)
						_, _, _ = service.review(ctx, p.ID)
					}
				}
			} else if err := service.settle(ctx, p.ID, settings); err != nil {
				return posts, err
			}
			p, err = service.store.GetPost(ctx, p.ID)
			if err != nil {
				return posts, err
			}
			recent = append(recent, recentPost{ID: p.ID, Headline: p.HeadlineSI, Summary: p.SummarySI, Category: candidate.Category,
				CategorySlug: candidate.CategorySlug, Breaking: candidate.Breaking, Status: p.Status, At: service.now(), StorylineID: storylineID(candidate.Storyline)})
		}
		posts = append(posts, p)
	}
	return posts, nil
}

func (service *Service) insertPost(ctx context.Context, candidate Candidate, item editorialItem, status string, callID *int64, trigger string, actor uuid.UUID) (Post, error) {
	manual := trigger == TriggerManual
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return Post{}, err
	}
	defer tx.Rollback(ctx)
	// Serialize subject evaluations so two overlapping selections cannot create duplicate skipped records.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "social:subject:"+candidate.SubjectKind+":"+candidate.ID); err != nil {
		return Post{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM social_posts WHERE (event_id::text=$1 OR article_id::text=$1) AND status NOT IN ('rejected','skipped','failed'))`, candidate.ID).Scan(&exists); err != nil {
		return Post{}, err
	}
	if exists {
		return Post{}, ErrConflict
	}
	if !manual {
		var sources int
		var breaking bool
		var previousKind string
		err = tx.QueryRow(ctx, `SELECT evaluated_source_count,evaluated_breaking,kind FROM social_posts WHERE (event_id::text=$1 OR article_id::text=$1) AND status='skipped' ORDER BY created_at DESC,id DESC LIMIT 1`, candidate.ID).Scan(&sources, &breaking, &previousKind)
		if err == nil && !reevaluateSkipped(candidate, sources, breaking) {
			var eligible bool
			if previousKind == "top_story" && candidate.SubjectKind == "event" && !candidate.Breaking {
				if err = tx.QueryRow(ctx, `SELECT `+storylineReevaluationSQL("$1::uuid", "$2::timestamptz"), candidate.ID, service.now()).Scan(&eligible); err != nil {
					return Post{}, err
				}
			}
			if !eligible {
				return Post{}, ErrConflict
			}
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Post{}, err
		}
	}
	id := uuid.NewString()
	kind := "top_story"
	if candidate.Storyline != nil && candidate.Storyline.Trending && !candidate.Breaking {
		var published bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM social_posts p JOIN storyline_events se ON se.event_id=p.event_id WHERE se.storyline_id=$1 AND p.status='published')`, candidate.Storyline.ID).Scan(&published); err != nil {
			return Post{}, err
		}
		if published {
			kind = "update"
		}
	}
	if candidate.Breaking {
		kind = "breaking"
	}
	var eventID, articleID *string
	if candidate.SubjectKind == "event" {
		eventID = &candidate.ID
	} else {
		articleID = &candidate.ID
	}
	var scheduled, approved *time.Time
	if status == "approved" {
		value := service.now()
		scheduled = &value
		approved = &value
	}
	creator := "selector"
	if manual {
		creator = actor.String()
	}
	// Captions never carry a URL; the card shows the website name instead.
	caption := BuildCaption(item.CaptionSI, candidate.SourceNames, "", item.Hashtags)
	_, err = tx.Exec(ctx, `INSERT INTO social_posts (id,subject_kind,event_id,article_id,status,score,selection_reason,kind,
 headline_si,summary_si,caption,hashtags,link_url,source_names,source_count,spectrum,scheduled_for,approved_at,llm_call_id,
 evaluated_source_count,evaluated_breaking,created_by,evidence_article_ids,trigger)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$15,$20,$21,$22,$23)`,
		id, candidate.SubjectKind, eventID, articleID, status, item.Score, item.Reason, kind, item.HeadlineSI, item.SummarySI, caption, item.Hashtags, candidate.LinkURL, candidate.SourceNames, candidate.SourceCount, candidate.Spectrum, scheduled, approved, callID, candidate.Breaking, creator, candidate.EvidenceArticleIDs, trigger)
	if err != nil {
		return Post{}, storeError(err)
	}
	if manual {
		if err = audit(ctx, tx, actor, "social_post_create", id); err != nil {
			return Post{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Post{}, storeError(err)
	}
	return service.store.GetPost(ctx, id)
}

func (service *Service) Transition(ctx context.Context, id, action string, actor uuid.UUID, scheduled *time.Time, confirm bool) (Post, error) {
	if action == "retry" {
		if err := service.withdrawFromLoomcast(ctx, id); err != nil {
			return Post{}, err
		}
	}
	p, err := service.store.transition(ctx, id, action, actor, scheduled, confirm, service.now())
	if err != nil {
		return p, err
	}
	if p.Status == "approved" && service.enqueuePublisher != nil {
		if err := service.enqueuePublisher(ctx); err != nil {
			return p, fmt.Errorf("post approved; publisher enqueue failed: %w", err)
		}
	}
	return p, nil
}

// withdrawFromLoomcast cancels a post's Loomcast copy before an editor retries it, so a retry
// can never publish the same story twice. A copy already out stops the retry.
func (service *Service) withdrawFromLoomcast(ctx context.Context, id string) error {
	p, err := service.store.GetPost(ctx, id)
	if err != nil || p.LoomcastPostID == nil || !service.config.Configured() {
		return err
	}
	_, err = service.loomcast.Cancel(ctx, *p.LoomcastPostID)
	var apiError *loomcast.APIError
	switch {
	case err == nil, errors.As(err, &apiError) && apiError.Status == 404:
		return nil
	case errors.As(err, &apiError) && apiError.Status == 409:
		return fmt.Errorf("%w: Loomcast has already published or is publishing this post; check the Page", ErrConflict)
	}
	return fmt.Errorf("withdraw the earlier Loomcast post: %w", err)
}
