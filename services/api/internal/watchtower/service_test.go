package watchtower

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/stretchr/testify/require"
)

func TestParseSearchScopeUnderstandsLastTwentyFourHours(t *testing.T) {
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.UTC)

	scope := ParseSearchScope("Tell me what happened in Sri Lanka in the last 24 hours", now)

	require.Equal(t, now.Add(-24*time.Hour), scope.From)
	require.Equal(t, now, scope.To)
	require.Equal(t, "Last 24 hours", scope.Label)
	require.Empty(t, scope.Category)
	require.True(t, scope.ExcludeWorld)
}

func TestParseSearchScopeInfersTopicAndDefaultWindow(t *testing.T) {
	now := time.Date(2026, time.August, 27, 12, 30, 0, 0, time.UTC)

	scope := ParseSearchScope("What are the important economic stories?", now)

	require.Equal(t, now.Add(-7*24*time.Hour), scope.From)
	require.Equal(t, now, scope.To)
	require.Equal(t, "economy", scope.Category)
	require.True(t, scope.CategoryOnly)
	require.Contains(t, scope.Terms, "economic")
}

func TestServiceAskPersistsAGroundedAnswer(t *testing.T) {
	threadID := uuid.New()
	userID := uuid.New()
	articleID := uuid.New()
	repository := &fakeRepository{
		settings: Settings{ResponseLanguage: LanguageSinhala},
		conversation: Conversation{
			Thread:   Thread{ID: threadID, UserID: userID, Title: "Sri Lanka today"},
			Messages: []Message{{Role: RoleUser, Content: "Focus on the economy."}},
		},
		articles: []ArticleEvidence{{
			ID: articleID, Headline: "Central bank holds policy rates", Source: "Daily News",
			Category: "Economy", PublishedAt: time.Date(2026, time.August, 27, 8, 0, 0, 0, time.UTC),
			OriginalURL: "https://example.com/economy", Summary: "The central bank kept its policy rates unchanged.",
		}},
	}
	model := &fakeCompleter{response: llm.Response{
		Text:     `{"answer_markdown":"## Economy\n\nThe central bank held policy rates steady [1].","cited_indices":[1],"follow_up_questions":["How did markets respond?"]}`,
		Provider: "openrouter",
		Model:    "test-model",
	}}
	service := NewService(repository, model, func() time.Time {
		return time.Date(2026, time.August, 27, 12, 30, 0, 0, time.UTC)
	})

	result, err := service.Ask(context.Background(), userID, threadID, "What happened in the economy today?")

	require.NoError(t, err)
	require.Contains(t, model.request.System, "Write the complete answer in Sinhala")
	require.Contains(t, model.request.Input, "Central bank holds policy rates")
	require.Contains(t, model.request.Input, "Focus on the economy.")
	require.Equal(t, "## Economy\n\nThe central bank held policy rates steady [1].", result.Assistant.Content)
	require.Equal(t, []string{"How did markets respond?"}, result.Assistant.Suggestions)
	require.Len(t, result.Assistant.Citations, 1)
	require.Equal(t, articleID, result.Assistant.Citations[0].ArticleID)
	require.Equal(t, "Daily News", result.Assistant.Citations[0].Source)
	require.Equal(t, "What happened in the economy today?", repository.savedQuestion)
	require.Equal(t, result.Assistant.Citations, repository.savedAssistant.Citations)
}

func TestServiceAskRetriesATruncatedAnswerWithACompactBrief(t *testing.T) {
	threadID := uuid.New()
	userID := uuid.New()
	repository := &fakeRepository{
		settings:     Settings{ResponseLanguage: LanguageSinhala},
		conversation: Conversation{Thread: Thread{ID: threadID, UserID: userID, Title: "Daily briefing"}},
		articles: []ArticleEvidence{{
			ID: uuid.New(), Headline: "Policy rates held", Source: "Daily News",
			Category: "Economy", Summary: "The central bank kept its policy rates unchanged.",
		}},
	}
	model := &fakeCompleter{responses: []llm.Response{
		{Text: `{"answer_markdown":"An unfinished`, FinishReason: "length"},
		{Text: `{"answer_markdown":"## කෙටි සාරාංශය\n\nප්‍රතිපත්ති පොලී අනුපාත නොවෙනස්ව පවතී [1].","cited_indices":[1],"follow_up_questions":[]}`, Provider: "openrouter", Model: "test-model", FinishReason: "stop"},
	}}
	service := NewService(repository, model, time.Now)

	result, err := service.Ask(context.Background(), userID, threadID, "What happened in the economy today?")

	require.NoError(t, err)
	require.Len(t, model.requests, 2)
	require.Equal(t, 3200, model.requests[0].MaxTokens)
	require.Equal(t, 4000, model.requests[1].MaxTokens)
	require.Contains(t, model.requests[1].System, "COMPACT RETRY")
	require.Contains(t, result.Assistant.Content, "කෙටි සාරාංශය")
	require.Equal(t, "openrouter", repository.savedAssistant.Provider)
}

