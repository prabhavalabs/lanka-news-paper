package storyline

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/stretchr/testify/require"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("SNAP_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	schema := "storyline_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = root.Exec(ctx, `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = root.Exec(context.Background(), `DROP SCHEMA `+schema+` CASCADE`); root.Close() })
	for _, table := range []string{"articles", "event_clusters", "sources", "source_endpoints", "rights_profiles", "categories", "article_analysis_documents", "event_narrative_analyses", "story_reports", "social_posts", "llm_task_profiles", "agent_workflows", "agent_workflow_versions"} {
		_, err = root.Exec(ctx, `CREATE TABLE `+schema+`.`+table+` (LIKE public.`+table+` INCLUDING ALL)`)
		require.NoError(t, err)
	}
	config, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)
	config.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	up, err := os.ReadFile("../../migrations/000066_storylines.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	controls, err := os.ReadFile("../../migrations/000067_storyline_editorial_controls.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(controls))
	require.NoError(t, err)
	return NewStore(pool)
}

func fixture(t *testing.T, s *Store, at time.Time) (string, []string) {
	t.Helper()
	ctx := context.Background()
	event := uuid.NewString()
	_, err := s.pool.Exec(ctx, `INSERT INTO event_clusters(id,display_title,last_update_at) VALUES($1,'පුවත් වර්ධනය',$2)`, event, at)
	require.NoError(t, err)
	ids := []string{}
	for n := 0; n < 4; n++ {
		source, endpoint, rights, id := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
		_, err = s.pool.Exec(ctx, `INSERT INTO sources(id,name,legal_name,source_type,active) VALUES($1,'Outlet','Outlet','other',true)`, source)
		require.NoError(t, err)
		_, err = s.pool.Exec(ctx, `INSERT INTO source_endpoints(id,source_id,endpoint_type,url) VALUES($1,$2,'rss','https://storyline.invalid/feed')`, endpoint, source)
		require.NoError(t, err)
		_, err = s.pool.Exec(ctx, `INSERT INTO rights_profiles(id,source_id,endpoint_id,mode,attribution,effective_from) VALUES($1,$2,$3,'discovery_only','Test',now())`, rights, source, endpoint)
		require.NoError(t, err)
		_, err = s.pool.Exec(ctx, `INSERT INTO articles(id,source_id,endpoint_id,rights_profile_id,source_item_id,original_url,canonical_url,headline,fingerprint,published_at,public_status,event_id) VALUES($1::uuid,$2,$3,$4,$1::text,'https://storyline.invalid/a','https://storyline.invalid/a','ගමන කොළඹ',$1::text,$5,'published',$6)`, id, source, endpoint, rights, at, event)
		require.NoError(t, err)
		_, err = s.pool.Exec(ctx, `INSERT INTO article_analysis_documents(article_id,original_text,cleaned_text,summary_text,cleaner_version) VALUES($1,'a','a','පුවත පිළිබඳ සාරාංශය','test')`, id)
		require.NoError(t, err)
		ids = append(ids, id)
	}
	return event, ids
}

type groupingModel struct {
	mu          sync.Mutex
	groups      []Group
	unavailable bool
	seen        int
}

func (m *groupingModel) Complete(_ context.Context, r llm.Request) (llm.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.unavailable {
		return llm.Response{}, errors.New("offline")
	}
	var input struct {
		Events []InputEvent `json:"events"`
	}
	_ = json.Unmarshal([]byte(r.Input), &input)
	m.seen = len(input.Events)
	data, _ := json.Marshal(map[string]any{"groups": m.groups})
	return llm.Response{Provider: "test", Text: string(data)}, nil
}

