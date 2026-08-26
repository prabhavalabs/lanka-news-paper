package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseArticleFilters_DefaultsToAllArticles(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/admin/articles", nil)
	now := time.Date(2026, time.August, 25, 20, 0, 0, 0, time.UTC)

	filters, err := parseArticleFilters(request, now)

	require.NoError(t, err)
	require.Empty(t, filters.Status)
	require.Empty(t, filters.PipelineStatus)
	require.Empty(t, filters.Category)
	require.Empty(t, filters.SourceID)
	require.Nil(t, filters.Since)
}

func TestParseArticleFilters_AcceptsAllSupportedFilters(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/admin/articles?status=published&pipeline=succeeded&category=politics&source=996bfe03-28b9-45d8-b1b5-9689cb2ae3ef&days=7",
		nil,
	)
	now := time.Date(2026, time.August, 25, 20, 0, 0, 0, time.UTC)

	filters, err := parseArticleFilters(request, now)

	require.NoError(t, err)
	require.Equal(t, "published", filters.Status)
	require.Equal(t, "succeeded", filters.PipelineStatus)
	require.Equal(t, "politics", filters.Category)
	require.Equal(t, "996bfe03-28b9-45d8-b1b5-9689cb2ae3ef", filters.SourceID)
	require.NotNil(t, filters.Since)
	require.Equal(t, now.Add(-7*24*time.Hour), *filters.Since)
}

func TestParseArticleFilters_RejectsInvalidSource(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/admin/articles?source=not-a-uuid", nil)

	_, err := parseArticleFilters(request, time.Now())

	require.EqualError(t, err, "source must be a UUID")
}

func TestParseArticleFilters_RejectsUnsupportedTimePeriod(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/admin/articles?days=365", nil)

	_, err := parseArticleFilters(request, time.Now())

	require.EqualError(t, err, "days must be 1, 7, 30, or 90")
}

func TestParseArticleFilters_RejectsLongCategory(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/admin/articles?category="+strings.Repeat("a", 51), nil)

	_, err := parseArticleFilters(request, time.Now())

	require.EqualError(t, err, "category is too long")
}

func TestParseArticleFilters_RejectsInvalidStatus(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/admin/articles?status=draft", nil)

	_, err := parseArticleFilters(request, time.Now())

	require.EqualError(t, err, "status has an unsupported value")
}
