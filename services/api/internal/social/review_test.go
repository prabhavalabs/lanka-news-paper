package social

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApprovableRequiresCleanPassAboveThreshold(t *testing.T) {
	pass := reviewResult{Pass: true, Score: 90, Issues: []ReviewIssue{{Severity: "minor", Field: "caption", Message: "Polish"}}}
	require.True(t, approvable(pass, 80))
	require.False(t, approvable(pass, 91), "below the configured score")
	failed := pass
	failed.Pass = false
	require.False(t, approvable(failed, 0), "reviewer did not pass it")
	major := pass
	major.Issues = []ReviewIssue{{Severity: "major", Field: "headline", Message: "Unattributed claim"}}
	require.False(t, approvable(major, 0), "major issues always need an editor")
	blocker := pass
	blocker.Issues = []ReviewIssue{{Severity: "blocker", Field: "story", Message: "Defamatory"}}
	require.False(t, approvable(blocker, 0))
}

func TestValidateReviewClampsAndCleans(t *testing.T) {
	issues := []ReviewIssue{{Severity: "odd", Field: "headline", Message: "Check name"}, {Severity: "minor", Field: "caption", Message: ""}}
	for range 20 {
		issues = append(issues, ReviewIssue{Severity: "minor", Field: "caption", Message: "x"})
	}
	result := validateReview(reviewResult{Pass: true, Score: 140, Summary: strings.Repeat("a", 2000), Issues: issues})
	require.Equal(t, 100, result.Score)
	require.LessOrEqual(t, len([]rune(result.Summary)), 1000)
	require.Len(t, result.Issues, 12)
	require.Equal(t, "major", result.Issues[0].Severity, "unknown severities are treated as major")
	require.False(t, approvable(result, 0))
}

func TestCaptionBodyReadsBothCreditLabels(t *testing.T) {
	require.Equal(t, "පුවත.", captionBody(BuildCaption("පුවත.", []string{"Hiru News"}, "", nil)))
	require.Equal(t, "පුවත.", captionBody("පුවත.\n\nමූලාශ්‍ර: Hiru News"))
	require.NotContains(t, sourceCreditLabel, "\u200d")
}
