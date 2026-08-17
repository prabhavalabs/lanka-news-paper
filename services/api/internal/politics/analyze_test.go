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

func TestEconomicPolicySignalGate(t *testing.T) {
	for _, value := range []string{
		"The budget expands welfare and raises tax revenue",
		"පෞද්ගලීකරණය ගැන ආර්ථික විවාදයක්",
		"பணவீக்கம் மற்றும் வட்டி விகிதம் குறித்து விவாதம்",
	} {
		require.Truef(t, hasEconomicPolicySignal(value), "expected policy signal in %q", value)
	}
	for _, value := range []string{
		"Police arrest four private-bank managers in financial-crime case",
		"මත්ද්‍රව්‍ය වැටලීම්වලදී සැකකරුවන් 891 දෙනෙකු අත්අඩංගුවට",
		"බන්ධනාගාර නිලධාරීන්ට ගිනි අවි පුහුණුවක්දී නැහැ",
	} {
		require.Falsef(t, hasEconomicPolicySignal(value), "unexpected policy signal in %q", value)
	}
}
