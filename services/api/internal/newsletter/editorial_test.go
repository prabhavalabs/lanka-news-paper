package newsletter

import (
	"context"
	"testing"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/stretchr/testify/require"
)

type editorialCompleter struct {
	response string
	provider string
	model    string
}

func (completer editorialCompleter) Complete(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{Text: completer.response, Provider: completer.provider, Model: completer.model}, nil
}

func (completer editorialCompleter) CompleteWithModel(context.Context, llm.Request, string, string) (llm.Response, error) {
	return llm.Response{Text: completer.response, Provider: completer.provider, Model: completer.model}, nil
}

func TestApplyEditorialPlanOnlyReordersKnownStories(t *testing.T) {
	t.Parallel()
	digest := Digest{Stories: []Story{{ID: "one"}, {ID: "two"}, {ID: "three"}}}
	settings := defaultSettings()

	result, updated := applyEditorialPlan(context.Background(), editorialCompleter{
		response: `{"intro":"සුබ උදෑසනක්.","story_ids":["three","unknown","one"]}`,
	}, digest, settings)

	require.Equal(t, []string{"three", "one", "two"}, []string{result.Stories[0].ID, result.Stories[1].ID, result.Stories[2].ID})
	require.Equal(t, "සුබ උදෑසනක්.", updated.IntroText)
}

func TestEditorialUnavailableProviderUsesCallerPolicy(t *testing.T) {
	digest := Digest{Stories: []Story{{ID: "one"}}}
	settings := defaultSettings()
	model := editorialCompleter{provider: "none", response: `{"intro":"untrusted fallback","story_ids":[]}`}
	_, _, _, err := applyEditorialPlanStrict(context.Background(), model, digest, settings)
	require.ErrorContains(t, err, "no model provider responded")
	result, updated := applyEditorialPlan(context.Background(), model, digest, settings)
	require.Equal(t, digest, result)
	require.Equal(t, settings, updated)
}
