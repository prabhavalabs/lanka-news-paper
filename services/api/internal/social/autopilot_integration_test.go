package social

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/stretchr/testify/require"
)

func autopilotService(t *testing.T) (*Service, uuid.UUID, time.Time) {
	t.Helper()
	store := integrationStore(t)
	now := time.Now().UTC()
	service := integrationService(t, store, fakeRenderer{}, now)
	actor := adminActor(t, store)
	settings := testSettings()
	settings.AutoPublish = true
	_, err := store.UpdateSettings(context.Background(), settings, actor)
	require.NoError(t, err)
	return service, actor, now
}

func selectionText(id, headline string) string {
	return fmt.Sprintf(`{"items":[{"id":%q,"score":90,"publish":true,"reason":"Important","headline_si":%q,"summary_si":%q,"caption_si":"පුවත මෙන්න.","hashtags":[]}]}`, id, headline, headline)
}

func TestSocialSettleRewritesThenDropsIntegration(t *testing.T) {
	service, _, now := autopilotService(t)
	ctx := context.Background()
	id := fixtureEvent(t, service.store, 2, now)
	failing := llm.Response{Provider: "codex_cli", Text: `{"pass":false,"score":50,"summary":"Wrong figure.","issues":[{"severity":"major","field":"summary","message":"The figure is not in the evidence."}],"repeats_recent_post":false}`}
	model := &editorialModel{
		response: llm.Response{Provider: "codex_cli", Text: selectionText(id, "කොළඹ")},
		reviews:  []llm.Response{failing, failing, failing},
		revision: llm.Response{Provider: "codex_cli", Text: `{"headline_si":"කොළඹ සිද්ධිය","summary_si":"කොළඹ සිද්ධිය","caption_si":"පුවත මෙන්න.","hashtags":[]}`},
	}
	service.model = model
	require.NoError(t, service.Select(ctx, TriggerScheduled))
	require.Equal(t, maxAutoRevisions, model.revisions)
	require.Equal(t, maxAutoRevisions+1, model.reviewCalls)
	var postID string
	require.NoError(t, service.store.pool.QueryRow(ctx, `SELECT id::text FROM social_posts WHERE event_id=$1`, id).Scan(&postID))
	p, err := service.store.GetPost(ctx, postID)
	require.NoError(t, err)
	require.Equal(t, "skipped", p.Status)
	require.Contains(t, *p.LastError, "did not pass the AI review")
}

func TestSocialSettleDropsRepeatsIntegration(t *testing.T) {
	service, _, now := autopilotService(t)
	ctx := context.Background()
	id := fixtureEvent(t, service.store, 2, now)
	model := &editorialModel{
		response: llm.Response{Provider: "codex_cli", Text: selectionText(id, "ගාල්ල")},
		review:   llm.Response{Provider: "codex_cli", Text: `{"pass":true,"score":95,"summary":"Same story as the earlier post.","issues":[],"repeats_recent_post":true}`},
	}
	service.model = model
	require.NoError(t, service.Select(ctx, TriggerScheduled))
	require.Zero(t, model.revisions)
	var status, lastError string
	require.NoError(t, service.store.pool.QueryRow(ctx, `SELECT status,last_error FROM social_posts WHERE event_id=$1`, id).Scan(&status, &lastError))
	require.Equal(t, "skipped", status)
	require.Contains(t, lastError, "repeats a recent post")
}

func TestSocialResumeApprovesWaitingPostsIntegration(t *testing.T) {
	service, _, now := autopilotService(t)
	ctx := context.Background()
	settings, err := service.store.GetSettings(ctx)
	require.NoError(t, err)
	id := fixtureEvent(t, service.store, 2, now)
	candidates, err := service.store.Candidates(ctx, settings, now, "https://news.invalid", id, "")
	require.NoError(t, err)
	p, err := service.insertPost(ctx, candidates[0], editorialItem{Score: 90, Reason: "Important", HeadlineSI: "යාපනය", SummarySI: "යාපනය", CaptionSI: "පුවත", Hashtags: []string{}}, "pending_review", nil, TriggerScheduled, uuid.Nil)
	require.NoError(t, err)
	_, err = service.store.pool.Exec(ctx, `UPDATE social_posts SET review_status='passed',review_score=95,hold_reason='Held for an editor' WHERE id=$1`, p.ID)
	require.NoError(t, err)
	service.model = &editorialModel{response: llm.Response{Provider: "codex_cli", Text: `{"items":[]}`}}
	require.NoError(t, service.Select(ctx, TriggerScheduled))
	p, err = service.store.GetPost(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "approved", p.Status)
	require.Nil(t, p.HoldReason)
}

