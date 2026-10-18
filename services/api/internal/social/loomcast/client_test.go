package loomcast

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientCreatesUploadsAndFinalizes(t *testing.T) {
	var uploaded []byte
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/upload" {
			require.Equal(t, http.MethodPut, request.Method)
			require.Empty(t, request.Header.Get("Authorization"), "the key never goes to the file store")
			require.Equal(t, "image/png", request.Header.Get("Content-Type"))
			require.Equal(t, "c2lnbmVk", request.Header.Get("x-amz-checksum-sha256"))
			require.EqualValues(t, 3, request.ContentLength)
			uploaded, _ = io.ReadAll(request.Body)
			return
		}
		require.Equal(t, "Bearer lc_api_key", request.Header.Get("Authorization"))
		switch request.URL.Path {
		case "/api/v1/posts":
			require.Equal(t, "thulawa-1", request.Header.Get("Idempotency-Key"))
			var body map[string]any
			require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
			require.Equal(t, "image", body["format"])
			require.Equal(t, "post-1", body["externalRef"])
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "lc-1", "status": "awaiting_media", "uploads": []map[string]any{{
				"mediaId": "m1", "url": server.URL + "/upload", "method": "PUT",
				"headers": map[string]string{"Content-Type": "image/png", "Content-Length": "3", "x-amz-checksum-sha256": "c2lnbmVk"},
			}}})
		case "/api/v1/posts/lc-1/finalize":
			require.Equal(t, http.MethodPost, request.Method)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "lc-1", "status": "scheduled", "targets": []map[string]any{{"channelId": "ch", "platform": "facebook", "status": "queued"}}})
		default:
			t.Fatalf("unexpected %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL + "/", APIKey: "lc_api_key"}, server.Client())
	ctx := context.Background()
	post, err := client.CreatePost(ctx, "thulawa-1", NewPost{Format: "image", Caption: "පුවත", Media: []Media{{Filename: "card.png", MIME: "image/png", Bytes: 3, SHA256: "ab"}},
		Channels: []ChannelRef{{ChannelID: "ch"}}, ScheduledAt: time.Now().Add(3 * time.Minute), ExternalRef: "post-1"})
	require.NoError(t, err)
	require.Len(t, post.Uploads, 1)
	require.NoError(t, client.Upload(ctx, post.Uploads[0], []byte("png")))
	require.Equal(t, "png", string(uploaded))
	post, err = client.Finalize(ctx, post.ID)
	require.NoError(t, err)
	require.Equal(t, "scheduled", post.Status)
	require.Equal(t, "queued", *post.Target("ch").Status)
	require.Nil(t, post.Target("other"))
}

func TestClientErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/channels":
			w.Header().Set("Retry-After", "42")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"Slow down"}}`))
		case "/api/v1/posts/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"No post"}}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`not json`))
		}
	}))
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, APIKey: "lc_api_key"}, server.Client())
	ctx := context.Background()
	_, err := client.Channels(ctx)
	var apiError *APIError
	require.True(t, errors.As(err, &apiError))
	require.True(t, apiError.Temporary())
	require.Equal(t, 42*time.Second, apiError.RetryAfter)
	require.Equal(t, "rate_limited", apiError.Code)
	_, err = client.Post(ctx, "missing")
	require.True(t, errors.As(err, &apiError))
	require.Equal(t, http.StatusNotFound, apiError.Status)
	require.False(t, apiError.Temporary())
	_, err = client.Cancel(ctx, "any")
	require.True(t, errors.As(err, &apiError))
	require.True(t, apiError.Unauthorized())
	require.NotContains(t, err.Error(), "lc_api_key")
	server.Close()
	_, err = client.Post(ctx, "gone")
	var transport *TransportError
	require.True(t, errors.As(err, &transport))
	require.Error(t, client.Upload(ctx, Upload{URL: "file:///etc/passwd"}, nil))
}
