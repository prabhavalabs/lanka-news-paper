package newsletter

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

func TestNormalizeTestInputKeepsPreviewOffTheMailingList(t *testing.T) {
	t.Parallel()
	input, err := normalizeTestInput(TestInput{
		Mode: " preview ", WindowMode: "latest_24h", RecipientEmail: "reader@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, "preview", input.Mode)
	require.Empty(t, input.RecipientEmail)
}

func TestNewsletterPreviewRecordsPerformanceIntegration(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	actor := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO admin_users (id, email, display_name, role, status)
		VALUES ($1, $2, 'Newsletter test', 'administrator', 'active')
	`, actor, fmt.Sprintf("newsletter-test-%s@example.invalid", actor))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM newsletter_test_runs WHERE created_by = $1`, actor)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE actor_id = $1`, actor)
		_, _ = pool.Exec(ctx, `DELETE FROM admin_users WHERE id = $1`, actor)
	})
	service := NewService(NewStore(pool), nil, editorialCompleter{
		response: `{"intro":"පරීක්ෂණ හැඳින්වීම.","story_ids":[]}`,
		provider: "openrouter", model: "test/model",
	}, RuntimeConfig{BaseURL: "https://example.invalid"}, func() time.Time {
		return time.Date(2026, time.August, 28, 12, 0, 0, 0, time.UTC)
	})
	result, err := service.RunTest(ctx, TestInput{Mode: "preview", WindowMode: "latest_24h"}, actor)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.NotEmpty(t, result.HTML)
	require.Empty(t, result.RecipientEmail)
	items, err := service.ListTests(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, items)
	require.Equal(t, result.ID, items[0].ID)
}

func TestNormalizeTestInputRequiresValidAddressForSend(t *testing.T) {
	t.Parallel()
	_, err := normalizeTestInput(TestInput{Mode: "send", WindowMode: "scheduled", RecipientEmail: "not-an-email"})
	require.ErrorIs(t, err, ErrInvalidEmail)
}

func TestScheduledTestWindowUsesMostRecentConfiguredHour(t *testing.T) {
	t.Parallel()
	location, err := time.LoadLocation("Asia/Colombo")
	require.NoError(t, err)
	now := time.Date(2026, time.August, 28, 6, 30, 0, 0, location)
	date, start, end := testWindow(now, location, 8, "scheduled")
	require.Equal(t, "2026-08-27", date)
	require.Equal(t, time.Date(2026, time.August, 27, 8, 0, 0, 0, location), end)
	require.Equal(t, 24*time.Hour, end.Sub(start))
}