func TestStorylineRefreshLifecycleIntegration(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	a, _ := fixture(t, s, now.Add(-time.Hour))
	b, _ := fixture(t, s, now.Add(-2*time.Hour))
	old, _ := fixture(t, s, now.Add(-5*24*time.Hour))
	model := &groupingModel{groups: []Group{{Title: "ගමන පිළිබඳ පුවත්", Summary: "පුවතේ අලුත් වර්ධනය", EventIDs: []string{a, b, old}}}}
	service := NewService(s, model)
	require.NoError(t, service.Refresh(ctx, 48*time.Hour))
	require.Equal(t, 3, model.seen, "first refresh includes the five-day-old event")
	items, err := s.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, items, 1)
	story := items[0]
	require.True(t, story.Trending)
	require.Equal(t, 3, story.EventCount)
	require.Equal(t, 12, story.ArticleCount)
	require.Equal(t, 8, story.Metrics.Articles24h)
	require.NotNil(t, story.TrendingSince)
	require.NotEmpty(t, story.LatestEvent.Summary)
	again, err := s.Get(ctx, story.Slug, false)
	require.NoError(t, err)
	require.Equal(t, story.ID, again.ID)
	require.NoError(t, service.Refresh(ctx, 48*time.Hour))
	var count int
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT count(*) FROM storylines`).Scan(&count))
	require.Equal(t, 1, count)
	title := "පරිපාලක නම"
	_, err = s.Update(ctx, story.ID, Patch{Title: &title})
	require.NoError(t, err)
	next, _ := fixture(t, s, now)
	model.groups = []Group{{StorylineID: story.ID, Title: "අලුත් නම", Summary: "අලුත් සාරාංශය", EventIDs: []string{next}}}
	require.NoError(t, service.Refresh(ctx, 48*time.Hour))
	again, err = s.Get(ctx, story.ID, true)
	require.NoError(t, err)
	require.Equal(t, title, again.Title)
	require.Equal(t, "අලුත් සාරාංශය", again.Summary)
	other := uuid.NewString()
	_, err = s.pool.Exec(ctx, `INSERT INTO storylines(id,slug,title_si) VALUES($1,$2,'වෙනත් පුවත්')`, other, "story-"+other)
	require.NoError(t, err)
	require.ErrorIs(t, s.Membership(ctx, other, a, false), ErrConflict)
	// Even stale model output cannot move an already assigned event.
	tx, err := s.pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, applyGroups(ctx, tx, []Group{{StorylineID: other, Title: "වෙනත්", EventIDs: []string{a}}}, now))
	require.NoError(t, tx.Commit(ctx))
	var owner string
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT storyline_id::text FROM storyline_events WHERE event_id=$1`, a).Scan(&owner))
	require.Equal(t, story.ID, owner)
	hidden := true
	_, err = s.Update(ctx, story.ID, Patch{Hidden: &hidden})
	require.NoError(t, err)
	_, err = s.Get(ctx, story.ID, false)
	require.ErrorIs(t, err, ErrNotFound)
	_, _ = fixture(t, s, now) // An unassigned event forces the unavailable grouping path.
	model.unavailable = true
	require.NoError(t, service.Refresh(ctx, 48*time.Hour), "unavailable model still rescores")
	hidden = false
	pinned := true
	_, err = s.Update(ctx, story.ID, Patch{Hidden: &hidden, Pinned: &pinned})
	require.NoError(t, err)
	_, err = s.pool.Exec(ctx, `UPDATE articles SET published_at=$1`, now.Add(-9*24*time.Hour))
	require.NoError(t, err)
	require.NoError(t, service.Refresh(ctx, 48*time.Hour))
	again, err = s.Get(ctx, story.ID, true)
	require.NoError(t, err)
	require.True(t, again.Trending)
	require.Equal(t, "active", again.Status)
	pinned = false
	again, err = s.Update(ctx, story.ID, Patch{Pinned: &pinned})
	require.NoError(t, err)
	require.Equal(t, "archived", again.Status)
	require.False(t, again.Trending)
	require.Nil(t, again.TrendingSince)
	require.NoError(t, s.Membership(ctx, story.ID, next, true))
	require.NoError(t, s.Membership(ctx, other, next, false))
}

