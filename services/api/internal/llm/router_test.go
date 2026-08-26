package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeCompleter struct {
	defaultResponse  Response
	explicitResponse Response
	provider         string
	model            string
}

func (completer *fakeCompleter) Complete(context.Context, Request) (Response, error) {
	return completer.defaultResponse, nil
}

func (completer *fakeCompleter) CompleteWithModel(_ context.Context, _ Request, provider, model string) (Response, error) {
	completer.provider, completer.model = provider, model
	return completer.explicitResponse, nil
}

type fakeModelProvider struct {
	response Response
	model    string
}

func (provider *fakeModelProvider) ID() string { return "codex_cli" }

func (provider *fakeModelProvider) Complete(_ context.Context, _ Request, model string) (Response, error) {
	provider.model = model
	return provider.response, nil
}

func TestRouterUsesDefaultProfilesForUnselectedCompletions(t *testing.T) {
	defaults := &fakeCompleter{defaultResponse: Response{Text: "default", Provider: "profile", Model: "profile-model"}}
	router := NewRouter(defaults)

	response, err := router.Complete(context.Background(), Request{Task: "article_summary"})

	require.NoError(t, err)
	require.Equal(t, defaults.defaultResponse, response)
}

func TestRouterRoutesRegisteredExplicitProvider(t *testing.T) {
	defaults := &fakeCompleter{}
	provider := &fakeModelProvider{response: Response{Text: "codex", Provider: "codex_cli", Model: "gpt-test"}}
	router := NewRouter(defaults, provider)

	response, err := router.CompleteWithModel(context.Background(), Request{Task: "content_cleaning"}, "codex_cli", "gpt-test")

	require.NoError(t, err)
	require.Equal(t, provider.response, response)
	require.Equal(t, "gpt-test", provider.model)
}

func TestRouterDelegatesUnregisteredExplicitProvider(t *testing.T) {
	defaults := &fakeCompleter{explicitResponse: Response{Text: "openrouter", Provider: "openrouter", Model: "model-a"}}
	router := NewRouter(defaults)

	response, err := router.CompleteWithModel(context.Background(), Request{Task: "article_summary"}, "openrouter", "model-a")

	require.NoError(t, err)
	require.Equal(t, defaults.explicitResponse, response)
	require.Equal(t, "openrouter", defaults.provider)
	require.Equal(t, "model-a", defaults.model)
}
