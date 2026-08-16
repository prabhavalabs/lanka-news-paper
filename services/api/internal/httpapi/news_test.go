package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
	"github.com/stretchr/testify/require"
)

type emptyNewsReader struct{}

func (emptyNewsReader) ListPublic(context.Context, int) (publish.Page, error) {
	return publish.Page{Items: []publish.Article{}}, nil
}

func TestListNewsReturnsEmptyPage(t *testing.T) {
	router := NewRouter(Dependencies{
		AllowedOrigins: []string{"http://127.0.0.1:5173"},
		News:           emptyNewsReader{},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/news", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"items":[],"next_cursor":null}`, recorder.Body.String())
}
