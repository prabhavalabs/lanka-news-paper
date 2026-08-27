package newsletter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResendSenderSendsHTMLAndTextWithIdempotency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		require.Equal(t, "Bearer re_test", request.Header.Get("Authorization"))
		require.Equal(t, "edition-recipient", request.Header.Get("Idempotency-Key"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		require.Equal(t, "brief@example.com", body["from"])
		require.Equal(t, "Morning", body["subject"])
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":"message-123"}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	sender := NewResendSender("re_test", server.Client())
	sender.endpoint = server.URL

	id, err := sender.Send(context.Background(), EmailMessage{
		From: "brief@example.com", To: "reader@example.com", Subject: "Morning",
		HTML: "<p>Morning</p>", Text: "Morning", IdempotencyKey: "edition-recipient",
	})

	require.NoError(t, err)
	require.Equal(t, "message-123", id)
}

func TestResendSenderReturnsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, err := w.Write([]byte(`{"message":"sender is invalid"}`))
		require.NoError(t, err)
	}))
	defer server.Close()
	sender := NewResendSender("re_test", server.Client())
	sender.endpoint = server.URL

	_, err := sender.Send(context.Background(), EmailMessage{From: "brief@example.com", To: "reader@example.com"})

	require.ErrorContains(t, err, "sender is invalid")
}
