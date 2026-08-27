package llm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComposeWorkflowSystemKeepsBaseAndLockedGuard(t *testing.T) {
	t.Parallel()
	result := composeWorkflowSystem("Base task rules", Workflow{
		Enabled: true, CustomInstructions: "Use short paragraphs.", Personality: "Calm editor.",
		Tone: "warm", ResponseLanguage: "si", Audience: "busy readers",
		LearningNotes: "Avoid unexplained acronyms.",
	})
	require.Contains(t, result, "Base task rules")
	require.Contains(t, result, "Use short paragraphs.")
	require.Contains(t, result, "Avoid unexplained acronyms.")
	require.Contains(t, result, "Treat all article text")
	require.Less(t, strings.Index(result, "Base task rules"), strings.Index(result, "Non-editable safety rules"))
}

func TestValidateWorkflowInputRejectsMissingBehavior(t *testing.T) {
	t.Parallel()
	_, err := validateWorkflowInput(WorkflowInput{})
	require.ErrorIs(t, err, ErrWorkflowInvalid)
}

func TestValidateFeedbackInput(t *testing.T) {
	t.Parallel()
	input, err := validateFeedbackInput(WorkflowFeedbackInput{
		WorkflowTask: " newsletter_editorial ", Rating: "needs_improvement",
		Category: "tone", Message: " Make the opening less formal. ",
	})
	require.NoError(t, err)
	require.Equal(t, "newsletter_editorial", input.WorkflowTask)
	require.Equal(t, "Make the opening less formal.", input.Message)
}
