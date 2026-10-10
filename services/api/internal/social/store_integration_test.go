package social

import (
	"bytes"
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
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social/meta"
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
	for _, table := range []string{"social_settings", "social_accounts", "social_oauth_states", "social_posts", "audit_logs", "admin_users", "sources", "source_endpoints", "rights_profiles", "articles", "categories", "event_clusters", "article_analysis_documents", "event_narrative_analyses"} {
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
	for _, table := range []string{"social_posts", "social_accounts"} {
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

type fakeGraph struct {
	pages      []meta.Page
	publishErr error
	uploads    int
	block      chan struct{}
	started    chan struct{}
}

func (*fakeGraph) AuthorizeURL(state string) string {
	return "https://www.facebook.com/dialog?state=" + state
}
func (*fakeGraph) ExchangeCode(context.Context, string) (meta.Token, error) {
	return meta.Token{AccessToken: "short"}, nil
}
func (*fakeGraph) ExchangeLongLived(context.Context, string) (meta.Token, error) {
	return meta.Token{AccessToken: "long"}, nil
}
func (graph *fakeGraph) ListPages(context.Context, string) ([]meta.Page, error) {
	return graph.pages, nil
}
func (*fakeGraph) DebugToken(context.Context, string) (meta.TokenInfo, error) {
	return meta.TokenInfo{IsValid: true, AppID: "app", Scopes: []string{"pages_show_list", "pages_read_engagement", "pages_manage_posts"}}, nil
}
func (graph *fakeGraph) PublishPhoto(context.Context, string, string, []byte, string) (meta.Photo, error) {
	graph.uploads++
	if graph.started != nil {
		close(graph.started)
		<-graph.block
	}
	return meta.Photo{ID: "photo", PostID: "page_post"}, graph.publishErr
}
func (*fakeGraph) Permalink(context.Context, string, string) (string, error) {
	return "https://www.facebook.com/page_post", nil
}

func integrationService(t *testing.T, store *Store, renderer card.Renderer, now time.Time) *Service {
	t.Helper()
	mediaStore, err := media.New(context.Background(), media.Config{LocalDirectory: t.TempDir()})
	require.NoError(t, err)
	service := NewService(store, config.SocialConfig{AppID: "app", AppSecret: "secret", TokenKey: bytes.Repeat([]byte{4}, 32), PublicBaseURL: "https://news.invalid"}, nil, renderer, mediaStore, func() time.Time { return now })
	service.graph = &fakeGraph{}
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

func TestSocialOAuthIntegration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	service := integrationService(t, store, fakeRenderer{}, now)
	actor := adminActor(t, store)
	other := adminActor(t, store)
	service.graph = &fakeGraph{pages: []meta.Page{{ID: "one", Name: "Page One", AccessToken: "token-one"}, {ID: "two", Name: "Page Two", AccessToken: "token-two"}}}
	authorize, err := service.Connect(ctx, actor)
	require.NoError(t, err)
	state := strings.Split(authorize, "state=")[1]
	outcome, stateID, err := service.Callback(ctx, "code", state, "")
	require.NoError(t, err)
	require.Equal(t, "choose_page", outcome)
	_, _, err = service.Callback(ctx, "code", state, "")
	require.ErrorIs(t, err, ErrOAuthState)
	pages, err := service.PendingPages(ctx, stateID, actor)
	require.NoError(t, err)
	require.Len(t, pages, 2)
	data, err := json.Marshal(pages)
	require.NoError(t, err)
	require.NotContains(t, string(data), "token-one")
	_, err = service.PendingPages(ctx, stateID, other)
	require.ErrorIs(t, err, ErrOAuthState)
	require.ErrorIs(t, service.SelectPage(ctx, stateID, "one", other), ErrOAuthState)
	require.NoError(t, service.SelectPage(ctx, stateID, "one", actor))
	require.ErrorIs(t, service.SelectPage(ctx, stateID, "one", actor), ErrOAuthState)
	account, err := store.Account(ctx)
	require.NoError(t, err)
	require.Equal(t, "one", account.PageID)
	var cipher, nonce []byte
	err = store.pool.QueryRow(ctx, `SELECT token_ciphertext,token_nonce FROM social_accounts WHERE page_id='one'`).Scan(&cipher, &nonce)
	require.NoError(t, err)
	require.NotContains(t, string(cipher), "token-one")
	token, err := service.cipher.decrypt(cipher, nonce, "social:one")
	require.NoError(t, err)
	require.Equal(t, "token-one", string(token))
	require.NoError(t, service.Disconnect(ctx, actor))
	err = store.pool.QueryRow(ctx, `SELECT token_ciphertext,token_nonce FROM social_accounts WHERE page_id='one'`).Scan(&cipher, &nonce)
	require.NoError(t, err)
	require.Empty(t, cipher)
	require.Empty(t, nonce)
	authorize, err = service.Connect(ctx, actor)
	require.NoError(t, err)
	state = strings.Split(authorize, "state=")[1]
	_, err = store.pool.Exec(ctx, `UPDATE social_oauth_states SET expires_at=now()-interval '1 minute' WHERE admin_user_id=$1 AND used_at IS NULL`, actor)
	require.NoError(t, err)
	_, _, err = service.Callback(ctx, "code", state, "")
	require.ErrorIs(t, err, ErrOAuthState)
	service.now = func() time.Time { return now }
	service.graph = &fakeGraph{pages: []meta.Page{{ID: "only", Name: "Only Page", AccessToken: "only-token"}}}
	authorize, err = service.Connect(ctx, actor)
	require.NoError(t, err)
	state = strings.Split(authorize, "state=")[1]
	outcome, _, err = service.Callback(ctx, "code", state, "")
	require.NoError(t, err)
	require.Equal(t, "connected", outcome)
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
	require.NoError(t, service.connectPage(ctx, meta.Page{ID: "page", Name: "Page", AccessToken: "page-token"}, actor, ""))
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

func TestSocialPublisherOutcomesIntegration(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		expected      string
		accountStatus string
	}{
		{"success", nil, "published", "connected"}, {"auth", &meta.Error{Code: 190}, "approved", "needs_reconnect"},
		{"rate limit", &meta.Error{Code: 4}, "approved", "connected"}, {"ambiguous", &meta.TransportError{Sent: true}, "needs_attention", "connected"},
		{"not sent", &meta.TransportError{Sent: false}, "approved", "connected"}, {"definite rejection", &meta.Error{Code: 100}, "failed", "connected"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, p, actor := publisherFixture(t)
			ctx := context.Background()
			graph := &fakeGraph{publishErr: test.err}
			service.graph = graph
			require.NoError(t, service.Publish(ctx))
			p, err := service.store.GetPost(ctx, p.ID)
			require.NoError(t, err)
			require.Equal(t, test.expected, p.Status)
			require.Equal(t, 1, p.Attempts)
			require.Equal(t, 1, graph.uploads)
			account, err := service.store.Account(ctx)
			require.NoError(t, err)
			require.Equal(t, test.accountStatus, account.Status)
			if test.expected == "published" {
				require.NotNil(t, p.FBPostID)
				require.NotNil(t, p.FBPermalink)
			}
			if test.err != nil {
				require.NotNil(t, p.LastError)
				require.NotContains(t, *p.LastError, "page-token")
			}
			if test.expected == "needs_attention" {
				_, err = service.Transition(ctx, p.ID, "retry", actor, nil, false)
				require.ErrorIs(t, err, ErrInvalid)
				p, err = service.Transition(ctx, p.ID, "retry", actor, nil, true)
				require.NoError(t, err)
				require.Equal(t, "approved", p.Status)
			}
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
	require.Equal(t, 0, service.graph.(*fakeGraph).uploads)
}

func TestSocialPublisherConcurrencyAndCapsIntegration(t *testing.T) {
	service, p, _ := publisherFixture(t)
	ctx := context.Background()
	graph := &fakeGraph{started: make(chan struct{}), block: make(chan struct{})}
	service.graph = graph
	var once sync.Once
	release := func() { once.Do(func() { close(graph.block) }) }
	defer release()
	finished := make(chan error, 1)
	go func() { finished <- service.Publish(ctx) }()
	select {
	case <-graph.started:
	case <-time.After(5 * time.Second):
		t.Fatal("publisher did not start")
	}
	require.NoError(t, service.Publish(ctx))
	release()
	require.NoError(t, <-finished)
	require.Equal(t, 1, graph.uploads)
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
	require.Equal(t, 1, graph.uploads)
}

func TestSocialPublisherRightsAndStaleClaimsIntegration(t *testing.T) {
	service, p, _ := publisherFixture(t)
	ctx := context.Background()
	_, err := service.store.pool.Exec(ctx, `UPDATE rights_profiles SET mode='disabled'`)
	require.NoError(t, err)
	require.NoError(t, service.Publish(ctx))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", p.Status)
	require.Equal(t, 0, p.Attempts)
	require.Contains(t, *p.LastError, "rights")
	_, err = service.store.pool.Exec(ctx, `ALTER TABLE social_posts DISABLE TRIGGER social_set_updated_at`)
	require.NoError(t, err)
	_, err = service.store.pool.Exec(ctx, `UPDATE social_posts SET status='publishing',updated_at=$2 WHERE id=$1`, p.ID, service.now().Add(-11*time.Minute))
	require.NoError(t, err)
	require.NoError(t, service.Publish(ctx))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "needs_attention", p.Status)
	require.Equal(t, 0, service.graph.(*fakeGraph).uploads)
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
	var input []Candidate
	require.NoError(t, json.Unmarshal([]byte(model.request.Input), &input))
	require.Len(t, input, 2)
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

func TestSocialOAuthAtomicConsumptionIntegration(t *testing.T) {
	store := integrationStore(t)
	ctx := context.Background()
	service := integrationService(t, store, nil, time.Now().UTC())
	actor := adminActor(t, store)
	authorize, err := service.Connect(ctx, actor)
	require.NoError(t, err)
	state := strings.Split(authorize, "state=")[1]
	type outcome struct {
		id    string
		actor uuid.UUID
		err   error
	}
	results := make(chan outcome, 8)
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			id, owner, err := service.consumeState(ctx, state)
			results <- outcome{id, owner, err}
		}()
	}
	group.Wait()
	close(results)
	consumed := 0
	for result := range results {
		if result.err == nil {
			consumed++
			require.NotEmpty(t, result.id)
			require.Equal(t, actor, result.actor)
		} else {
			require.ErrorIs(t, result.err, ErrOAuthState)
		}
	}
	require.Equal(t, 1, consumed)
	authorize, err = service.Connect(ctx, actor)
	require.NoError(t, err)
	state = strings.Split(authorize, "state=")[1]
	_, err = store.pool.Exec(ctx, `UPDATE admin_users SET role='editor' WHERE id=$1`, actor)
	require.NoError(t, err)
	_, _, err = service.Callback(ctx, "code", state, "")
	require.ErrorIs(t, err, ErrOAuthState)
}

func TestSocialRateLimitPausesOtherQueuedPostsIntegration(t *testing.T) {
	service, first, actor := publisherFixture(t)
	ctx := context.Background()
	graph := &fakeGraph{publishErr: &meta.Error{Code: 4}}
	service.graph = graph
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
	graph.publishErr = nil
	require.NoError(t, service.Publish(ctx))
	require.Equal(t, 1, graph.uploads)
	status, err := service.Status(ctx)
	require.NoError(t, err)
	require.True(t, service.now().Add(30*time.Minute).Equal(*status.NextPublishEligibleAt))
	now := service.now().Add(31 * time.Minute)
	service.now = func() time.Time { return now }
	require.NoError(t, service.Publish(ctx))
	require.Equal(t, 2, graph.uploads)
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
	require.Zero(t, service.graph.(*fakeGraph).uploads)
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
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := fixtureEvent(t, store, 2, now)
			text := fmt.Sprintf(`{"items":[{"id":%q,"score":90,"publish":true,"reason":"Important","headline_si":"පුවත","summary_si":"සාරාංශය","caption_si":"පුවත මෙන්න.","hashtags":[]}]}`, id)
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
	settings.RealtimeEnabled = false
	_, err = store.UpdateSettings(ctx, settings, actor)
	require.NoError(t, err)
	model := &editorialModel{response: llm.Response{Provider: "codex_cli", Text: `{"items":[]}`}}
	service.model = model
	fixtureEvent(t, store, 2, now)
	require.NoError(t, service.Select(ctx, TriggerRealtime))
	require.Zero(t, model.calls, "realtime runs are ignored when turned off")
}
