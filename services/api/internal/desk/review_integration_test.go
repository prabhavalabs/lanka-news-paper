package desk

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
	"github.com/stretchr/testify/require"
)

func TestReviewArticleRecordsActorAndStateTransition(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	defer pool.Close()

	articleID := seedEditorialTestArticle(t, ctx, pool)
	actorID := uuid.New()

	category := "politics"
	err = NewStore(pool).ReviewArticle(ctx, articleID.String(), actorID, ArticleReview{
		Status: "published", Category: &category, Reason: "Reviewed in integration test",
	})
	require.NoError(t, err)

	var status, savedCategory, model string
	var confidence float64
	err = pool.QueryRow(ctx, `
		SELECT article.public_status, category.slug,
		       article.classify_confidence::double precision, article.classify_model
		FROM articles article
		JOIN categories category ON category.id = article.category_id
		WHERE article.id = $1
	`, articleID).Scan(&status, &savedCategory, &confidence, &model)
	require.NoError(t, err)
	require.Equal(t, "published", status)
	require.Equal(t, "politics", savedCategory)
	require.Equal(t, 1.0, confidence)
	require.Equal(t, "manual", model)

	var savedActor uuid.UUID
	var beforeValue, afterValue []byte
	var reason string
	err = pool.QueryRow(ctx, `
		SELECT actor_id, before_value, after_value, reason
		FROM editorial_actions
		WHERE entity_id = $1 AND action = 'review'
	`, articleID).Scan(&savedActor, &beforeValue, &afterValue, &reason)
	require.NoError(t, err)
	require.Equal(t, actorID, savedActor)
	require.Equal(t, "Reviewed in integration test", reason)

	var before, after articleEditorialState
	require.NoError(t, json.Unmarshal(beforeValue, &before))
	require.NoError(t, json.Unmarshal(afterValue, &after))
	require.Equal(t, "held", before.Status)
	require.Equal(t, "latest", *before.Category)
	require.Equal(t, "published", after.Status)
	require.Equal(t, "politics", *after.Category)
}

func TestRemoveArticleRecordsActorAndDeleteAction(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	defer pool.Close()

	articleID := seedEditorialTestArticle(t, ctx, pool)
	actorID := uuid.New()

	err = NewStore(pool).RemoveArticle(ctx, articleID.String(), actorID, "Deleted in integration test")
	require.NoError(t, err)

	var status string
	err = pool.QueryRow(ctx, `SELECT public_status FROM articles WHERE id = $1`, articleID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "removed", status)

	var savedActor uuid.UUID
	var beforeValue, afterValue []byte
	var reason string
	err = pool.QueryRow(ctx, `
		SELECT actor_id, before_value, after_value, reason
		FROM editorial_actions
		WHERE entity_id = $1 AND action = 'delete'
	`, articleID).Scan(&savedActor, &beforeValue, &afterValue, &reason)
	require.NoError(t, err)
	require.Equal(t, actorID, savedActor)
	require.Equal(t, "Deleted in integration test", reason)

	var before, after articleEditorialState
	require.NoError(t, json.Unmarshal(beforeValue, &before))
	require.NoError(t, json.Unmarshal(afterValue, &after))
	require.Equal(t, "held", before.Status)
	require.Equal(t, "removed", after.Status)

	params := pagination.Params{Page: 1, PerPage: 10, Search: articleID.String()}
	items, total, err := NewStore(pool).Articles(ctx, params, ArticleFilters{})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)

	items, total, err = NewStore(pool).Articles(ctx, params, ArticleFilters{Status: "removed"})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, 1, total)

	queueItems, queueTotal, err := NewStore(pool).Queue(ctx, params, "")
	require.NoError(t, err)
	require.Empty(t, queueItems)
	require.Zero(t, queueTotal)
}

func seedEditorialTestArticle(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	sourceID := uuid.New()
	endpointID := uuid.New()
	rightsID := uuid.New()
	articleID := uuid.New()

	_, err := pool.Exec(ctx, `
		INSERT INTO sources (id, name, legal_name, source_type, website, active)
		VALUES ($1, 'Editorial review test', 'Editorial review test', 'other', $2, false)
	`, sourceID, "https://review-test-"+sourceID.String()+".invalid")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO source_endpoints (id, source_id, endpoint_type, url, paused)
		VALUES ($1, $2, 'rss', $3, true)
	`, endpointID, sourceID, "https://review-test-"+endpointID.String()+".invalid/feed")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO rights_profiles (
			id, source_id, endpoint_id, mode, attribution, effective_from
		) VALUES ($1, $2, $3, 'discovery_only', 'Editorial review test', clock_timestamp())
	`, rightsID, sourceID, endpointID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO articles (
			id, source_id, endpoint_id, rights_profile_id, source_item_id,
			original_url, canonical_url, headline, fingerprint, published_at,
			public_status, category_id, classify_confidence, classify_model
		)
		SELECT $1::uuid, $2, $3, $4, $5, $6, $6, 'Editorial review test article ' || $1::uuid::text,
		       $7, clock_timestamp(), 'held', category.id, 0.3, 'rules-v2:fallback'
		FROM categories category WHERE category.slug = 'latest'
	`, articleID, sourceID, endpointID, rightsID, articleID.String(), "https://review-test.invalid/"+articleID.String(), articleID.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM editorial_actions WHERE entity_id = $1`, articleID)
		_, _ = pool.Exec(ctx, `DELETE FROM articles WHERE id = $1`, articleID)
		_, _ = pool.Exec(ctx, `DELETE FROM rights_profiles WHERE id = $1`, rightsID)
		_, _ = pool.Exec(ctx, `DELETE FROM source_endpoints WHERE id = $1`, endpointID)
		_, _ = pool.Exec(ctx, `DELETE FROM sources WHERE id = $1`, sourceID)
	})
	return articleID
}