func TestConcurrentRefreshIntegration(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	a, _ := fixture(t, s, now)
	b, _ := fixture(t, s, now)
	c, _ := fixture(t, s, now)
	model := &groupingModel{groups: []Group{{Title: "පුවත් පෙළ", EventIDs: []string{a, b, c}}}}
	service := NewService(s, model)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- service.Refresh(context.Background(), 48*time.Hour) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count int
	require.NoError(t, s.pool.QueryRow(context.Background(), `SELECT count(*) FROM storylines`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestStorylineMigrationRoundTripIntegration(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	var provider, model, effort string
	var timeout int
	var enabled bool
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT provider_id,model,reasoning_effort,timeout_seconds,enabled FROM llm_task_profiles WHERE task='storyline_grouping'`).Scan(&provider, &model, &effort, &timeout, &enabled))
	require.Equal(t, "codex_cli", provider)
	require.Equal(t, "gpt-6-luna", model)
	require.Equal(t, "medium", effort)
	require.Equal(t, 180, timeout)
	require.True(t, enabled)
	var count int
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT count(*) FROM agent_workflow_versions WHERE task='storyline_grouping'`).Scan(&count))
	require.Equal(t, 1, count)
	controlsDown, err := os.ReadFile("../../migrations/000067_storyline_editorial_controls.down.sql")
	require.NoError(t, err)
	_, err = s.pool.Exec(ctx, string(controlsDown))
	require.NoError(t, err)
	down, err := os.ReadFile("../../migrations/000066_storylines.down.sql")
	require.NoError(t, err)
	_, err = s.pool.Exec(ctx, string(down))
	require.NoError(t, err)
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT count(*) FROM agent_workflows WHERE task='storyline_grouping'`).Scan(&count))
	require.Zero(t, count)
	up, err := os.ReadFile("../../migrations/000066_storylines.up.sql")
	require.NoError(t, err)
	_, err = s.pool.Exec(ctx, string(up))
	require.NoError(t, err)
	_, err = s.pool.Exec(ctx, `INSERT INTO storylines(slug,title_si) VALUES('Bad slug','පුවත්')`)
	require.Error(t, err)
}

func TestPublicRightsAndAdminMembershipIntegration(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	event, articles := fixture(t, s, time.Now())
	id := uuid.NewString()
	_, err := s.pool.Exec(ctx, `INSERT INTO storylines(id,slug,title_si,pinned) VALUES($1,$2,'පුවත් පෙළ',true)`, id, "story-"+id)
	require.NoError(t, err)
	require.NoError(t, s.Membership(ctx, id, event, false))
	_, err = s.pool.Exec(ctx, `UPDATE rights_profiles SET mode='disabled' WHERE id IN (SELECT rights_profile_id FROM articles WHERE id=ANY($1::uuid[]))`, articles)
	require.NoError(t, err)
	_, err = s.Get(ctx, id, false)
	require.ErrorIs(t, err, ErrNotFound)
	items, err := s.List(ctx, false)
	require.NoError(t, err)
	require.Empty(t, items)
	admin, err := s.Get(ctx, id, true)
	require.NoError(t, err)
	require.Equal(t, 1, admin.EventCount, "admin can remove a member whose reports are no longer public")
	require.NoError(t, s.Membership(ctx, id, event, true))
}

type modelFunc func(context.Context, llm.Request) (llm.Response, error)

func (f modelFunc) Complete(ctx context.Context, r llm.Request) (llm.Response, error) {
	return f(ctx, r)
}

func TestAdminEditsDuringGroupingIntegration(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now()
	a, _ := fixture(t, s, now.Add(-time.Hour))
	b, _ := fixture(t, s, now.Add(-30*time.Minute))
	id := uuid.NewString()
	_, err := s.pool.Exec(ctx, `INSERT INTO storylines(id,slug,title_si) VALUES($1,$2,'පුවත් පෙළ')`, id, "story-"+id)
	require.NoError(t, err)
	require.NoError(t, s.Membership(ctx, id, a, false))
	entered, release := make(chan struct{}), make(chan struct{})
	model := modelFunc(func(callCtx context.Context, _ llm.Request) (llm.Response, error) {
		close(entered)
		select {
		case <-release:
		case <-callCtx.Done():
			return llm.Response{}, callCtx.Err()
		}
		data, err := json.Marshal(map[string]any{"groups": []Group{{StorylineID: id, Title: "අලුත් නම", Summary: "අලුත් සාරාංශය", EventIDs: []string{b}}}})
		return llm.Response{Provider: "test", Text: string(data)}, err
	})
	refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- NewService(s, model).Refresh(refreshCtx, 48*time.Hour) }()
	select {
	case <-entered:
	case <-refreshCtx.Done():
		t.Fatal("model did not start")
	}
	// The model remains blocked until both admin operations return.
	editCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	defer stop()
	summary := "පරිපාලක සාරාංශය"
	_, err = s.Update(editCtx, id, Patch{Summary: &summary})
	require.NoError(t, err, "admin summary must not wait for the model")
	require.NoError(t, s.Membership(editCtx, id, a, true), "admin membership must not wait for the model")
	close(release)
	require.NoError(t, <-done)
	story, err := s.Get(ctx, id, true)
	require.NoError(t, err)
	require.True(t, story.SummaryLocked)
	require.Equal(t, summary, story.Summary)
	require.Len(t, story.Events, 1)
	require.Equal(t, b, story.Events[0].ID)
	events, _, _, err := groupingInput(ctx, s.pool, now.Add(time.Minute), 48*time.Hour)
	require.NoError(t, err)
	require.Empty(t, events, "removed events stay out of model input")
	// Stale output cannot reattach a removed event or seed a replacement saga with it.
	tx, err := s.pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, applyGroups(ctx, tx, []Group{{StorylineID: id, Title: "පුවත්", EventIDs: []string{a}}}, now))
	require.NoError(t, tx.Commit(ctx))
	story, err = s.Get(ctx, id, true)
	require.NoError(t, err)
	require.Len(t, story.Events, 1)
	require.NoError(t, s.Membership(ctx, id, a, false))
	var excluded bool
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM storyline_event_exclusions WHERE event_id=$1)`, a).Scan(&excluded))
	require.False(t, excluded, "manual add clears the exclusion")
	archived := "archived"
	story, err = s.Update(ctx, id, Patch{Status: &archived})
	require.NoError(t, err)
	require.True(t, story.ArchivedByAdmin)
	require.Equal(t, archived, story.Status)
	c, _ := fixture(t, s, now)
	tx, err = s.pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, applyGroups(ctx, tx, []Group{{StorylineID: id, Title: "පුවත්", EventIDs: []string{c}}}, now))
	require.NoError(t, rescore(ctx, tx, now, id))
	require.NoError(t, tx.Commit(ctx))
	story, err = s.Get(ctx, id, true)
	require.NoError(t, err)
	require.Equal(t, archived, story.Status)
	require.False(t, story.Trending)
	require.Len(t, story.Events, 2)
	_, existing, _, err := groupingInput(ctx, s.pool, now.Add(time.Minute), 48*time.Hour)
	require.NoError(t, err)
	require.Empty(t, existing, "admin archives are never offered to the model")
}

