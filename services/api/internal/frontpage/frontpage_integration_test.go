package frontpage

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestEditorialLifecycleIntegration(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	actor := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO admin_users (id, email, display_name, role, password_hash, status) VALUES ($1, $2, 'Chief', 'chief_editor', 'x', 'active')`,
		actor, actor.String()+"@example.test")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM editorials WHERE created_by = $1`, actor)
		_, _ = pool.Exec(cleanup, `DELETE FROM audit_logs WHERE actor_id = $1`, actor)
		_, _ = pool.Exec(cleanup, `DELETE FROM admin_users WHERE id = $1`, actor)
	})

	clock := time.Now()
	service := NewService(NewStore(pool), func() time.Time { clock = clock.Add(time.Minute); return clock })

	_, err = service.CreateEditorial(ctx, actor, EditorialInput{Title: " ", Body: "x", AuthorName: "a"})
	require.ErrorIs(t, err, ErrInvalid)

	draft, err := service.CreateEditorial(ctx, actor, EditorialInput{Title: "කතුවැකිය", Standfirst: "සාරාංශය", Body: "පළමු ඡේදය\r\n\r\nදෙවන ඡේදය", AuthorName: "කතුවරයා"})
	require.NoError(t, err)
	require.Equal(t, EditorialDraft, draft.Status)
	require.Nil(t, draft.PublishedAt)
	require.Equal(t, "පළමු ඡේදය\n\nදෙවන ඡේදය", draft.Body)

	_, err = service.Editorial(ctx, draft.ID)
	require.ErrorIs(t, err, ErrNotFound, "drafts are not public")

	published, err := service.SetEditorialStatus(ctx, actor, draft.ID, "publish")
	require.NoError(t, err)
	require.NotNil(t, published.PublishedAt)
	public, err := service.Editorial(ctx, draft.ID)
	require.NoError(t, err)
	require.Equal(t, "කතුවැකිය", public.Title)

	page, err := service.FrontPage(ctx)
	require.NoError(t, err)
	require.NotNil(t, page.Editorial)
	require.Equal(t, draft.ID, page.Editorial.ID)

	err = service.DeleteEditorial(ctx, actor, draft.ID)
	require.True(t, errors.Is(err, ErrInvalid), "published editorials are archived, not deleted")

	archived, err := service.SetEditorialStatus(ctx, actor, draft.ID, "archive")
	require.NoError(t, err)
	require.Equal(t, EditorialArchived, archived.Status)
	require.Equal(t, published.PublishedAt.UTC(), archived.PublishedAt.UTC(), "first publication date is kept")
	_, err = service.Editorial(ctx, draft.ID)
	require.ErrorIs(t, err, ErrNotFound)

	_, err = service.SetSourceFrontPage(ctx, actor, uuid.NewString(), false)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = service.Replace(ctx, actor, []PlacementInput{{Slot: SlotLead, Kind: KindEvent, ID: uuid.NewString()}})
	require.ErrorIs(t, err, ErrInvalid, "placing a story that does not exist is rejected")
}