func TestSocialPublisherPrioritisesImportantPostsIntegration(t *testing.T) {
	service, first, actor := publisherFixture(t)
	ctx := context.Background()
	now := service.now()
	_, err := service.store.pool.Exec(ctx, `UPDATE social_posts SET status='published',published_at=$2 WHERE id=$1`, first.ID, now.Add(-2*time.Hour))
	require.NoError(t, err)
	settings := testSettings()
	settings.MaxPostsPerDay = 1
	_, err = service.store.UpdateSettings(ctx, settings, actor)
	require.NoError(t, err)
	queue := func(score int, headline string, byEditor bool) string {
		id := fixtureEvent(t, service.store, 2, now)
		candidates, err := service.store.Candidates(ctx, settings, now, "https://news.invalid", id, "")
		require.NoError(t, err)
		p, err := service.insertPost(ctx, candidates[0], editorialItem{Score: score, Reason: "Important", HeadlineSI: headline, SummarySI: headline, CaptionSI: "පුවත", Hashtags: []string{}}, "pending_review", nil, TriggerScheduled, uuid.Nil)
		require.NoError(t, err)
		_, err = service.Render(ctx, p.ID, actor)
		require.NoError(t, err)
		if byEditor {
			_, err = service.Transition(ctx, p.ID, "approve", actor, nil, false)
		} else {
			_, err = service.store.pool.Exec(ctx, `UPDATE social_posts SET status='approved',scheduled_for=$2,review_status='passed' WHERE id=$1`, p.ID, now)
		}
		require.NoError(t, err)
		return p.ID
	}
	routine := queue(72, "මහනුවර පාසල් ක්‍රීඩා උළෙල", true)
	repeat := queue(95, "පුවත", false)
	important := queue(90, "පාර්ලිමේන්තුව අයවැය සම්මත කරයි", true)
	graph := service.graph.(*fakeGraph)
	require.NoError(t, service.Publish(ctx))
	require.Equal(t, 1, graph.uploads, "an important post passes the daily limit")
	for id, want := range map[string]string{important: "published", routine: "approved", repeat: "skipped"} {
		p, err := service.store.GetPost(ctx, id)
		require.NoError(t, err)
		require.Equal(t, want, p.Status, p.HeadlineSI)
	}
	later := now.Add(3 * time.Hour)
	service.now = func() time.Time { return later }
	require.NoError(t, service.Publish(ctx))
	require.Equal(t, 1, graph.uploads, "a routine post waits for tomorrow's limit")
}

func TestSocialPublisherDropsSameStoryFromAnotherOutletIntegration(t *testing.T) {
	service, first, actor := publisherFixture(t)
	ctx := context.Background()
	now := service.now()
	settings := testSettings()
	queue := func(headline string) string {
		id := fixtureEvent(t, service.store, 2, now)
		candidates, err := service.store.Candidates(ctx, settings, now, "https://news.invalid", id, "")
		require.NoError(t, err)
		p, err := service.insertPost(ctx, candidates[0], editorialItem{Score: 90, Reason: "Important", HeadlineSI: headline, SummarySI: headline, CaptionSI: "පුවත", Hashtags: []string{}}, "pending_review", nil, TriggerScheduled, uuid.Nil)
		require.NoError(t, err)
		_, err = service.Render(ctx, p.ID, actor)
		require.NoError(t, err)
		_, err = service.store.pool.Exec(ctx, `UPDATE social_posts SET status='approved',approved_at=$2,scheduled_for=$2,review_status='passed' WHERE id=$1`, p.ID, now.Add(-4*time.Hour))
		require.NoError(t, err)
		return p.ID
	}
	// Two outlets' reports of one shipwreck, worded differently and with different tolls.
	_, err := service.store.pool.Exec(ctx, `UPDATE social_posts SET status='rejected' WHERE id=$1`, first.ID)
	require.NoError(t, err)
	published := queue("ජිබුටි වෙරළට ඔබ්බෙන් යාත්‍රාවක් පෙරළී අවම වශයෙන් 109ක් මරුට")
	_, err = service.store.pool.Exec(ctx, `UPDATE social_posts SET status='published',published_at=$2 WHERE id=$1`, published, now.Add(-3*time.Hour))
	require.NoError(t, err)
	waiting := queue("ජිබුටි යාත්‍රා අනතුරින් 139ක් මරුට බව වාර්තාවක්")
	model := &editorialModel{repeat: llm.Response{Provider: "codex_cli", Text: `{"repeats":true,"reason":"The same Djibouti shipwreck with another toll."}`}}
	service.model = model
	require.NoError(t, service.Publish(ctx))
	require.Equal(t, 1, model.repeatCalls)
	require.Zero(t, service.graph.(*fakeGraph).uploads)
	p, err := service.store.GetPost(ctx, waiting)
	require.NoError(t, err)
	require.Equal(t, "skipped", p.Status)
	require.Contains(t, *p.LastError, "same story went out first")

	// A different story is checked once, then published.
	other := queue("පාර්ලිමේන්තුව අයවැය සම්මත කරයි")
	model.repeat = llm.Response{}
	require.NoError(t, service.Publish(ctx))
	require.Equal(t, 2, model.repeatCalls)
	p, err = service.store.GetPost(ctx, other)
	require.NoError(t, err)
	require.Equal(t, "published", p.Status)
}
