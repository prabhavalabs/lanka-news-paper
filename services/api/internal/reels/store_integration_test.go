package reels

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestStoreDraftLifecycle(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	defer pool.Close()
	var storylineID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO storylines (slug, title_si) VALUES ($1, 'පරීක්ෂණ කතාව') RETURNING id::text`,
		"reel-test-"+time.Now().Format("150405.000000")[7:]).Scan(&storylineID))
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM social_reels WHERE storyline_id = $1`, storylineID)
		pool.Exec(context.Background(), `DELETE FROM storylines WHERE id = $1`, storylineID)
	})
	store := NewStore(pool)

	id, err := store.Create(ctx, "schedule", storylineID, nil, Trend{Score: 90, GoogleTrends: []string{"namal"}, Reason: "searched"})
	require.NoError(t, err)
	require.NoError(t, store.Start(ctx, id))
	require.ErrorIs(t, store.Start(ctx, id), ErrConflict)

	_, _, err = store.Edit(ctx, id, Edit{})
	require.ErrorIs(t, err, ErrConflict, "a draft being built cannot be edited")

	script := Script{Tag: "අධිකරණ පුවත", Headline: []string{"එයාර්බස් නඩුව"}, Facts: []string{"Fact one"}, Question: "ඔබ සෑහීමකට පත්වෙනවාද?", YesLabel: "ඔව්", NoLabel: "නැහැ"}
	music := Music{File: "epic.mp3", Title: "Epic", Start: 28.2}
	require.NoError(t, store.SaveDraft(ctx, id, "", "Story", script, "Caption", Assets{Photos: []Photo{{Key: "reels/x/ai-1.png", Kind: "ai"}}, Music: &music}))
	_, err = store.Review(ctx, id, "approve", nil)
	require.ErrorIs(t, err, ErrConflict, "nothing to approve before the video exists")
	require.NoError(t, store.SaveRendered(ctx, id, "reels/x/reel.mp4", "reels/x/cover.jpg"))

	reel, err := store.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, StatusReady, reel.Status)
	require.True(t, reel.HasVideo)
	require.Equal(t, "පරීක්ෂණ කතාව", reel.StorylineTitle)
	require.Equal(t, []string{"namal"}, reel.Trend.GoogleTrends)

	caption := "New caption"
	reel, rerender, err := store.Edit(ctx, id, Edit{Caption: &caption})
	require.NoError(t, err)
	require.False(t, rerender)
	require.Equal(t, StatusReady, reel.Status)
	require.Equal(t, caption, reel.Caption)

	question := "මේ නඩුවෙන් සාධාරණ තීන්දුවක් ලැබේවිද?"
	reel, rerender, err = store.Edit(ctx, id, Edit{Question: &question})
	require.NoError(t, err)
	require.True(t, rerender)
	require.Equal(t, StatusQueued, reel.Status)
	require.Equal(t, question, reel.Script.Question)
	tooLong := []string{"a", "b", "c", "d"}
	_, _, err = store.Edit(ctx, id, Edit{Headline: &tooLong})
	require.True(t, errors.Is(err, ErrConflict) || errors.Is(err, ErrInvalid))

	require.NoError(t, store.Start(ctx, id))
	require.NoError(t, store.SaveRendered(ctx, id, "reels/x/reel-2.mp4", "reels/x/cover-2.jpg"))
	reel, err = store.Review(ctx, id, "approve", nil)
	require.NoError(t, err)
	require.Equal(t, StatusApproved, reel.Status)
	reel, err = store.Review(ctx, id, "posted", nil)
	require.NoError(t, err)
	require.Equal(t, StatusPosted, reel.Status)
	_, err = store.Review(ctx, id, "approve", nil)
	require.ErrorIs(t, err, ErrConflict)
	_, err = store.Review(ctx, id, "publish", nil)
	require.ErrorIs(t, err, ErrInvalid)

	recent, err := store.recentStories(ctx, time.Hour)
	require.NoError(t, err)
	require.Contains(t, recent, storylineID)
	used, err := store.lastMusic(ctx, 3)
	require.NoError(t, err)
	require.Contains(t, used, "epic.mp3")

	failed, err := store.Create(ctx, "manual", "", nil, Trend{})
	require.NoError(t, err)
	require.NoError(t, store.Start(ctx, failed))
	require.NoError(t, store.Fail(ctx, failed, errors.New("boom")))
	reel, err = store.Get(ctx, failed)
	require.NoError(t, err)
	require.Equal(t, StatusFailed, reel.Status)
	require.Equal(t, "boom", *reel.Error)
	require.NoError(t, store.Delete(ctx, failed))
	_, err = store.Get(ctx, failed)
	require.ErrorIs(t, err, ErrNotFound)
}