func TestStorylineEligibilityAndBatchedLoadIntegration(t *testing.T) {
	for _, gate := range []string{"rights", "expiry", "source", "article_category", "event_category"} {
		t.Run(gate, func(t *testing.T) {
			s := testStore(t)
			ctx := context.Background()
			now := time.Now()
			event, _ := fixture(t, s, now)
			_, _ = fixture(t, s, now) // Unassigned input must obey the same gate.
			id := uuid.NewString()
			_, err := s.pool.Exec(ctx, `INSERT INTO storylines(id,slug,title_si,pinned) VALUES($1,$2,'පුවත් පෙළ',true)`, id, "story-"+id)
			require.NoError(t, err)
			require.NoError(t, s.Membership(ctx, id, event, false))
			before := s.pool.Stat().AcquireCount()
			items, err := s.List(ctx, true)
			require.NoError(t, err)
			require.Equal(t, int64(2), s.pool.Stat().AcquireCount()-before, "list uses two queries for all storylines")
			require.Len(t, items, 1)
			data, err := json.Marshal(items[0].Public())
			require.NoError(t, err)
			for _, field := range []string{"pinned", "hidden", "title_locked", "summary_locked", "archived_by_admin", "metrics"} {
				require.NotContains(t, string(data), `"`+field+`"`)
			}
			query := map[string]string{
				"rights":           `UPDATE rights_profiles SET mode='disabled'`,
				"expiry":           `UPDATE rights_profiles SET expires_at=now()-interval '1 minute'`,
				"source":           `UPDATE sources SET active=false`,
				"article_category": `INSERT INTO categories(id,slug,name_si,name_en,held) VALUES('9ff91022-7fe1-40c4-99f1-417b1b202f22','held','රඳවා','Held',true); UPDATE articles SET category_id='9ff91022-7fe1-40c4-99f1-417b1b202f22'`,
				"event_category":   `INSERT INTO categories(id,slug,name_si,name_en,held) VALUES('9ff91022-7fe1-40c4-99f1-417b1b202f22','held','රඳවා','Held',true); UPDATE event_clusters SET category_id='9ff91022-7fe1-40c4-99f1-417b1b202f22'`,
			}[gate]
			_, err = s.pool.Exec(ctx, query)
			require.NoError(t, err)
			require.NoError(t, NewService(s, nil).Refresh(ctx, 48*time.Hour))
			_, err = s.Get(ctx, id, false)
			require.ErrorIs(t, err, ErrNotFound)
			items, err = s.List(ctx, false)
			require.NoError(t, err)
			require.Empty(t, items)
			admin, err := s.Get(ctx, id, true)
			require.NoError(t, err)
			require.Equal(t, 1, admin.EventCount)
			require.Zero(t, admin.Metrics.Articles24h)
			events, _, terms, err := groupingInput(ctx, s.pool, now.Add(time.Minute), 48*time.Hour)
			require.NoError(t, err)
			require.Empty(t, events)
			require.Empty(t, terms)
		})
	}
}

