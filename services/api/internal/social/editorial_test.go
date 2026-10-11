package social

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/stretchr/testify/require"
)

func TestCaptionCreditsAndHashtags(t *testing.T) {
	caption := BuildCaption("පුවත මෙන්න.", []string{"A", "B"}, "https://news.invalid/e/one", []string{"#News", "පුවත්", "bad tag", "News", "Extra"})
	require.Equal(t, "පුවත මෙන්න.\n\nමූලාශ්‍ර: A, B\n\nhttps://news.invalid/e/one\n\n#News #පුවත් #Extra", caption)
	require.Equal(t, "පුවත මෙන්න.\n\nමූලාශ්‍ර: A, B\n\n#News", BuildCaption("පුවත මෙන්න.", []string{"A", "B"}, "", []string{"News"}))
}

func TestEditorialValidation(t *testing.T) {
	item, valid := validateEditorial(editorialItem{ID: "one", Score: 120, Publish: true, Reason: "public interest", HeadlineSI: strings.Repeat("ප", 100), SummarySI: strings.Repeat("ප", 300), CaptionSI: strings.Repeat("ප", 600), Hashtags: []string{"#පුවත්"}})
	require.True(t, valid)
	require.Equal(t, 100, item.Score)
	require.Equal(t, 90, utf8.RuneCountInString(item.HeadlineSI))
	require.Equal(t, 280, utf8.RuneCountInString(item.SummarySI))
	require.Equal(t, 500, utf8.RuneCountInString(item.CaptionSI))
	_, valid = validateEditorial(editorialItem{ID: "one", Publish: true, Reason: "fine", HeadlineSI: "English", SummarySI: "English", CaptionSI: "English"})
	require.False(t, valid)
	_, valid = validateEditorial(editorialItem{ID: "one", Publish: false, Reason: "Not enough coverage"})
	require.True(t, valid)
	_, valid = validateEditorial(editorialItem{Publish: false, Reason: "missing ID"})
	require.False(t, valid)
}

func TestRankingAndSkippedReevaluation(t *testing.T) {
	now := time.Now()
	items := []Candidate{
		{ID: "recent", LatestAt: now, SourceCount: 2, ArticleCount: 2},
		{ID: "many", LatestAt: now, SourceCount: 4, ArticleCount: 4},
		{ID: "breaking", Breaking: true, SourceCount: 1, LatestAt: now.Add(-3 * time.Hour)},
		{ID: "more-members", LatestAt: now, SourceCount: 4, ArticleCount: 6},
		{ID: "politics", CategorySlug: "politics", LatestAt: now, SourceCount: 2, ArticleCount: 2},
		{ID: "stale", LatestAt: now.Add(-10 * time.Hour), SourceCount: 4, ArticleCount: 6},
	}
	rankCandidates(items, now)
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	require.Equal(t, []string{"breaking", "politics", "more-members", "many", "recent", "stale"}, ids)
	require.False(t, reevaluateSkipped(Candidate{SourceCount: 3}, 2, false))
	require.True(t, reevaluateSkipped(Candidate{SourceCount: 4}, 2, false))
	require.True(t, reevaluateSkipped(Candidate{Breaking: true, SourceCount: 2}, 2, false))
	require.False(t, reevaluateSkipped(Candidate{Breaking: true, SourceCount: 2}, 2, true))
}

type editorialModel struct {
	response    llm.Response
	err         error
	request     llm.Request
	calls       int
	onComplete  func()
	review      llm.Response // social_post_review answer; empty means a clean pass
	reviewErr   error
	reviewCalls int
	reviews     []llm.Response // successive review answers, used before review
	revision    llm.Response   // answer to the rewrite request after a failed review
	revisions   int
}

func (model *editorialModel) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	if request.System == revisionPrompt {
		model.revisions++
		return model.revision, nil
	}
	if request.Task == "social_post_review" {
		model.reviewCalls++
		if len(model.reviews) > 0 {
			next := model.reviews[0]
			model.reviews = model.reviews[1:]
			return next, nil
		}
		if model.review.Provider == "" && model.reviewErr == nil {
			return llm.Response{Provider: "codex_cli", Text: `{"pass":true,"score":95,"summary":"Meets the standards.","issues":[]}`}, nil
		}
		return model.review, model.reviewErr
	}
	model.request = request
	model.calls++
	if model.onComplete != nil {
		model.onComplete()
	}
	return model.response, model.err
}
func (model *editorialModel) CompleteWithModel(ctx context.Context, request llm.Request, _, _ string) (llm.Response, error) {
	return model.Complete(ctx, request)
}

func TestEditorialDropsUnknownAndDuplicateItems(t *testing.T) {
	model := &editorialModel{response: llm.Response{Provider: "codex_cli", CallID: 42, Text: `{"items":[{"id":"one","score":30,"reason":"not urgent","publish":false},{"id":"one","score":80,"reason":"duplicate","publish":false},{"id":"unknown","reason":"unknown","publish":false}]}`}}
	service := &Service{model: model}
	items, callID, err := service.editorial(context.Background(), []Candidate{{ID: "one"}}, nil, pageActivity{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(42), *callID)
	require.Equal(t, "social_post_editorial", model.request.Task)
	model.response.Provider = "none"
	_, _, err = service.editorial(context.Background(), []Candidate{{ID: "one"}}, nil, pageActivity{})
	require.ErrorIs(t, err, ErrModelUnavailable)
}

func TestEditorialRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	for _, text := range []string{`{"items":[],"unexpected":true}`, `{"items":[{"id":"one","reason":"skip","unexpected":true}]}`, `{"items":[]} {}`} {
		service := &Service{model: &editorialModel{response: llm.Response{Provider: "codex_cli", Text: text}}}
		_, _, err := service.editorial(context.Background(), []Candidate{{ID: "one"}}, nil, pageActivity{})
		require.ErrorIs(t, err, ErrModelUnavailable)
	}
}
