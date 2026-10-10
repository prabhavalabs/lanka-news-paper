package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/config"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/iam"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social"
	"github.com/stretchr/testify/require"
)

func TestSocialRoutesRequireAuthenticationAndCSRF(t *testing.T) {
	router := NewRouter(Dependencies{})
	for _, path := range []string{"/api/admin/social/status", "/api/admin/social/settings", "/api/admin/social/posts", "/api/admin/social/facebook/pending-pages?state_id=one"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/admin/social/facebook/connect", strings.NewReader("{}"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	request = httptest.NewRequest(http.MethodGet, "/api/admin/social/facebook/callback?code=private-code&state=state", nil)
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusSeeOther, recorder.Code)
	require.Equal(t, "/social?error=not_configured", recorder.Header().Get("Location"))
	require.NotContains(t, recorder.Body.String(), "private-code")
}

func TestSocialHandlerRejectsRoleAndInvalidBodies(t *testing.T) {
	service := social.NewService(nil, config.SocialConfig{}, nil, nil, nil, nil)
	handler := socialHandler{service: service}
	request := httptest.NewRequest(http.MethodGet, "/api/admin/social/status", nil)
	recorder := httptest.NewRecorder()
	handler.serve(recorder, request)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	for _, body := range []string{`{"unexpected":"secret"}`, `{} {}`, `null`, ``} {
		request = httptest.NewRequest(http.MethodPost, "/api/admin/social/posts", strings.NewReader(body))
		request = request.WithContext(context.WithValue(request.Context(), userContextKey{}, iam.User{ID: uuid.New(), Role: "administrator"}))
		recorder = httptest.NewRecorder()
		handler.serve(recorder, request)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/admin/social/facebook/callback?code=private-code", nil)
	request = request.WithContext(context.WithValue(request.Context(), userContextKey{}, iam.User{ID: uuid.New(), Role: "administrator"}))
	recorder = httptest.NewRecorder()
	handler.callback(recorder, request)
	require.Equal(t, http.StatusSeeOther, recorder.Code)
	require.Equal(t, "/social?error=not_configured", recorder.Header().Get("Location"))
}

func TestSocialCallbackIsRateLimitedWithoutSession(t *testing.T) {
	router := NewRouter(Dependencies{})
	for attempt := 0; attempt < 6; attempt++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/social/facebook/callback?code=private-code&error_description=private-message", nil))
		require.Equal(t, http.StatusSeeOther, recorder.Code)
		location := "/social?error=not_configured"
		if attempt == 5 {
			location = "/social?error=rate_limited"
		}
		require.Equal(t, location, recorder.Header().Get("Location"))
		require.NotContains(t, recorder.Body.String(), "private")
	}
}

func TestSocialMutationsRequireCSRFAndSession(t *testing.T) {
	router := NewRouter(Dependencies{})
	for _, route := range []struct{ method, path string }{
		{"POST", "/api/admin/social/settings"}, {"POST", "/api/admin/social/facebook/connect"},
		{"POST", "/api/admin/social/facebook/select"}, {"POST", "/api/admin/social/facebook/disconnect"},
		{"POST", "/api/admin/social/posts"}, {"PATCH", "/api/admin/social/posts/one"},
		{"POST", "/api/admin/social/posts/one/approve"}, {"POST", "/api/admin/social/posts/one/reject"},
		{"POST", "/api/admin/social/posts/one/retry"}, {"POST", "/api/admin/social/posts/one/render"},
		{"POST", "/api/admin/social/run-selection"},
	} {
		request := httptest.NewRequest(route.method, route.path, strings.NewReader("{}"))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusForbidden, recorder.Code, route.path)
		request.Header.Set("X-SNAP-CSRF", "1")
		recorder = httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, route.path)
	}
}

func TestSocialCallbackDoesNotForwardSensitiveReferrer(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewRouter(Dependencies{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/admin/social/facebook/callback?code=private-code&state=private-state", nil))
	require.Equal(t, "no-referrer", recorder.Header().Get("Referrer-Policy"))
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}

func TestSessionLogoutKeepsStrictCookie(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := newAuthHandler(nil, time.Hour, false)
	handler.logout(recorder, httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil))
	require.Equal(t, http.SameSiteStrictMode, recorder.Result().Cookies()[0].SameSite)
}
