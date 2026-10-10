package social

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/card"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/meta"
)

type graphClient interface {
	AuthorizeURL(string) string
	ExchangeCode(context.Context, string) (meta.Token, error)
	ExchangeLongLived(context.Context, string) (meta.Token, error)
	ListPages(context.Context, string) ([]meta.Page, error)
	DebugToken(context.Context, string) (meta.TokenInfo, error)
	PublishPhoto(context.Context, string, string, []byte, string) (meta.Photo, error)
	Permalink(context.Context, string, string) (string, error)
}

type Service struct {
	store            *Store
	config           config.SocialConfig
	graph            graphClient
	cipher           *tokenCipher
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
	cipher, _ := newTokenCipher(configuration.TokenKey)
	graph := meta.NewClient(meta.Config{AppID: configuration.AppID, AppSecret: configuration.AppSecret, RedirectURL: configuration.RedirectURL, LoginConfigID: configuration.LoginConfigID, Version: configuration.GraphVersion}, nil)
	return &Service{store: store, config: configuration, graph: graph, cipher: cipher, model: model, renderer: renderer, media: mediaStore, now: now}
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

func (service *Service) Select(ctx context.Context) error {
	if err := service.clearExpiredStates(ctx); err != nil {
		return err
	}
	settings, err := service.store.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}
	candidates, err := service.store.Candidates(ctx, settings, service.now(), service.config.PublicBaseURL, "", "")
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}
	_, err = service.createFromCandidates(ctx, candidates, settings, false, uuid.Nil)
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
	posts, err := service.createFromCandidates(ctx, candidates, settings, true, actor)
	if err != nil {
		return Post{}, err
	}
	if len(posts) == 0 {
		return Post{}, ErrModelUnavailable
	}
	return posts[0], nil
}

func (service *Service) createFromCandidates(ctx context.Context, candidates []Candidate, settings Settings, manual bool, actor uuid.UUID) ([]Post, error) {
	items, callID, err := service.editorial(ctx, candidates)
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
		status := "skipped"
		if manual {
			if item.HeadlineSI == "" || item.SummarySI == "" || item.CaptionSI == "" || !containsSinhala(item.HeadlineSI) || !containsSinhala(item.SummarySI) || !containsSinhala(item.CaptionSI) {
				continue
			}
			status = "pending_review"
		} else if item.Publish && item.Score >= settings.MinScore {
			status = "pending_review"
			if settings.AutoPublish {
				status = "approved"
			}
		}
		p, err := service.insertPost(ctx, candidate, item, status, callID, manual, actor)
		if errors.Is(err, ErrConflict) {
			continue
		}
		if err != nil {
			return posts, err
		}
		if status != "skipped" {
			_ = service.render(ctx, p.ID, uuid.Nil)
			p, err = service.store.GetPost(ctx, p.ID)
			if err != nil {
				return posts, err
			}
		}
		posts = append(posts, p)
	}
	return posts, nil
}

func (service *Service) insertPost(ctx context.Context, candidate Candidate, item editorialItem, status string, callID *int64, manual bool, actor uuid.UUID) (Post, error) {
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
		err = tx.QueryRow(ctx, `SELECT evaluated_source_count,evaluated_breaking FROM social_posts WHERE (event_id::text=$1 OR article_id::text=$1) AND status='skipped' ORDER BY created_at DESC,id DESC LIMIT 1`, candidate.ID).Scan(&sources, &breaking)
		if err == nil && !reevaluateSkipped(candidate, sources, breaking) {
			return Post{}, ErrConflict
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Post{}, err
		}
	}
	if status == "approved" && !manual {
		var autoPublish bool
		if err = tx.QueryRow(ctx, `SELECT enabled AND auto_publish FROM social_settings WHERE singleton FOR SHARE`).Scan(&autoPublish); err != nil {
			return Post{}, err
		}
		// A long editorial call must not bypass an administrator who has since disabled auto-publish.
		if !autoPublish {
			status = "pending_review"
		}
	}
	id := uuid.NewString()
	kind := "top_story"
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
	caption := BuildCaption(item.CaptionSI, candidate.SourceNames, candidate.LinkURL, item.Hashtags)
	_, err = tx.Exec(ctx, `INSERT INTO social_posts (id,subject_kind,event_id,article_id,status,score,selection_reason,kind,
 headline_si,summary_si,caption,hashtags,link_url,source_names,source_count,spectrum,scheduled_for,approved_at,llm_call_id,
 evaluated_source_count,evaluated_breaking,created_by,evidence_article_ids)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$15,$20,$21,$22)`,
		id, candidate.SubjectKind, eventID, articleID, status, item.Score, item.Reason, kind, item.HeadlineSI, item.SummarySI, caption, item.Hashtags, candidate.LinkURL, candidate.SourceNames, candidate.SourceCount, candidate.Spectrum, scheduled, approved, callID, candidate.Breaking, creator, candidate.EvidenceArticleIDs)
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
