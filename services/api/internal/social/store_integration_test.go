package social

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/card"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/loomcast"
	"github.com/stretchr/testify/require"
)

func integrationStore(t *testing.T) *Store {
	t.Helper()
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	schema := "social_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = root.Exec(ctx, `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	// Shadow migrated tables to keep settings and queues isolated, including from other test packages.
	t.Cleanup(func() { _, _ = root.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`); root.Close() })
	for _, table := range []string{"storylines", "storyline_events", "social_settings", "social_posts", "audit_logs", "admin_users", "sources", "source_endpoints", "rights_profiles", "articles", "categories", "event_clusters", "article_analysis_documents", "event_narrative_analyses"} {
		_, err = root.Exec(ctx, `CREATE TABLE `+schema+`.`+table+` (LIKE public.`+table+` INCLUDING ALL)`)
		require.NoError(t, err)
	}
	configuration, err := pgxpool.ParseConfig(databaseURL)
	require.NoError(t, err)
	configuration.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `INSERT INTO social_settings (singleton) VALUES (true)`)
	require.NoError(t, err)
	for _, table := range []string{"social_posts"} {
		_, err = pool.Exec(ctx, `CREATE TRIGGER social_set_updated_at BEFORE UPDATE ON `+table+` FOR EACH ROW EXECUTE FUNCTION public.set_updated_at()`)
		require.NoError(t, err)
	}
	return NewStore(pool)
}

func fixtureEvent(t *testing.T, store *Store, sources int, now time.Time) string {
	t.Helper()
	id := uuid.NewString()
	_, err := store.pool.Exec(context.Background(), `INSERT INTO event_clusters (id,display_title,last_update_at) VALUES ($1,'සිදුවීම',$2)`, id, now)
	require.NoError(t, err)
	for i := 0; i < sources; i++ {
		fixtureArticle(t, store, id, "discovery_only", now)
	}
	return id
}

func fixtureArticle(t *testing.T, store *Store, eventID, mode string, now time.Time) string {
	t.Helper()
	ctx := context.Background()
	source, endpoint, rights, article := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	_, err := store.pool.Exec(ctx, `INSERT INTO sources (id,name,legal_name,source_type,active) VALUES ($1,$2,$2,'other',true)`, source, "Source "+source[:6])
	require.NoError(t, err)
	_, err = store.pool.Exec(ctx, `INSERT INTO source_endpoints (id,source_id,endpoint_type,url) VALUES ($1,$2,'rss','https://social.invalid/feed')`, endpoint, source)
	require.NoError(t, err)
	_, err = store.pool.Exec(ctx, `INSERT INTO rights_profiles (id,source_id,endpoint_id,mode,attribution,effective_from) VALUES ($1,$2,$3,$4,'Test',clock_timestamp())`, rights, source, endpoint, mode)
	require.NoError(t, err)
	var event *string
	if eventID != "" {
		event = &eventID
	}
	_, err = store.pool.Exec(ctx, `INSERT INTO articles (id,source_id,endpoint_id,rights_profile_id,source_item_id,original_url,canonical_url,headline,fingerprint,published_at,public_status,event_id) VALUES ($1::uuid,$2,$3,$4,$1::text,'https://social.invalid/story','https://social.invalid/story','සිංහල පුවත',$1::text,$5,'published',$6)`, article, source, endpoint, rights, now, event)
	require.NoError(t, err)
	_, err = store.pool.Exec(ctx, `INSERT INTO article_analysis_documents (article_id,original_text,cleaned_text,summary_text,cleaner_version) VALUES ($1,'original','cleaned','පුවත පිළිබඳ සාරාංශය.','test')`, article)
	require.NoError(t, err)
	return article
}

type fakeRenderer struct{ err error }

func (renderer fakeRenderer) Render(context.Context, card.Card) ([]byte, error) {
	if renderer.err != nil {
		return nil, renderer.err
	}
	return []byte("png-test-image"), nil
}

// fakeLoomcast stands in for Loomcast's machine API. Finalized posts stay queued until the test
// publishes them, as Loomcast does at their scheduled time.
type fakeLoomcast struct {
	mu          sync.Mutex
	channels    []loomcast.Channel
	createErr   error
	uploadErr   error
	finalizeErr error
	created     int
	finalized   int
	cancelled   []string
	keys        []string
	requests    []loomcast.NewPost
	posts       map[string]loomcast.Post
	block       chan struct{}
	started     chan struct{}
}

func newFakeLoomcast() *fakeLoomcast {
	return &fakeLoomcast{posts: map[string]loomcast.Post{}, channels: []loomcast.Channel{{ID: "fb-channel", Platform: "facebook", Name: "Thulawa News", Status: "active", Access: "publish", Formats: []string{"image"}}}}
}

func (fake *fakeLoomcast) Channels(context.Context) ([]loomcast.Channel, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return append([]loomcast.Channel(nil), fake.channels...), nil
}

func (fake *fakeLoomcast) CreatePost(_ context.Context, key string, post loomcast.NewPost) (loomcast.Post, error) {
	fake.mu.Lock()
	fake.created++
	fake.keys = append(fake.keys, key)
	fake.requests = append(fake.requests, post)
	started, block := fake.started, fake.block
	fake.started = nil
	fake.mu.Unlock()
	if started != nil {
		close(started)
		<-block
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.createErr != nil {
		return loomcast.Post{}, fake.createErr
	}
	id := fmt.Sprintf("lc-%d", fake.created)
	remote := loomcast.Post{ID: id, Status: "awaiting_media", ScheduledAt: post.ScheduledAt,
		Targets: []loomcast.Target{{ChannelID: post.Channels[0].ChannelID, Platform: "facebook", ScheduledAt: post.ScheduledAt}}}
	fake.posts[id] = remote
	remote.Uploads = []loomcast.Upload{{MediaID: "media", URL: "https://upload.invalid/" + id, Method: "PUT"}}
	return remote, nil
}

func (fake *fakeLoomcast) Upload(context.Context, loomcast.Upload, []byte) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.uploadErr
}

func (fake *fakeLoomcast) Finalize(_ context.Context, id string) (loomcast.Post, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.finalizeErr != nil {
		return loomcast.Post{}, fake.finalizeErr
	}
	fake.finalized++
	remote := fake.posts[id]
	remote.Status = "scheduled"
	queued := "queued"
	remote.Targets[0].Status = &queued
	fake.posts[id] = remote
	return remote, nil
}

func (fake *fakeLoomcast) Post(_ context.Context, id string) (loomcast.Post, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	remote, ok := fake.posts[id]
	if !ok {
		return loomcast.Post{}, &loomcast.APIError{Status: 404, Code: "not_found"}
	}
	return remote, nil
}

func (fake *fakeLoomcast) Cancel(_ context.Context, id string) (loomcast.Post, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.cancelled = append(fake.cancelled, id)
	remote, ok := fake.posts[id]
	if !ok {
		return loomcast.Post{}, &loomcast.APIError{Status: 404, Code: "not_found"}
	}
	if remote.Status == "published" {
		return loomcast.Post{}, &loomcast.APIError{Status: 409, Code: "post_not_editable"}
	}
	remote.Status = "cancelled"
	fake.posts[id] = remote
	return remote, nil
}

// settle moves every queued post to the given target status, as Loomcast does at its time.
func (fake *fakeLoomcast) settle(status string, targetError *loomcast.TargetError, dryRun bool) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for id, remote := range fake.posts {
		if remote.Status != "scheduled" {
			continue
		}
		value := status
		remote.Targets[0].Status = &value
		remote.Targets[0].LastError = targetError
		remote.Targets[0].DryRun = dryRun
		remote.Status = map[string]string{"published": "published", "failed": "failed", "missed": "failed", "cancelled": "cancelled"}[status]
		if status == "published" {
			externalID, permalink, at := "page_"+id, "https://www.facebook.com/page_"+id, remote.ScheduledAt
			remote.Targets[0].ExternalID, remote.Targets[0].Permalink, remote.Targets[0].PublishedAt = &externalID, &permalink, &at
		}
		fake.posts[id] = remote
	}
}

// syncNow reads back handed-off posts without handing over another, as the start of a run does.
func (service *Service) syncNow(ctx context.Context) error {
	connection, err := service.store.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer connection.Release()
	return service.syncHandedOff(ctx, connection)
}

// publishNow hands over the next post, lets Loomcast publish it and records the outcome.
func publishNow(t *testing.T, service *Service) {
	t.Helper()
	require.NoError(t, service.Publish(context.Background()))
	service.loomcast.(*fakeLoomcast).settle("published", nil, false)
	require.NoError(t, service.syncNow(context.Background()))
}

func integrationService(t *testing.T, store *Store, renderer card.Renderer, now time.Time) *Service {
	t.Helper()
	mediaStore, err := media.New(context.Background(), media.Config{LocalDirectory: t.TempDir()})
	require.NoError(t, err)
	service := NewService(store, config.SocialConfig{LoomcastAPIKey: "lc_api_test", PublicBaseURL: "https://news.invalid"}, nil, renderer, mediaStore, func() time.Time { return now })
	service.loomcast = newFakeLoomcast()
	return service
}

func adminActor(t *testing.T, store *Store) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := store.pool.Exec(context.Background(), `INSERT INTO admin_users (id,email,password_hash,display_name,role,status) VALUES ($1,$2,'test-hash','Social test','administrator','active')`, id, id.String()+"@example.invalid")
	require.NoError(t, err)
	return id
}

func TestSocialSelectionStoreIntegration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	settings := testSettings()
	event := fixtureEvent(t, store, 2, now)
	candidates, err := store.Candidates(ctx, settings, now, "https://news.invalid", "", "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, event, candidates[0].ID)
	require.Len(t, candidates[0].MemberSummaries, 2)
	require.Contains(t, candidates[0].Summary, "සාරාංශය")
	service := integrationService(t, store, fakeRenderer{}, now)
	item := editorialItem{ID: event, Score: 30, Reason: "Coverage is limited", Hashtags: []string{}}
	p, err := service.insertPost(ctx, candidates[0], item, "skipped", nil, TriggerScheduled, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, 2, p.EvaluatedSourceCount)
	candidates, err = store.Candidates(ctx, settings, now, "https://news.invalid", "", "")
	require.NoError(t, err)
	require.Empty(t, candidates)
	_, err = store.pool.Exec(ctx, `UPDATE event_clusters SET is_breaking=true WHERE id=$1`, event)
	require.NoError(t, err)
	candidates, err = store.Candidates(ctx, settings, now, "https://news.invalid", "", "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	_, err = service.insertPost(ctx, candidates[0], item, "skipped", nil, TriggerScheduled, uuid.Nil)
	require.NoError(t, err)
	candidates, err = store.Candidates(ctx, settings, now, "https://news.invalid", "", "")
	require.NoError(t, err)
	require.Empty(t, candidates)
	fixtureArticle(t, store, event, "discovery_only", now)
	fixtureArticle(t, store, event, "discovery_only", now)
	fixtureArticle(t, store, event, "internal_verification", now)
	candidates, err = store.Candidates(ctx, settings, now, "https://news.invalid", "", "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, 4, candidates[0].SourceCount)
	p, err = service.insertPost(ctx, candidates[0], editorialItem{ID: event, Score: 80, Reason: "Public interest", HeadlineSI: "සිංහල පුවත", SummarySI: "පුවත් සාරාංශය", CaptionSI: "පුවත මෙන්න.", Hashtags: []string{}}, "pending_review", nil, TriggerScheduled, uuid.Nil)
	require.NoError(t, err)
	candidates, err = store.Candidates(ctx, settings, now, "https://news.invalid", "", "")
	require.NoError(t, err)
	require.Empty(t, candidates)
	page, err := store.ListPosts(ctx, "pending_review", 1, "")
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, p.ID, page.Items[0].ID)
	all, err := store.ListPosts(ctx, "", 1, "")
	require.NoError(t, err)
	require.NotEmpty(t, all.NextCursor)
	more, err := store.ListPosts(ctx, "", 1, all.NextCursor)
	require.NoError(t, err)
	require.NotEqual(t, all.Items[0].ID, more.Items[0].ID)
}

func TestSocialRightsAndManualIntegration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	settings := testSettings()
	article := fixtureArticle(t, store, "", "discovery_only", now.Add(-24*time.Hour))
	fixtureArticle(t, store, "", "disabled", now)
	candidates, err := store.Candidates(ctx, settings, now, "https://news.invalid", "", "")
	require.NoError(t, err)
	require.Empty(t, candidates)
	candidates, err = store.Candidates(ctx, settings, now, "https://news.invalid", "", article)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	service := integrationService(t, store, nil, now)
	actor := adminActor(t, store)
	output := fmt.Sprintf(`{"items":[{"id":%q,"score":5,"publish":false,"reason":"Manual request","headline_si":"සිංහල පුවත","summary_si":"පුවත් සාරාංශය","caption_si":"පුවත මෙන්න. වැඩිදුර කියවන්න.","hashtags":[]}]}`, article)
	service.model = &editorialModel{response: llm.Response{Provider: "codex_cli", Text: output}}
	p, err := service.ManualPost(ctx, "", article, actor)
	require.NoError(t, err)
	require.Equal(t, "pending_review", p.Status)
	require.Equal(t, 5, p.Score)
	require.Nil(t, p.ImageKey)
	require.NotNil(t, p.LastError)
	service.renderer = fakeRenderer{}
	p, err = service.Render(ctx, p.ID, actor)
	require.NoError(t, err)
	require.NotNil(t, p.ImageKey)
	require.Contains(t, p.ImageURL, "/api/admin/media/social/posts/")
	caption := "නව පාඨය."
	_, err = service.Edit(ctx, p.ID, EditInput{Caption: &caption}, actor)
	require.ErrorIs(t, err, ErrInvalid)
	caption = BuildCaption(caption, p.SourceNames, p.LinkURL, p.Hashtags)
	p, err = service.Edit(ctx, p.ID, EditInput{Caption: &caption}, actor)
	require.NoError(t, err)
	require.Equal(t, caption, p.Caption)
	require.NotNil(t, p.ImageKey)
	category := uuid.NewString()
	_, err = store.pool.Exec(ctx, `INSERT INTO categories (id,slug,name_si,name_en,held) VALUES ($1::uuid,$1::text,'අත්හිටවූ','Held',true)`, category)
	require.NoError(t, err)
	held := fixtureArticle(t, store, "", "discovery_only", now)
	_, err = store.pool.Exec(ctx, `UPDATE articles SET category_id=$2 WHERE id=$1`, held, category)
	require.NoError(t, err)
	candidates, err = store.Candidates(ctx, settings, now, "https://news.invalid", "", held)
	require.NoError(t, err)
	require.Empty(t, candidates)
}

func publisherFixture(t *testing.T) (*Service, Post, uuid.UUID) {
	store := integrationStore(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	service := integrationService(t, store, fakeRenderer{}, now)
	actor := adminActor(t, store)
	settings := testSettings()
	_, err := store.UpdateSettings(ctx, settings, actor)
	require.NoError(t, err)
	fixtureEvent(t, store, 2, now)
	candidates, err := store.Candidates(ctx, settings, now, "https://news.invalid", "", "")
	require.NoError(t, err)
	p, err := service.insertPost(ctx, candidates[0], editorialItem{Score: 80, Reason: "Important", HeadlineSI: "පුවත", SummarySI: "සාරාංශය", CaptionSI: "පුවත මෙන්න.", Hashtags: []string{}}, "pending_review", nil, TriggerScheduled, uuid.Nil)
	require.NoError(t, err)
	p, err = service.Render(ctx, p.ID, actor)
	require.NoError(t, err)
	p, err = service.Transition(ctx, p.ID, "approve", actor, nil, false)
	require.NoError(t, err)
	return service, p, actor
}

func TestSocialPublisherHandsOffAndRecordsTheOutcomeIntegration(t *testing.T) {
	service, p, _ := publisherFixture(t)
	ctx := context.Background()
	fake := service.loomcast.(*fakeLoomcast)
	require.NoError(t, service.Publish(ctx))
	p, err := service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "publishing", p.Status)
	require.Equal(t, "lc-1", *p.LoomcastPostID)
	require.Equal(t, service.now().Add(loomcastLeadTime).UTC(), p.HandedOffAt.UTC())
	require.Equal(t, 1, fake.finalized)
	require.Equal(t, []string{"thulawa-social-" + p.ID + "-1"}, fake.keys)
	request := fake.requests[0]
	require.Equal(t, "image", request.Format)
	require.Equal(t, p.ID, request.ExternalRef)
	require.Equal(t, "fb-channel", request.Channels[0].ChannelID)
	require.Equal(t, p.Caption, request.Caption)
	require.Equal(t, "image/png", request.Media[0].MIME)
	require.Equal(t, len("png-test-image"), request.Media[0].Bytes)
	// The handed-off post counts against the minimum gap before Loomcast reports it live.
	status, err := service.Status(ctx)
	require.NoError(t, err)
	require.True(t, status.Ready)
	require.Equal(t, "Thulawa News", status.Channel.Name)
	require.True(t, status.NextPublishEligibleAt.After(service.now().Add(20*time.Minute)))
	require.NoError(t, service.Publish(ctx))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "publishing", p.Status, "still queued in Loomcast")
	fake.settle("published", nil, false)
	require.NoError(t, service.Publish(ctx))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "published", p.Status)
	require.Equal(t, "page_lc-1", *p.FBPostID)
	require.Equal(t, "https://www.facebook.com/page_lc-1", *p.FBPermalink)
	require.Nil(t, p.LastError)
	require.Equal(t, 1, fake.created)
}

func TestSocialPublisherHandOffFailuresIntegration(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*fakeLoomcast)
		expected string
		paused   bool
		cancel   bool
	}{
		{"unauthorized", func(fake *fakeLoomcast) { fake.createErr = &loomcast.APIError{Status: 401, Code: "unauthorized"} }, "approved", true, false},
		{"rate limited", func(fake *fakeLoomcast) {
			fake.createErr = &loomcast.APIError{Status: 429, Code: "rate_limited", RetryAfter: 5 * time.Minute}
		}, "approved", true, false},
		{"unreachable", func(fake *fakeLoomcast) { fake.createErr = &loomcast.TransportError{Op: "create"} }, "approved", false, false},
		{"refused", func(fake *fakeLoomcast) {
			fake.createErr = &loomcast.APIError{Status: 422, Code: "missing_artifact", Message: "The files do not fit the format"}
		}, "failed", false, false},
		{"upload lost", func(fake *fakeLoomcast) { fake.uploadErr = &loomcast.TransportError{Op: "upload"} }, "approved", false, true},
		{"finalize refused", func(fake *fakeLoomcast) {
			fake.finalizeErr = &loomcast.APIError{Status: 422, Code: "media_type_mismatch"}
		}, "failed", false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, p, _ := publisherFixture(t)
			ctx := context.Background()
			fake := service.loomcast.(*fakeLoomcast)
			test.setup(fake)
			require.NoError(t, service.Publish(ctx))
			p, err := service.store.GetPost(ctx, p.ID)
			require.NoError(t, err)
			require.Equal(t, test.expected, p.Status)
			require.Equal(t, 1, p.Attempts)
			require.Nil(t, p.LoomcastPostID)
			require.Contains(t, *p.LastError, "Loomcast")
			require.NotContains(t, *p.LastError, "lc_api_test")
			retryAt, err := service.store.publishRetryAt(ctx, service.store.pool)
			require.NoError(t, err)
			require.Equal(t, test.paused, retryAt != nil && retryAt.After(service.now()))
			require.Equal(t, test.cancel, len(fake.cancelled) == 1)
		})
	}
}

func TestSocialPublisherConfirmsAnUncertainFinalizeIntegration(t *testing.T) {
	service, p, _ := publisherFixture(t)
	ctx := context.Background()
	fake := service.loomcast.(*fakeLoomcast)
	fake.finalizeErr = &loomcast.TransportError{Op: "finalize"}
	require.NoError(t, service.Publish(ctx))
	p, err := service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "publishing", p.Status, "finalize may have taken effect")
	require.Empty(t, fake.cancelled)
	// Loomcast never finalized it, so the next run withdraws it and starts over.
	fake.finalizeErr = nil
	later := service.now().Add(time.Minute)
	service.now = func() time.Time { return later }
	require.NoError(t, service.syncNow(ctx))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", p.Status)
	require.Equal(t, []string{"lc-1"}, fake.cancelled)
	require.NoError(t, service.Publish(ctx))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "publishing", p.Status)
	require.Equal(t, "lc-2", *p.LoomcastPostID)
	require.Equal(t, "thulawa-social-"+p.ID+"-2", fake.keys[1])
}

func TestSocialPublisherRecordsLoomcastOutcomesIntegration(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		err      *loomcast.TargetError
		dryRun   bool
		expected string
		note     string
	}{
		{"definite failure", "failed", &loomcast.TargetError{Code: "invalid_media", Message: "Photo too large"}, false, "failed", "Photo too large"},
		{"passing failure", "failed", &loomcast.TargetError{Code: "platform", Message: "Facebook is busy", Retryable: true}, false, "approved", "Facebook is busy"},
		{"missed", "missed", nil, false, "failed", "Missed its time"},
		{"cancelled by the owner", "cancelled", nil, false, "failed", "Cancelled in Loomcast"},
		{"dry run", "published", nil, true, "failed", "dry run"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, p, _ := publisherFixture(t)
			ctx := context.Background()
			fake := service.loomcast.(*fakeLoomcast)
			require.NoError(t, service.Publish(ctx))
			fake.settle(test.status, test.err, test.dryRun)
			require.NoError(t, service.syncNow(ctx))
			p, err := service.store.GetPost(ctx, p.ID)
			require.NoError(t, err)
			require.Equal(t, test.expected, p.Status)
			require.Contains(t, *p.LastError, test.note)
			require.Nil(t, p.FBPermalink)
		})
	}
}

func TestSocialPublisherWaitsForApprovalAndStaleHandOffsIntegration(t *testing.T) {
	service, p, _ := publisherFixture(t)
	ctx := context.Background()
	fake := service.loomcast.(*fakeLoomcast)
	require.NoError(t, service.Publish(ctx))
	fake.mu.Lock()
	remote := fake.posts["lc-1"]
	remote.Status = "pending_approval"
	planned := "planned"
	remote.Targets[0].Status = &planned
	fake.posts["lc-1"] = remote
	fake.mu.Unlock()
	require.NoError(t, service.syncNow(ctx))
	p, err := service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "publishing", p.Status)
	require.Contains(t, *p.LastError, "approve it in Loomcast")
	later := service.now().Add(7 * time.Hour)
	service.now = func() time.Time { return later }
	require.NoError(t, service.syncNow(ctx))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "needs_attention", p.Status)
	// A retry withdraws the earlier copy so it can never go out twice.
	p, err = service.Transition(ctx, p.ID, "retry", uuid.New(), nil, true)
	require.NoError(t, err)
	require.Equal(t, "approved", p.Status)
	require.Nil(t, p.LoomcastPostID)
	require.Equal(t, []string{"lc-1"}, fake.cancelled)
}

func TestSocialPublisherRetryStopsWhenLoomcastPublishedIntegration(t *testing.T) {
	service, p, actor := publisherFixture(t)
	ctx := context.Background()
	fake := service.loomcast.(*fakeLoomcast)
	require.NoError(t, service.Publish(ctx))
	fake.settle("published", nil, false)
	_, err := service.store.pool.Exec(ctx, `UPDATE social_posts SET status='needs_attention' WHERE id=$1`, p.ID)
	require.NoError(t, err)
	_, err = service.Transition(ctx, p.ID, "retry", actor, nil, true)
	require.ErrorIs(t, err, ErrConflict)
}

func TestSocialPublisherHoldsWhenTheChannelIsNotReadyIntegration(t *testing.T) {
	for name, channel := range map[string]loomcast.Channel{
		"dry run":         {ID: "fb-channel", Platform: "facebook", Name: "Thulawa News", Status: "active", Access: "publish", DryRun: true},
		"needs reconnect": {ID: "fb-channel", Platform: "facebook", Name: "Thulawa News", Status: "needs_reauth", Access: "publish"},
		"read only":       {ID: "fb-channel", Platform: "facebook", Name: "Thulawa News", Status: "active", Access: "read"},
		"instagram only":  {ID: "ig-channel", Platform: "instagram", Name: "thulawa", Status: "active", Access: "publish"},
	} {
		t.Run(name, func(t *testing.T) {
			service, p, _ := publisherFixture(t)
			ctx := context.Background()
			fake := service.loomcast.(*fakeLoomcast)
			fake.channels = []loomcast.Channel{channel}
			require.NoError(t, service.Publish(ctx))
			require.Zero(t, fake.created)
			p, err := service.store.GetPost(ctx, p.ID)
			require.NoError(t, err)
			require.Equal(t, "approved", p.Status)
			status, err := service.Status(ctx)
			require.NoError(t, err)
			require.False(t, status.Ready)
			require.NotEmpty(t, status.Problem)
			require.Nil(t, status.NextPublishEligibleAt)
		})
	}
}

func TestSocialPublisherRefusesMissingImageIntegration(t *testing.T) {
	service, p, _ := publisherFixture(t)
	ctx := context.Background()
	_, err := service.store.pool.Exec(ctx, `UPDATE social_posts SET image_key=NULL WHERE id=$1`, p.ID)
	require.NoError(t, err)
	require.NoError(t, service.Publish(ctx))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", p.Status)
	require.Equal(t, 0, p.Attempts)
	require.NotNil(t, p.LastError)
	require.Zero(t, service.loomcast.(*fakeLoomcast).created)
}

func TestSocialPublisherConcurrencyAndCapsIntegration(t *testing.T) {
	service, p, _ := publisherFixture(t)
	ctx := context.Background()
	fake := service.loomcast.(*fakeLoomcast)
	fake.started, fake.block = make(chan struct{}), make(chan struct{})
	block := fake.block
	var once sync.Once
	release := func() { once.Do(func() { close(block) }) }
	defer release()
	started := fake.started
	finished := make(chan error, 1)
	go func() { finished <- service.Publish(ctx) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("publisher did not start")
	}
	require.NoError(t, service.Publish(ctx))
	release()
	require.NoError(t, <-finished)
	require.Equal(t, 1, fake.created)
	fake.settle("published", nil, false)
	require.NoError(t, service.syncNow(ctx))
	p, err := service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "published", p.Status)
	event := fixtureEvent(t, service.store, 2, service.now())
	settings, err := service.store.GetSettings(ctx)
	require.NoError(t, err)
	candidates, err := service.store.Candidates(ctx, settings, service.now(), "https://news.invalid", event, "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	second, err := service.insertPost(ctx, candidates[0], editorialItem{Score: 90, Reason: "Second story", HeadlineSI: "පුවත", SummarySI: "සාරාංශය", CaptionSI: "පුවත මෙන්න.", Hashtags: []string{}}, "approved", nil, TriggerScheduled, uuid.Nil)
	require.NoError(t, err)
	require.NoError(t, service.render(ctx, second.ID, uuid.Nil))
	require.NoError(t, service.Publish(ctx))
	require.Equal(t, 1, fake.created, "the minimum gap holds")
}

func TestSocialPublisherRightsAndInterruptedHandOffsIntegration(t *testing.T) {
	service, p, _ := publisherFixture(t)
	ctx := context.Background()
	fake := service.loomcast.(*fakeLoomcast)
	_, err := service.store.pool.Exec(ctx, `UPDATE rights_profiles SET mode='disabled'`)
	require.NoError(t, err)
	require.NoError(t, service.Publish(ctx))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", p.Status)
	require.Equal(t, 0, p.Attempts)
	require.Contains(t, *p.LastError, "rights")
	require.Zero(t, fake.created)
	// A process that stopped before saving Loomcast's post id finalized nothing, so it is queued again.
	_, err = service.store.pool.Exec(ctx, `UPDATE rights_profiles SET mode='discovery_only'`)
	require.NoError(t, err)
	_, err = service.store.pool.Exec(ctx, `ALTER TABLE social_posts DISABLE TRIGGER social_set_updated_at`)
	require.NoError(t, err)
	_, err = service.store.pool.Exec(ctx, `UPDATE social_posts SET status='publishing',handed_off_at=$2,updated_at=$2 WHERE id=$1`, p.ID, service.now().Add(-11*time.Minute))
	require.NoError(t, err)
	_, err = service.store.pool.Exec(ctx, `ALTER TABLE social_posts ENABLE TRIGGER social_set_updated_at`)
	require.NoError(t, err)
	require.NoError(t, service.syncNow(ctx))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", p.Status)
	require.Contains(t, *p.LastError, "interrupted")
}

func TestSocialManualRequiresExactlyOneSubject(t *testing.T) {
	service := &Service{}
	_, err := service.ManualPost(context.Background(), "", "", uuid.Nil)
	require.ErrorIs(t, err, ErrInvalid)
	_, err = service.ManualPost(context.Background(), uuid.NewString(), uuid.NewString(), uuid.Nil)
	require.ErrorIs(t, err, ErrInvalid)
}

func TestSocialSelectionBatchAndUnavailableModelIntegration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	service := integrationService(t, store, fakeRenderer{}, now)
	actor := adminActor(t, store)
	settings := testSettings()
	settings.AutoPublish = true
	_, err := store.UpdateSettings(ctx, settings, actor)
	require.NoError(t, err)
	first := fixtureEvent(t, store, 2, now)
	second := fixtureEvent(t, store, 3, now)
	service.model = &editorialModel{response: llm.Response{Provider: "none"}}
	require.NoError(t, service.Select(ctx, TriggerScheduled))
	var count int
	err = store.pool.QueryRow(ctx, `SELECT count(*)::int FROM social_posts`).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)
	output := fmt.Sprintf(`{"items":[{"id":%q,"score":80,"publish":true,"reason":"Important","headline_si":"පුවත","summary_si":"සාරාංශය","caption_si":"පුවත මෙන්න. වැඩිදුර කියවන්න.","hashtags":[]},{"id":%q,"score":20,"publish":false,"reason":"Not urgent","headline_si":"පුවත","summary_si":"සාරාංශය","caption_si":"පුවත මෙන්න.","hashtags":[]}]}`, first, second)
	model := &editorialModel{response: llm.Response{Provider: "codex_cli", Text: output}}
	service.model = model
	settings.Enabled = false
	_, err = store.UpdateSettings(ctx, settings, actor)
	require.NoError(t, err)
	require.NoError(t, service.Select(ctx, TriggerScheduled))
	require.Zero(t, model.calls)
	settings.Enabled = true
	_, err = store.UpdateSettings(ctx, settings, actor)
	require.NoError(t, err)
	require.NoError(t, service.Select(ctx, TriggerScheduled))
	require.Equal(t, 1, model.calls)
	var input struct {
		Candidates []Candidate `json:"candidates"`
	}
	require.NoError(t, json.Unmarshal([]byte(model.request.Input), &input))
	require.Len(t, input.Candidates, 2)
	page, err := store.ListPosts(ctx, "approved", 10, "")
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.NotNil(t, page.Items[0].ScheduledFor)
	require.NotNil(t, page.Items[0].ImageKey)
	skipped, err := store.ListPosts(ctx, "skipped", 10, "")
	require.NoError(t, err)
	require.Len(t, skipped.Items, 1)
	require.NoError(t, service.Select(ctx, TriggerScheduled))
	err = store.pool.QueryRow(ctx, `SELECT count(*)::int FROM social_posts`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestSocialRateLimitPausesOtherQueuedPostsIntegration(t *testing.T) {
	service, first, actor := publisherFixture(t)
	ctx := context.Background()
	fake := service.loomcast.(*fakeLoomcast)
	fake.createErr = &loomcast.APIError{Status: 429, Code: "rate_limited", RetryAfter: 30 * time.Minute}
	require.NoError(t, service.Publish(ctx))
	fixtureEvent(t, service.store, 2, service.now())
	settings, err := service.store.GetSettings(ctx)
	require.NoError(t, err)
	candidates, err := service.store.Candidates(ctx, settings, service.now(), "https://news.invalid", "", "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	p, err := service.insertPost(ctx, candidates[0], editorialItem{Score: 80, Reason: "Important", HeadlineSI: "පුවත", SummarySI: "සාරාංශය", CaptionSI: "පුවත", Hashtags: []string{}}, "pending_review", nil, TriggerScheduled, uuid.Nil)
	require.NoError(t, err)
	p, err = service.Render(ctx, p.ID, actor)
	require.NoError(t, err)
	_, err = service.Transition(ctx, p.ID, "approve", actor, nil, false)
	require.NoError(t, err)
	fake.createErr = nil
	require.NoError(t, service.Publish(ctx))
	require.Equal(t, 1, fake.created)
	status, err := service.Status(ctx)
	require.NoError(t, err)
	require.True(t, service.now().Add(30*time.Minute).Equal(*status.NextPublishEligibleAt))
	now := service.now().Add(31 * time.Minute)
	service.now = func() time.Time { return now }
	publishNow(t, service)
	require.Equal(t, 2, fake.created)
	published, err := service.store.GetPost(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, "published", published.Status)
}

func TestSocialPublisherChecksEveryEvidenceArticleIntegration(t *testing.T) {
	service, _, actor := publisherFixture(t)
	ctx := context.Background()
	// The same source has one permitted and one now-restricted article. Source totals alone still match.
	event := fixtureEvent(t, service.store, 2, service.now())
	extra := fixtureArticle(t, service.store, event, "discovery_only", service.now())
	_, err := service.store.pool.Exec(ctx, `UPDATE articles SET source_id=(SELECT source_id FROM articles WHERE event_id=$2 AND id<>$1 LIMIT 1) WHERE id=$1`, extra, event)
	require.NoError(t, err)
	settings, err := service.store.GetSettings(ctx)
	require.NoError(t, err)
	candidates, err := service.store.Candidates(ctx, settings, service.now(), "https://news.invalid", event, "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Len(t, candidates[0].EvidenceArticleIDs, 3)
	p, err := service.insertPost(ctx, candidates[0], editorialItem{Score: 80, Reason: "Important", HeadlineSI: "පුවත", SummarySI: "සාරාංශය", CaptionSI: "පුවත", Hashtags: []string{}}, "pending_review", nil, TriggerScheduled, uuid.Nil)
	require.NoError(t, err)
	_, err = service.Render(ctx, p.ID, actor)
	require.NoError(t, err)
	_, err = service.Transition(ctx, p.ID, "approve", actor, nil, false)
	require.NoError(t, err)
	// Remove the older fixture from the queue to exercise the new post.
	_, err = service.store.pool.Exec(ctx, `UPDATE social_posts SET status='rejected' WHERE id<>$1`, p.ID)
	require.NoError(t, err)
	_, err = service.store.pool.Exec(ctx, `UPDATE rights_profiles SET mode='disabled' WHERE id=(SELECT rights_profile_id FROM articles WHERE id=$1)`, extra)
	require.NoError(t, err)
	require.NoError(t, service.Publish(ctx))
	require.Zero(t, service.loomcast.(*fakeLoomcast).created)
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", p.Status)
	require.Contains(t, *p.LastError, "rights")
}

func TestSocialAutoPublishDisabledDuringEditorialIntegration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	service := integrationService(t, store, fakeRenderer{}, now)
	actor := adminActor(t, store)
	settings := testSettings()
	settings.AutoPublish = true
	_, err := store.UpdateSettings(ctx, settings, actor)
	require.NoError(t, err)
	id := fixtureEvent(t, store, 2, now)
	text := fmt.Sprintf(`{"items":[{"id":%q,"score":90,"publish":true,"reason":"Important","headline_si":"පුවත","summary_si":"සාරාංශය","caption_si":"පුවත","hashtags":[]}]}`, id)
	service.model = &editorialModel{response: llm.Response{Provider: "codex_cli", Text: text}, onComplete: func() {
		settings.AutoPublish = false
		_, err := store.UpdateSettings(ctx, settings, actor)
		require.NoError(t, err)
	}}
	require.NoError(t, service.Select(ctx, TriggerScheduled))
	posts, err := store.ListPosts(ctx, "", 10, "")
	require.NoError(t, err)
	require.Len(t, posts.Items, 1)
	require.Equal(t, "pending_review", posts.Items[0].Status)
	require.Nil(t, posts.Items[0].ApprovedAt)
	require.Nil(t, posts.Items[0].ScheduledFor)
}

func TestSocialReviewGateIntegration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	service := integrationService(t, store, fakeRenderer{}, now)
	actor := adminActor(t, store)
	settings := testSettings()
	settings.AutoPublish = true
	_, err := store.UpdateSettings(ctx, settings, actor)
	require.NoError(t, err)
	cases := []struct {
		name       string
		review     llm.Response
		err        error
		status     string
		wantReview string
	}{
		{"passes", llm.Response{}, nil, "approved", "passed"},
		{"fails", llm.Response{Provider: "codex_cli", Text: `{"pass":false,"score":40,"summary":"Allegation stated as fact.","issues":[{"severity":"blocker","field":"headline","message":"Attribute the allegation."}]}`}, nil, "pending_review", "failed"},
		{"low score", llm.Response{Provider: "codex_cli", Text: `{"pass":true,"score":60,"summary":"Weak Sinhala.","issues":[]}`}, nil, "pending_review", "failed"},
		{"unavailable", llm.Response{Provider: "none"}, nil, "pending_review", "unavailable"},
	}
	// Distinct places keep the drafts from being skipped as repeats of each other.
	places := []string{"කොළඹ", "මහනුවර", "ගාල්ල", "යාපනය"}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := fixtureEvent(t, store, 2, now)
			text := fmt.Sprintf(`{"items":[{"id":%q,"score":90,"publish":true,"reason":"Important","headline_si":%q,"summary_si":%q,"caption_si":"පුවත මෙන්න.","hashtags":[]}]}`, id, places[i], places[i])
			model := &editorialModel{response: llm.Response{Provider: "codex_cli", Text: text}, review: tc.review, reviewErr: tc.err}
			service.model = model
			require.NoError(t, service.Select(ctx, TriggerRealtime))
			require.Equal(t, 1, model.reviewCalls)
			var postID string
			require.NoError(t, store.pool.QueryRow(ctx, `SELECT id::text FROM social_posts WHERE event_id=$1`, id).Scan(&postID))
			p, err := store.GetPost(ctx, postID)
			require.NoError(t, err)
			require.Equal(t, tc.status, p.Status)
			require.Equal(t, tc.wantReview, p.ReviewStatus)
			require.Equal(t, TriggerRealtime, p.Trigger)
			require.Equal(t, "facebook", p.Channel)
			require.NotNil(t, p.ReviewedAt)
			if tc.status == "approved" {
				require.NotNil(t, p.ScheduledFor)
			} else {
				require.Nil(t, p.ApprovedAt)
			}
		})
	}
	t.Run("revised after review", func(t *testing.T) {
		// Keep the posts left waiting above out of this run's resume step.
		_, err := store.pool.Exec(ctx, `UPDATE social_posts SET status='rejected' WHERE status='pending_review'`)
		require.NoError(t, err)
		id := fixtureEvent(t, store, 2, now)
		text := fmt.Sprintf(`{"items":[{"id":%q,"score":90,"publish":true,"reason":"Important","headline_si":"මාතර","summary_si":"මාතර","caption_si":"පුවත මෙන්න.","hashtags":[]}]}`, id)
		model := &editorialModel{
			response: llm.Response{Provider: "codex_cli", Text: text},
			reviews:  []llm.Response{{Provider: "codex_cli", Text: `{"pass":false,"score":55,"summary":"Unattributed accusation.","issues":[{"severity":"major","field":"caption","message":"Attribute the accusation to the police."}]}`}},
			revision: llm.Response{Provider: "codex_cli", Text: `{"headline_si":"මාතර සිද්ධිය","summary_si":"පොලිසිය පවසයි","caption_si":"පොලිසිය පවසන පරිදි පුවත මෙන්න.","hashtags":[]}`},
		}
		service.model = model
		require.NoError(t, service.Select(ctx, TriggerRealtime))
		require.Equal(t, 1, model.revisions)
		require.Equal(t, 2, model.reviewCalls)
		var postID string
		require.NoError(t, store.pool.QueryRow(ctx, `SELECT id::text FROM social_posts WHERE event_id=$1`, id).Scan(&postID))
		p, err := store.GetPost(ctx, postID)
		require.NoError(t, err)
		require.Equal(t, "approved", p.Status)
		require.Equal(t, "passed", p.ReviewStatus)
		require.Equal(t, 1, p.ReviewRevisions)
		require.Equal(t, "මාතර සිද්ධිය", p.HeadlineSI)
		require.Contains(t, p.Caption, "පොලිසිය පවසන පරිදි")
		require.Contains(t, p.Caption, "වාර්තා කළේ:")
		require.NotContains(t, p.Caption, "https://")
	})
	settings.RealtimeEnabled = false
	_, err = store.UpdateSettings(ctx, settings, actor)
	require.NoError(t, err)
	model := &editorialModel{response: llm.Response{Provider: "codex_cli", Text: `{"items":[]}`}}
	service.model = model
	fixtureEvent(t, store, 2, now)
	require.NoError(t, service.Select(ctx, TriggerRealtime))
	require.Zero(t, model.calls, "realtime runs are ignored when turned off")
}

func TestStorylineCandidatesAndQueuedPostsIntegration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	now := time.Now()
	event := fixtureEvent(t, store, 3, now)
	saga := uuid.NewString()
	_, err := store.pool.Exec(ctx, `INSERT INTO storylines(id,slug,title_si,trending,trend_score) VALUES($1,$2,'අඛණ්ඩ පුවත්',true,150)`, saga, "story-"+saga)
	require.NoError(t, err)
	_, err = store.pool.Exec(ctx, `INSERT INTO storyline_events(storyline_id,event_id,added_by) VALUES($1,$2,'admin')`, saga, event)
	require.NoError(t, err)
	settings, err := store.GetSettings(ctx)
	require.NoError(t, err)
	candidates, err := store.Candidates(ctx, settings, now, "https://thulawa.test", event, "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	candidate := candidates[0]
	require.NotNil(t, candidate.Storyline)
	require.Equal(t, saga, candidate.Storyline.ID)
	require.True(t, candidate.Storyline.Trending)
	service := NewService(store, config.SocialConfig{}, nil, nil, nil, func() time.Time { return now })
	p, err := service.insertPost(ctx, candidate, editorialItem{Score: 90, HeadlineSI: "අලුත් වර්ධනය", SummarySI: "පුවත පිළිබඳ සාරාංශය", CaptionSI: "පුවත පිළිබඳ සාරාංශය", Hashtags: []string{}}, "approved", nil, TriggerScheduled, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, "top_story", p.Kind, "first storyline post keeps the normal kind")
	require.Equal(t, saga, p.Storyline.ID)
	recent, err := store.recentPosts(ctx, now)
	require.NoError(t, err)
	require.Len(t, recent, 1)
	require.Equal(t, saga, recent[0].StorylineID)
	queued, found, err := service.nextInQueue(ctx, store.pool)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, queued.Storyline)
	require.Equal(t, saga, queued.Storyline.ID)
}

func TestStorylineQueueSpacingBreakingAndDailyCapIntegration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	now := time.Now()
	service := NewService(store, config.SocialConfig{}, nil, nil, nil, func() time.Time { return now })
	id := uuid.NewString()
	_, err := store.pool.Exec(ctx, `INSERT INTO storylines(id,slug,title_si,trending) VALUES($1,$2,'අඛණ්ඩ පුවත්',true)`, id, "story-"+id)
	require.NoError(t, err)
	create := func(headline, summary string) Post {
		t.Helper()
		event := fixtureEvent(t, store, 3, now.Add(-time.Hour))
		_, err := store.pool.Exec(ctx, `INSERT INTO storyline_events(storyline_id,event_id,added_by) VALUES($1,$2,'admin')`, id, event)
		require.NoError(t, err)
		candidate := Candidate{ID: event, SubjectKind: "event", Storyline: &StorylineInfo{ID: id, Trending: true}, SourceCount: 3, SourceNames: []string{}, EvidenceArticleIDs: []string{}}
		post, err := service.insertPost(ctx, candidate, editorialItem{Score: 90, HeadlineSI: headline, SummarySI: summary, CaptionSI: summary, Hashtags: []string{}}, "approved", nil, TriggerScheduled, uuid.Nil)
		require.NoError(t, err)
		return post
	}
	first := create("විරෝධතාකරුවන් නුවරින් පිටත් වුණා", "පළමු පුවත")
	require.Equal(t, "top_story", first.Kind)
	_, err = store.pool.Exec(ctx, `UPDATE social_posts SET status='published',published_at=$2 WHERE id=$1`, first.ID, now.Add(-4*time.Hour))
	require.NoError(t, err)
	update := create("අධිකරණය ඇප ඉල්ලීම සලකා බලයි", "විනිසුරු නියෝගය හෙට ලැබේ")
	require.Equal(t, "update", update.Kind)
	queued, found, err := service.nextInQueue(ctx, store.pool)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, update.ID, queued.ID)
	_, err = store.pool.Exec(ctx, `UPDATE social_posts SET published_at=$2 WHERE id=$1`, first.ID, now.Add(-time.Hour))
	require.NoError(t, err)
	_, found, err = service.nextInQueue(ctx, store.pool)
	require.NoError(t, err)
	require.False(t, found, "saga spacing defers ordinary updates")
	post, err := store.GetPost(ctx, update.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", post.Status, "cooldown does not discard a queued post")
	_, err = store.pool.Exec(ctx, `UPDATE social_posts SET kind='breaking' WHERE id=$1`, update.ID)
	require.NoError(t, err)
	queued, found, err = service.nextInQueue(ctx, store.pool)
	require.NoError(t, err)
	require.True(t, found, "queued breaking news bypasses spacing")
	require.Equal(t, update.ID, queued.ID)
	_, err = store.pool.Exec(ctx, `UPDATE social_posts SET kind=CASE WHEN id=$1 THEN 'breaking' ELSE 'update' END WHERE id=ANY($2::uuid[])`, first.ID, []string{first.ID, update.ID})
	require.NoError(t, err)
	_, found, err = service.nextInQueue(ctx, store.pool)
	require.NoError(t, err)
	require.True(t, found, "earlier breaking news does not impose saga spacing")
	published := []string{}
	for range storylineDailyUpdateCap {
		p := create("විදුලි ජනන ව්‍යාපෘතිය අරඹයි", "ඉන්ධන සැපයුම යළි ක්‍රියාත්මකයි")
		published = append(published, p.ID)
		_, err = store.pool.Exec(ctx, `UPDATE social_posts SET status='published',published_at=$2 WHERE id=$1`, p.ID, now.Add(-4*time.Hour))
		require.NoError(t, err)
	}
	_, found, err = service.nextInQueue(ctx, store.pool)
	require.NoError(t, err)
	require.False(t, found, "four published updates exhaust the rolling daily cap")
	post, err = store.GetPost(ctx, update.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", post.Status)
	_, err = store.pool.Exec(ctx, `UPDATE social_posts SET published_at=$2 WHERE id=$1`, published[0], now.Add(-25*time.Hour))
	require.NoError(t, err)
	_, found, err = service.nextInQueue(ctx, store.pool)
	require.NoError(t, err)
	require.True(t, found, "cap clears when an update leaves the rolling window")
}

func TestTrendingReevaluationOnlyLatestRecentEventIntegration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	now := time.Now()
	service := NewService(store, config.SocialConfig{}, nil, nil, nil, func() time.Time { return now })
	events := []string{}
	for _, age := range []time.Duration{7 * time.Hour, 2 * time.Hour, time.Hour} {
		event := fixtureEvent(t, store, 3, now.Add(-age))
		events = append(events, event)
		_, err := service.insertPost(ctx, Candidate{ID: event, SubjectKind: "event", SourceCount: 3, SourceNames: []string{}, EvidenceArticleIDs: []string{}}, editorialItem{Hashtags: []string{}}, "skipped", nil, TriggerScheduled, uuid.Nil)
		require.NoError(t, err)
	}
	id := uuid.NewString()
	_, err := store.pool.Exec(ctx, `INSERT INTO storylines(id,slug,title_si,trending) VALUES($1,$2,'අඛණ්ඩ පුවත්',true)`, id, "story-"+id)
	require.NoError(t, err)
	for _, event := range events {
		_, err = store.pool.Exec(ctx, `INSERT INTO storyline_events(storyline_id,event_id,added_by) VALUES($1,$2,'admin')`, id, event)
		require.NoError(t, err)
	}
	settings, err := store.GetSettings(ctx)
	require.NoError(t, err)
	candidates, err := store.Candidates(ctx, settings, now, "https://thulawa.test", "", "")
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, events[2], candidates[0].ID)
	// The write path repeats the bounded eligibility check for stale candidate snapshots.
	_, err = service.insertPost(ctx, Candidate{ID: events[1], SubjectKind: "event", SourceNames: []string{}, EvidenceArticleIDs: []string{}, SourceCount: 3, Storyline: &StorylineInfo{ID: id, Trending: true}}, editorialItem{Hashtags: []string{}}, "skipped", nil, TriggerScheduled, uuid.Nil)
	require.ErrorIs(t, err, ErrConflict)
	_, err = service.insertPost(ctx, candidates[0], editorialItem{Hashtags: []string{}}, "skipped", nil, TriggerScheduled, uuid.Nil)
	require.NoError(t, err)
	_, err = store.pool.Exec(ctx, `UPDATE articles SET published_at=$1`, now.Add(-7*time.Hour))
	require.NoError(t, err)
	candidates, err = store.Candidates(ctx, settings, now, "https://thulawa.test", "", "")
	require.NoError(t, err)
	require.Empty(t, candidates, "no old development is re-evaluated merely for starting to trend")
}
