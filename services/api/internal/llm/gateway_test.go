package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCallProviderSupportsStructuredOpenAIResponses(t *testing.T) {
	t.Setenv("TEST_LLM_API_KEY", "test-secret")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "Bearer test-secret", request.Header.Get("Authorization"))
		var payload map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Equal(t, "json_schema", payload["response_format"].(map[string]any)["type"])
		require.Equal(t, "none", payload["reasoning_effort"])
		require.Equal(t, float64(1024), payload["max_tokens"])
		require.Equal(t, true, payload["stream"])
		require.Equal(t, true, payload["stream_options"].(map[string]any)["include_usage"])
		writer.Header().Set("Content-Type", "text/event-stream")
		_, err := writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"score\\\":\"},\"finish_reason\":null}]}\n\n"))
		require.NoError(t, err)
		_, err = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"0}\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":4}}\n\n"))
		require.NoError(t, err)
		_, err = writer.Write([]byte("data: [DONE]\n\n"))
		require.NoError(t, err)
	}))
	defer server.Close()

	gateway := &Gateway{client: server.Client()}
	firstTokenObserved := false
	response, err := gateway.callProvider(context.Background(), "openai_api", server.URL, "TEST_LLM_API_KEY", "test-model", Request{
		Task:             "narration_framing",
		JSONSchema:       map[string]any{"type": "object"},
		DisableReasoning: true,
		MaxTokens:        1024,
	}, func(int) { firstTokenObserved = true })

	require.NoError(t, err)
	require.JSONEq(t, `{"score":0}`, response.Text)
	require.True(t, firstTokenObserved)
	require.Equal(t, 12, *response.InputTokens)
	require.Equal(t, 4, *response.OutputTokens)
	require.Equal(t, "stop", response.FinishReason)
}

func TestCallProviderSendsConfiguredBearerToken(t *testing.T) {
	t.Setenv("TEST_LLM_API_KEY", "test-secret")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "Bearer test-secret", request.Header.Get("Authorization"))
		writer.Header().Set("Content-Type", "text/event-stream")
		_, err := writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))
		require.NoError(t, err)
	}))
	defer server.Close()

	gateway := &Gateway{client: server.Client()}
	response, err := gateway.callProvider(
		context.Background(), "openai_api", server.URL,
		"TEST_LLM_API_KEY", "remote-model", Request{}, nil,
	)

	require.NoError(t, err)
	require.Equal(t, "ok", response.Text)
}

func TestCheckOpenRouterReportsHealthyAuthenticatedKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/key", request.URL.Path)
		require.Equal(t, "Bearer test-secret", request.Header.Get("Authorization"))
		writer.Header().Set("Content-Type", "application/json")
		_, err := writer.Write([]byte(`{"data":{"is_free_tier":false,"limit":25,"limit_remaining":19.5,"expires_at":null}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	gateway := &Gateway{client: server.Client()}
	status, err := gateway.checkOpenRouter(context.Background(), server.URL, "test-secret")

	require.NoError(t, err)
	require.False(t, status.FreeTier)
	require.Equal(t, 25.0, *status.LimitUSD)
	require.Equal(t, 19.5, *status.LimitRemainingUSD)
	require.GreaterOrEqual(t, status.LatencyMS, int64(0))
}

func TestDecodeOpenRouterModelsPreservesPricingContextAndCapabilities(t *testing.T) {
	models, err := decodeOpenRouterModels(strings.NewReader(`{"data":[{
		"id":"deepseek/deepseek-v4-flash-0731",
		"name":"DeepSeek: DeepSeek V4 Flash 0731",
		"context_length":1310720,
		"pricing":{"prompt":"0.00000014","completion":"0.00000028"},
		"architecture":{"input_modalities":["text"],"output_modalities":["text"]},
		"supported_parameters":["max_tokens","reasoning","structured_outputs"]
	}]}`))

	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, 0.14, models[0].InputPricePerMillion)
	require.Equal(t, 0.28, models[0].OutputPricePerMillion)
	require.Equal(t, 1310720, models[0].ContextLength)
	require.NotNil(t, models[0].CompatibleTasks)
	require.ElementsMatch(t, []string{"classify", "narration_framing"}, models[0].CompatibleTasks)
}

func TestNarrationRejectsModelWithoutStructuredOutput(t *testing.T) {
	model := Model{
		OutputModalities:    []string{"text"},
		SupportedParameters: []string{"max_tokens", "reasoning"},
	}

	require.True(t, modelSupportsTask(model, "classify"))
	require.False(t, modelSupportsTask(model, "narration_framing"))
}

func TestCallProviderUsesOpenRouterReasoningAndRoutingParameters(t *testing.T) {
	t.Setenv("TEST_OPENROUTER_KEY", "test-secret")
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Nil(t, payload["reasoning_effort"])
		require.Equal(t, "none", payload["reasoning"].(map[string]any)["effort"])
		require.Equal(t, true, payload["provider"].(map[string]any)["require_parameters"])
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")),
			Request:    request,
		}, nil
	})
	gateway := &Gateway{client: &http.Client{Transport: transport}}

	response, err := gateway.callProvider(context.Background(), "openai_api", "https://openrouter.ai/api/v1", "TEST_OPENROUTER_KEY", "deepseek/deepseek-v4-flash-0731", Request{
		JSONSchema: map[string]any{"type": "object"}, DisableReasoning: true,
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "ok", response.Text)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
