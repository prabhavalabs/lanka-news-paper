package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCallProviderSupportsStructuredOpenAICompatibleResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Empty(t, request.Header.Get("Authorization"))
		var payload map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Equal(t, "json_schema", payload["response_format"].(map[string]any)["type"])
		require.Equal(t, "none", payload["reasoning_effort"])
		require.Equal(t, float64(1024), payload["max_tokens"])
		_, err := writer.Write([]byte(`{"choices":[{"message":{"content":"{\"score\":0}"}}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	gateway := &Gateway{client: server.Client()}
	response, err := gateway.callProvider(context.Background(), "openai_compatible", server.URL, "", "local-model", Request{
		Task:             "narration_framing",
		JSONSchema:       map[string]any{"type": "object"},
		DisableReasoning: true,
		MaxTokens:        1024,
	})

	require.NoError(t, err)
	require.JSONEq(t, `{"score":0}`, response)
}

func TestCallProviderSendsConfiguredBearerToken(t *testing.T) {
	t.Setenv("TEST_LLM_API_KEY", "test-secret")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "Bearer test-secret", request.Header.Get("Authorization"))
		_, err := writer.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	gateway := &Gateway{client: server.Client()}
	response, err := gateway.callProvider(
		context.Background(), "openai_compatible", server.URL,
		"TEST_LLM_API_KEY", "remote-model", Request{},
	)

	require.NoError(t, err)
	require.Equal(t, "ok", response)
}
