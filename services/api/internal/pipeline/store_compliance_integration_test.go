package pipeline

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestProcessSkipsRunWhenAIUseIsNotApprovedIntegration(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	sourceID, endpointID, rightsID, articleID, runID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
		INSERT INTO sources (id, name, legal_name, source_type, website, active)
		VALUES ($1, $2, $2, 'other', 'https://pipeline.example', false)
	`, sourceID, "Pipeline compliance "+sourceID.String())
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO source_endpoints (id, source_id, endpoint_type, url, paused)
		VALUES ($1, $2, 'rss', 'https://pipeline.example/feed.xml', true)
	`, endpointID, sourceID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO rights_profiles (id, source_id, endpoint_id, mode, attribution, effective_from)
		VALUES ($1, $2, $3, 'discovery_only', $4, clock_timestamp())
	`, rightsID, sourceID, endpointID, "Pipeline compliance "+sourceID.String())
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO source_compliance_reviews (
			source_id, version, active, status, allow_discovery,
			allow_ai_processing, notes, reviewed_by
		) VALUES ($1, 1, true, 'restricted', true, false, 'AI disabled for test', 'test')
	`, sourceID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO articles (
			id, source_id, endpoint_id, rights_profile_id, source_item_id,
			original_url, canonical_url, headline, fingerprint, published_at
		) VALUES (
			$1, $2, $3, $4, $5, 'https://pipeline.example/story/1',
			'https://pipeline.example/story/1', 'Compliance test article', $5,
			clock_timestamp()
		)
	`, articleID, sourceID, endpointID, rightsID, articleID.String())
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO article_pipeline_runs (id, article_id, trigger)
		VALUES ($1, $2, 'test')
	`, runID, articleID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO article_pipeline_steps (run_id, name, position)
		VALUES ($1, 'categorization', 1), ($1, 'event_clustering', 2), ($1, 'narration_analysis', 3)
	`, runID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM articles WHERE source_id = $1`, sourceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM source_compliance_reviews WHERE source_id = $1`, sourceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM rights_profiles WHERE source_id = $1`, sourceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM source_endpoints WHERE source_id = $1`, sourceID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM sources WHERE id = $1`, sourceID)
	})

	require.NoError(t, NewStore(pool, nil, nil, nil).Process(ctx, runID.String()))
	var runStatus string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT status FROM article_pipeline_runs WHERE id = $1
	`, runID).Scan(&runStatus))
	require.Equal(t, "succeeded", runStatus)
	var skipped, complianceLogs int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE step.status = 'skipped'),
		       count(*) FILTER (WHERE log.event = 'compliance_blocked')
		FROM article_pipeline_steps step
		LEFT JOIN article_pipeline_logs log ON log.step_id = step.id
		WHERE step.run_id = $1
	`, runID).Scan(&skipped, &complianceLogs))
	require.Equal(t, 3, skipped)
	require.Equal(t, 3, complianceLogs)
}
