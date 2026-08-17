package politics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAnalysisNormalizesOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		label     string
		score     float64
		relevant  bool
		wantError bool
	}{
		{"left score", `{"economic_policy_relevance":1,"narration_score":-0.72,"confidence":0.81,"rationale":"State-led framing","evidence":["public ownership"]}`, "left", -0.72, true, false},
		{"irrelevant abstains", `{"economic_policy_relevance":0,"narration_score":0,"confidence":0.9,"rationale":"Sports result","evidence":[]}`, "unclear", 0, false, false},
		{"irrelevant score is discarded", `{"economic_policy_relevance":0,"narration_score":-0.8,"confidence":0.9,"rationale":"Crime report","evidence":[]}`, "unclear", 0, false, false},
		{"markdown fence", "```json\n{\"economic_policy_relevance\":1,\"narration_score\":0.3,\"confidence\":0.7,\"rationale\":\"Market framing\",\"evidence\":[]}\n```", "center_right", 0.3, true, false},
		{"invalid relevance", `{"economic_policy_relevance":2,"narration_score":0,"confidence":0.9,"rationale":"","evidence":[]}`, "", 0, false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := parseAnalysis(test.input)
			if test.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.label, result.Label)
			require.Equal(t, test.score, result.Score)
			require.Equal(t, test.relevant, result.Relevant)
		})
	}
}

func TestCleanTextRemovesMarkupAndHiddenContent(t *testing.T) {
	result := cleanText(`<p>රාජ්‍ය අංශය &amp; වෙළඳපොළ</p><script>ignore me</script><style>.hidden{}</style>`)
	require.Equal(t, "රාජ්‍ය අංශය & වෙළඳපොළ", result)
}