func TestServiceAskRejectsBlankQuestions(t *testing.T) {
	service := NewService(&fakeRepository{}, &fakeCompleter{}, time.Now)

	_, err := service.Ask(context.Background(), uuid.New(), uuid.New(), "   ")

	require.ErrorIs(t, err, ErrInvalidQuestion)
}

func TestServiceAskCarriesContextIntoAFollowUpQuestion(t *testing.T) {
	threadID := uuid.New()
	userID := uuid.New()
	from := time.Date(2026, time.August, 26, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.August, 27, 0, 0, 0, 0, time.UTC)
	repository := &fakeRepository{
		settings: Settings{ResponseLanguage: LanguageSinhala},
		conversation: Conversation{
			Thread: Thread{ID: threadID, UserID: userID, Title: "Economic briefing"},
			Messages: []Message{
				{Role: RoleUser, Content: "What happened in the economy?"},
				{Role: RoleAssistant, Content: "A briefing.", Search: &SearchSummary{Label: "Last 24 hours", From: from, To: to, ArticleCount: 10}},
			},
		},
		articles: []ArticleEvidence{{ID: uuid.New(), Headline: "Rates held", Source: "Daily News", Summary: "Rates were unchanged."}},
	}
	model := &fakeCompleter{response: llm.Response{Text: `{"answer_markdown":"More detail [1].","cited_indices":[1],"follow_up_questions":[]}`}}
	service := NewService(repository, model, func() time.Time { return to })

	_, err := service.Ask(context.Background(), userID, threadID, "Tell me more about the first story.")

	require.NoError(t, err)
	require.Equal(t, "economy", repository.searchedScope.Category)
	require.Equal(t, "Last 24 hours", repository.searchedScope.Label)
	require.Equal(t, from, repository.searchedScope.From)
}

func TestBuildPromptCapsTheEvidencePack(t *testing.T) {
	articles := make([]ArticleEvidence, 0, 50)
	for range 50 {
		articles = append(articles, ArticleEvidence{
			ID: uuid.New(), Headline: "A relevant newsroom headline", Source: "News source",
			Summary: strings.Repeat("Detailed evidence. ", 100), PublishedAt: time.Now(),
		})
	}

	prompt := buildPrompt("What happened?", nil, SearchScope{From: time.Now().Add(-24 * time.Hour), To: time.Now(), Label: "Last 24 hours"}, articles)

	require.Contains(t, prompt, "[32]")
	require.NotContains(t, prompt, "[33]")
	require.Less(t, len(prompt), 40_000)
}

func TestServiceAskExpandsAnAmbiguousQuestionForSinhalaRetrieval(t *testing.T) {
	threadID := uuid.New()
	userID := uuid.New()
	repository := &fakeRepository{
		settings:     Settings{ResponseLanguage: LanguageSinhala},
		conversation: Conversation{Thread: Thread{ID: threadID, UserID: userID, Title: "Fuel prices"}},
		articles:     []ArticleEvidence{{ID: uuid.New(), Headline: "Fuel report", Source: "Daily News", Summary: "A report about fuel."}},
	}
	model := &fakeCompleter{responses: []llm.Response{
		{Text: `{"category":"economy","search_terms":["fuel","prices","ඉන්ධන","මිල"]}`},
		{Text: `{"answer_markdown":"Fuel briefing [1].","cited_indices":[1],"follow_up_questions":[]}`},
	}}
	service := NewService(repository, model, time.Now)

	_, err := service.Ask(context.Background(), userID, threadID, "What is happening with fuel prices?")

	require.NoError(t, err)
	require.Equal(t, "economy", repository.searchedScope.Category)
	require.Contains(t, repository.searchedScope.Terms, "ඉන්ධන")
	require.False(t, repository.searchedScope.CategoryOnly)
	require.Len(t, model.requests, 2)
	require.Equal(t, "watch_tower_retrieval", model.requests[0].Task)
	require.Equal(t, "watch_tower_answer", model.requests[1].Task)
}

