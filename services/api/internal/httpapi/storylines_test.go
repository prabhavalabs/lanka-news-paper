package httpapi

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStorylineAdminRoutesRequireAuthentication(t *testing.T) {
	router := NewRouter(Dependencies{})
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/admin/storylines"}, {"PATCH", "/api/admin/storylines/one"},
		{"POST", "/api/admin/storylines/refresh"}, {"POST", "/api/admin/storylines/one/events/two"},
		{"DELETE", "/api/admin/storylines/one/events/two"},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(route.method, route.path, nil))
		require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, w.Code)
	}
}
