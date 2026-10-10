package meta

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func expectedProof(token string) string {
	hash := hmac.New(sha256.New, []byte("test-secret"))
	_, _ = hash.Write([]byte(token))
	return hex.EncodeToString(hash.Sum(nil))
}
func testClient(server *httptest.Server) *Client {
	return NewClient(Config{AppID: "app-one", AppSecret: "test-secret", RedirectURL: "https://admin.invalid/api/admin/social/facebook/callback", BaseURL: server.URL}, server.Client())
}

func TestAuthorizeURL(t *testing.T) {
	client := NewClient(Config{AppID: "app", Version: "v26.0", RedirectURL: "https://admin.invalid/callback"}, nil)
	parsed, err := url.Parse(client.AuthorizeURL("random-state"))
	require.NoError(t, err)
	require.Equal(t, "www.facebook.com", parsed.Host)
	require.Equal(t, "/v26.0/dialog/oauth", parsed.Path)
	require.Equal(t, "random-state", parsed.Query().Get("state"))
	require.Equal(t, requiredScopes, parsed.Query().Get("scope"))
	client.config.LoginConfigID = "config-one"
	parsed, _ = url.Parse(client.AuthorizeURL("state"))
	require.Equal(t, "config-one", parsed.Query().Get("config_id"))
	require.Empty(t, parsed.Query().Get("scope"))
}

func TestTokenFlowAndPagePagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/access_token":
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, r.ParseForm())
			require.Equal(t, "test-secret", r.Form.Get("client_secret"))
			if r.Form.Get("code") != "" {
				require.Equal(t, "auth-code", r.Form.Get("code"))
				_, _ = io.WriteString(w, `{"access_token":"short"}`)
			} else {
				require.Equal(t, "fb_exchange_token", r.Form.Get("grant_type"))
				require.Equal(t, "short", r.Form.Get("fb_exchange_token"))
				require.Equal(t, expectedProof("short"), r.Form.Get("appsecret_proof"))
				_, _ = io.WriteString(w, `{"access_token":"long","expires_in":5000}`)
			}
		case "/me/accounts":
			require.Equal(t, "Bearer long", r.Header.Get("Authorization"))
			require.Equal(t, expectedProof("long"), r.URL.Query().Get("appsecret_proof"))
			require.Equal(t, "id,name,access_token,tasks,picture{url}", r.URL.Query().Get("fields"))
			if r.URL.Query().Get("after") == "" {
				_, _ = io.WriteString(w, `{"data":[{"id":"one","name":"Page One","access_token":"page-token","tasks":["CREATE_CONTENT"],"picture":{"data":{"url":"https://image.invalid/one"}}},{"id":"no","access_token":"other","tasks":["ANALYZE"]}],"paging":{"next":"https://evil.invalid/never-follow","cursors":{"after":"second"}}}`)
			} else {
				require.Equal(t, "second", r.URL.Query().Get("after"))
				_, _ = io.WriteString(w, `{"data":[{"id":"two","name":"Page Two","access_token":"page-two","tasks":["MANAGE"]}]}`)
			}
		case "/debug_token":
			require.Equal(t, "Bearer app-one|test-secret", r.Header.Get("Authorization"))
			require.Equal(t, expectedProof("app-one|test-secret"), r.URL.Query().Get("appsecret_proof"))
			require.Equal(t, "page-token", r.URL.Query().Get("input_token"))
			_, _ = io.WriteString(w, `{"data":{"is_valid":true,"app_id":"app-one","scopes":["pages_manage_posts"],"expires_at":0}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := testClient(server)
	short, err := client.ExchangeCode(context.Background(), "auth-code")
	require.NoError(t, err)
	long, err := client.ExchangeLongLived(context.Background(), short.AccessToken)
	require.NoError(t, err)
	pages, err := client.ListPages(context.Background(), long.AccessToken)
	require.NoError(t, err)
	require.Len(t, pages, 2)
	require.Equal(t, "one", pages[0].ID)
	info, err := client.DebugToken(context.Background(), pages[0].AccessToken)
	require.NoError(t, err)
	require.True(t, info.IsValid)
}

func TestPublishPhotoUsesMultipartBytesAndProof(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/one/photos" {
			require.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, r.ParseMultipartForm(1<<20))
			require.Equal(t, "true", r.FormValue("published"))
			require.Equal(t, "සිංහල caption", r.FormValue("message"))
			require.Equal(t, "page-token", r.FormValue("access_token"))
			require.Equal(t, expectedProof("page-token"), r.FormValue("appsecret_proof"))
			file, _, err := r.FormFile("source")
			require.NoError(t, err)
			defer file.Close()
			data, err := io.ReadAll(file)
			require.NoError(t, err)
			require.Equal(t, []byte("png-bytes"), data)
			_, _ = io.WriteString(w, `{"id":"photo","post_id":"one_post"}`)
		} else {
			require.Equal(t, "/one_post", r.URL.Path)
			require.Equal(t, "Bearer page-token", r.Header.Get("Authorization"))
			require.Equal(t, expectedProof("page-token"), r.URL.Query().Get("appsecret_proof"))
			_, _ = io.WriteString(w, `{"permalink_url":"https://www.facebook.com/one_post"}`)
		}
	}))
	defer server.Close()
	client := testClient(server)
	photo, err := client.PublishPhoto(context.Background(), "one", "page-token", []byte("png-bytes"), "සිංහල caption")
	require.NoError(t, err)
	require.Equal(t, "one_post", photo.PostID)
	link, err := client.Permalink(context.Background(), photo.PostID, "page-token")
	require.NoError(t, err)
	require.Equal(t, "https://www.facebook.com/one_post", link)
}

func TestGraphErrorsAndClassification(t *testing.T) {
	for _, code := range []int{190, 102, 10, 200, 299} {
		require.Equal(t, "needs_reconnect", (&Error{Code: code}).Classification())
	}
	for _, code := range []int{4, 17, 32, 613, 80001} {
		require.Equal(t, "rate_limited", (&Error{Code: code}).Classification())
	}
	require.Equal(t, "failed", (&Error{Code: 100}).Classification())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = io.WriteString(w, `{"error":{"message":"private-page-token appsecret_proof=secret","code":190,"error_subcode":463,"type":"OAuthException","fbtrace_id":"trace-one"}}`)
	}))
	defer server.Close()
	_, err := testClient(server).PublishPhoto(context.Background(), "one", "private-page-token", []byte("png"), "caption")
	var graphError *Error
	require.ErrorAs(t, err, &graphError)
	require.Equal(t, 463, graphError.Subcode)
	require.Equal(t, "trace-one", graphError.TraceID)
	require.NotContains(t, err.Error(), "private-page-token")
	require.NotContains(t, err.Error(), "appsecret_proof")
}

func TestPublishAmbiguousResponseIsNeverSafeToRetry(t *testing.T) {
	for _, body := range []string{`{"id":"photo"}`, `not json`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }))
		_, err := testClient(server).PublishPhoto(context.Background(), "one", "page-token", []byte("png"), "caption")
		var transport *TransportError
		require.ErrorAs(t, err, &transport)
		require.True(t, transport.Sent)
		server.Close()
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = connection.Close()
		}
	}))
	defer server.Close()
	_, err := testClient(server).PublishPhoto(context.Background(), "one", "page-token", []byte("png"), "caption")
	var transport *TransportError
	require.ErrorAs(t, err, &transport)
	require.True(t, transport.Sent)
}

func TestCancellationAndRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	client := testClient(server)
	client.http.Timeout = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.PublishPhoto(ctx, "one", "private-token", []byte("png"), "caption")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private-token")
	require.Equal(t, "access_token=[redacted] appsecret_proof=[redacted] [redacted]", Redact("access_token=abc appsecret_proof=xyz private-token", "private-token"))
	require.NotContains(t, (&Error{Code: 190, Type: "token=private", TraceID: strings.Repeat("t", 100)}).Error(), "private")
	require.False(t, errors.Is(err, context.Canceled)) // transport details deliberately stay private
}