func TestServiceUpdateSettingsAcceptsEnglish(t *testing.T) {
	repository := &fakeRepository{settings: Settings{ResponseLanguage: LanguageSinhala}}
	service := NewService(repository, &fakeCompleter{}, time.Now)
	userID := uuid.New()

	settings, err := service.UpdateSettings(context.Background(), userID, LanguageEnglish)

	require.NoError(t, err)
	require.Equal(t, LanguageEnglish, settings.ResponseLanguage)
	require.Equal(t, userID, repository.settingsUpdatedBy)
}

func TestServiceUpdateSettingsRejectsUnsupportedLanguage(t *testing.T) {
	service := NewService(&fakeRepository{}, &fakeCompleter{}, time.Now)

	_, err := service.UpdateSettings(context.Background(), uuid.New(), "fr")

	require.ErrorIs(t, err, ErrInvalidSettings)
}

type fakeRepository struct {
	settings          Settings
	settingsUpdatedBy uuid.UUID
	conversation      Conversation
	articles          []ArticleEvidence
	savedQuestion     string
	savedAssistant    MessageDraft
	searchedScope     SearchScope
}

func (repository *fakeRepository) Settings(context.Context) (Settings, error) {
	if repository.settings.ResponseLanguage == "" {
		return Settings{ResponseLanguage: LanguageSinhala}, nil
	}
	return repository.settings, nil
}

func (repository *fakeRepository) UpdateSettings(_ context.Context, userID uuid.UUID, settings Settings) (Settings, error) {
	repository.settings = settings
	repository.settingsUpdatedBy = userID
	return settings, nil
}

func (repository *fakeRepository) CreateThread(_ context.Context, userID uuid.UUID, title string) (Thread, error) {
	return Thread{ID: uuid.New(), UserID: userID, Title: title}, nil
}

func (repository *fakeRepository) ListThreads(context.Context, uuid.UUID) ([]Thread, error) {
	return nil, nil
}

func (repository *fakeRepository) Conversation(context.Context, uuid.UUID, uuid.UUID) (Conversation, error) {
	return repository.conversation, nil
}

func (repository *fakeRepository) SearchArticles(_ context.Context, scope SearchScope) ([]ArticleEvidence, error) {
	repository.searchedScope = scope
	return repository.articles, nil
}

func (repository *fakeRepository) SaveExchange(_ context.Context, _ uuid.UUID, _ uuid.UUID, question string, assistant MessageDraft) (Conversation, error) {
	repository.savedQuestion = question
	repository.savedAssistant = assistant
	return Conversation{
		Thread: repository.conversation.Thread,
		Messages: append(repository.conversation.Messages,
			Message{Role: RoleUser, Content: question},
			Message{Role: RoleAssistant, Content: assistant.Content, Citations: assistant.Citations, Suggestions: assistant.Suggestions},
		),
	}, nil
}

func (repository *fakeRepository) DeleteThread(context.Context, uuid.UUID, uuid.UUID) error {
	return nil
}

type fakeCompleter struct {
	request   llm.Request
	requests  []llm.Request
	response  llm.Response
	responses []llm.Response
	err       error
}

func (completer *fakeCompleter) Complete(_ context.Context, request llm.Request) (llm.Response, error) {
	completer.request = request
	completer.requests = append(completer.requests, request)
	if len(completer.responses) > 0 {
		response := completer.responses[0]
		completer.responses = completer.responses[1:]
		return response, completer.err
	}
	return completer.response, completer.err
}

func (completer *fakeCompleter) CompleteWithModel(_ context.Context, request llm.Request, _, _ string) (llm.Response, error) {
	completer.request = request
	return completer.response, completer.err
}
