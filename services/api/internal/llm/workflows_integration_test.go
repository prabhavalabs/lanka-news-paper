package llm

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestWorkflowUpdateAndReviewedFeedbackAreVersionedIntegration(t *testing.T) {
	databaseURL := os.Getenv("SNAP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SNAP_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	userID := uuid.New()
	task := "integration_" + uuid.NewString()
	_, err = pool.Exec(ctx, `
		INSERT INTO admin_users (id, email, display_name, role, status)
		VALUES ($1, $2, 'Workflow test', 'administrator', 'active')
	`, userID, fmt.Sprintf("workflow-%s@example.invalid", userID))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO agent_workflows (task, name, purpose, category)
		VALUES ($1, 'Integration workflow', 'Validates versioned settings.', 'Test')
	`, task)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM agent_workflows WHERE task = $1`, task)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE actor_id = $1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM admin_users WHERE id = $1`, userID)
	})

	gateway := NewGateway(pool)
	workflow, err := gateway.UpdateWorkflow(ctx, task, WorkflowInput{
		CustomInstructions: "Prefer the public-interest angle.", Personality: "Calm editor.",
		Tone: "warm", ResponseLanguage: "si", Audience: "readers", Enabled: true,
	}, userID)
	require.NoError(t, err)
	require.Equal(t, 2, workflow.Revision)

	feedback, err := gateway.CreateWorkflowFeedback(ctx, WorkflowFeedbackInput{
		WorkflowTask: task, Rating: "needs_improvement", Category: "tone",
		Message: "Use a less formal introduction.",
	}, userID)
	require.NoError(t, err)
	feedback, err = gateway.ReviewWorkflowFeedback(ctx, feedback.ID, "applied", userID)
	require.NoError(t, err)
	require.Equal(t, "applied", feedback.Status)

	var revision, versions int
	var learningNotes string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT revision, learning_notes,
		       (SELECT count(*)::integer FROM agent_workflow_versions version WHERE version.task = workflow.task)
		FROM agent_workflows workflow WHERE task = $1
	`, task).Scan(&revision, &learningNotes, &versions))
	require.Equal(t, 3, revision)
	require.Contains(t, learningNotes, "less formal introduction")
	require.Equal(t, 2, versions)
}
