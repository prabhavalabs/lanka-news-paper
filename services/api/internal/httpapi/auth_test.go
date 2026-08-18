package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoginLimiter(t *testing.T) {
	var limiter loginLimiter
	now := time.Now()
	for range 5 {
		require.True(t, limiter.Allow(now))
	}
	require.False(t, limiter.Allow(now))
	require.True(t, limiter.Allow(now.Add(time.Minute+time.Second)))
}

func TestCSRFMiddleware(t *testing.T) {
	handler := withCSRF(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	blocked := httptest.NewRecorder()
	handler.ServeHTTP(blocked, httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil))
	require.Equal(t, http.StatusForbidden, blocked.Code)

	allowedRequest := httptest.NewRequest(http.MethodPost, "/api/admin/logout", nil)
	allowedRequest.Header.Set("X-SNAP-CSRF", "1")
	allowed := httptest.NewRecorder()
	handler.ServeHTTP(allowed, allowedRequest)
	require.Equal(t, http.StatusNoContent, allowed.Code)
}