func TestExistingStorylinesSevenDayWindowIntegration(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now()
	id := uuid.NewString()
	_, err := s.pool.Exec(ctx, `INSERT INTO storylines(id,slug,title_si,last_activity_at) VALUES($1,$2,'පුවත් පෙළ',$3)`, id, "story-"+id, now.Add(-6*24*time.Hour))
	require.NoError(t, err)
	_, existing, _, err := groupingInput(ctx, s.pool, now, 48*time.Hour)
	require.NoError(t, err)
	require.Len(t, existing, 1)
	_, err = s.pool.Exec(ctx, `UPDATE storylines SET hidden=true WHERE id=$1`, id)
	require.NoError(t, err)
	_, existing, _, err = groupingInput(ctx, s.pool, now, 48*time.Hour)
	require.NoError(t, err)
	require.Empty(t, existing)
}

func TestStorylineControlsAndIndexRoundTripIntegration(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	var schema string
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema))
	// LIKE INCLUDING ALL copies the live event index; remove only the isolated copy.
	_, err := s.pool.Exec(ctx, `DROP INDEX IF EXISTS `+pgx.Identifier{schema, "articles_event_id_idx"}.Sanitize())
	require.NoError(t, err)
	for _, path := range []string{
		"000068_articles_event_index.up.sql", "000068_articles_event_index.down.sql",
		"000067_storyline_editorial_controls.down.sql", "000067_storyline_editorial_controls.up.sql",
		"000068_articles_event_index.up.sql",
	} {
		data, err := os.ReadFile("../../migrations/" + path)
		require.NoError(t, err)
		_, err = s.pool.Exec(ctx, string(data))
		require.NoError(t, err, path)
	}
	var valid bool
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT i.indisvalid FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname='articles_event_id_idx'`, schema).Scan(&valid))
	require.True(t, valid)
	_, err = s.pool.Exec(ctx, `SELECT summary_locked,archived_by_admin FROM storylines; SELECT event_id FROM storyline_event_exclusions`)
	require.NoError(t, err)
}
