package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	require.Equal(t, "/social?error=session_required", recorder.Header().Get("Location"))
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
	handler.serve(recorder, request)
	require.Equal(t, http.StatusSeeOther, recorder.Code)
	require.Equal(t, "/social?error=not_configured", recorder.Header().Get("Location"))
}
