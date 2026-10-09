package llm

import (
	"context"
	"testing"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm/codex"
	"github.com/stretchr/testify/require"
)

type fakeCodexClient struct {
	options codex.CallOptions
	result  codex.Result
	err     error
}

func (client *fakeCodexClient) Complete(_ context.Context, options codex.CallOptions) (codex.Result, error) {
	client.options = options
	return client.result, client.err
}
func (*fakeCodexClient) Probe(context.Context) codex.Status {
	return codex.Status{Ready: true, AuthMethod: "chatgpt"}
}

func TestCodexDispatchUsesProfileEffortBeforeDisableReasoning(t *testing.T) {
	client := &fakeCodexClient{result: codex.Result{Text: `{"score":0}`, InputTokens: 12, OutputTokens: 4}}
	gateway := NewGateway(nil, client)
	profile := providerProfile{kind: "codex_cli", model: "gpt-6-luna", timeout: 180, reasoningEffort: "medium"}
	request := Request{System: "system", Input: "input", DisableReasoning: true, JSONSchema: map[string]any{"type": "object"}}
	result, err := gateway.callProvider(context.Background(), profile, request)
	require.NoError(t, err)
	require.Equal(t, "medium", client.options.ReasoningEffort)
	require.Equal(t, 180*time.Second, client.options.Timeout)
	require.Equal(t, request.System, client.options.System)
	require.Equal(t, request.Input, client.options.Input)
	require.Equal(t, request.JSONSchema, client.options.JSONSchema)
	require.Equal(t, 12, *result.InputTokens)
	require.Equal(t, 4, *result.OutputTokens)
	require.Equal(t, "stop", result.FinishReason)
	profile.reasoningEffort = ""
	_, err = gateway.callProvider(context.Background(), profile, request)
	require.NoError(t, err)
	require.Equal(t, "low", client.options.ReasoningEffort)
}

func TestFallbackAndConfiguredCatalog(t *testing.T) {
	gateway := NewGateway(nil)
	require.Equal(t, "keyword-rules", gateway.fallback(Request{Task: "classify", Input: "politics"}).Provider)
	require.Equal(t, "none", gateway.fallback(Request{Task: "article_summary"}).Provider)
	t.Setenv("SNAP_CODEX_MODELS", "model-a, model-b")
	models, err := gateway.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "model-a", models[0].ID)
	require.ElementsMatch(t, orderedTasks, models[0].CompatibleTasks)
	require.Equal(t, []string{"low", "medium", "high"}, models[0].ReasoningEfforts)
	require.NoError(t, validateProfile("classify", "codex_cli", "model-a", "low"))
	require.Error(t, validateProfile("classify", "retired", "model-a", "low"))
	require.Error(t, validateProfile("classify", "codex_cli", "unknown", "low"))
	require.Error(t, validateProfile("classify", "codex_cli", "model-a", "extreme"))
}

func TestBlockedStatusExpiresWithoutExtendingCooldown(t *testing.T) {
	now := time.Now()
	recent, expired := now.Add(-time.Minute), now.Add(-16*time.Minute)
	for _, status := range []string{"auth_expired", "quota_exhausted"} {
		require.True(t, blockedStatus(status, &recent, now, 15*time.Minute))
		require.False(t, blockedStatus(status, &expired, now, 15*time.Minute))
		require.False(t, blockedStatus(status, nil, now, 15*time.Minute))
	}
	require.False(t, blockedStatus("operational", &recent, now, 15*time.Minute))
}
