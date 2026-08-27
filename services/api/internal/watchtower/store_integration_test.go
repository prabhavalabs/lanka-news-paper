package watchtower

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestStorePersistsListsAndDeletesAConversation(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	defer pool.Close()

	userID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO admin_users (id, email, display_name, role, status)
		VALUES ($1, $2, 'Watch Tower test', 'administrator', 'active')
	`, userID, fmt.Sprintf("watch-tower-%s@example.invalid", userID))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM admin_users WHERE id = $1`, userID) })

	store := NewStore(pool)
	thread, err := store.CreateThread(ctx, userID, "What happened today?")
	require.NoError(t, err)

	articleID := uuid.New()
	from := time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	conversation, err := store.SaveExchange(ctx, userID, thread.ID, "What happened today?", MessageDraft{
		Content: "A grounded answer [1].",
		Citations: []Citation{{
			Number: 1, ArticleID: articleID, Headline: "Test report", Source: "Test source",
			Category: "Local", PublishedAt: from, OriginalURL: "https://example.invalid/report",
		}},
		Suggestions: []string{"What happened next?"},
		Provider:    "openrouter", Model: "test-model",
		Search: SearchSummary{Label: "Last 24 hours", From: from, To: to, ArticleCount: 1},
	})
	require.NoError(t, err)
	require.Len(t, conversation.Messages, 2)
	require.Equal(t, articleID, conversation.Messages[1].Citations[0].ArticleID)
	require.Equal(t, "Last 24 hours", conversation.Messages[1].Search.Label)

	threads, err := store.ListThreads(ctx, userID)
	require.NoError(t, err)
	require.Len(t, threads, 1)
	require.Equal(t, 2, threads[0].MessageCount)

	require.NoError(t, store.DeleteThread(ctx, userID, thread.ID))
	_, err = store.Conversation(ctx, userID, thread.ID)
	require.ErrorIs(t, err, ErrNotFound)
}
