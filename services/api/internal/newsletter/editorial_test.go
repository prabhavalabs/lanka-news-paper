package newsletter

import (
	"context"
	"testing"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/stretchr/testify/require"
)

type editorialCompleter struct {
	response string
}

func (completer editorialCompleter) Complete(context.Context, llm.Request) (llm.Response, error) {
	return llm.Response{Text: completer.response}, nil
}

func (completer editorialCompleter) CompleteWithModel(context.Context, llm.Request, string, string) (llm.Response, error) {
	return llm.Response{Text: completer.response}, nil
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
