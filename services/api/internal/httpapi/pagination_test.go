package httpapi

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		page    int
		perPage int
		search  string
		wantErr bool
	}{
		{name: "defaults", page: 1, perPage: 10},
		{name: "values", query: "?page=3&per_page=25&search=+newsroom+", page: 3, perPage: 25, search: "newsroom"},
		{name: "invalid page", query: "?page=0", wantErr: true},
		{name: "invalid page text", query: "?page=many", wantErr: true},
		{name: "invalid page size", query: "?per_page=101", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/api/admin/sources"+test.query, nil)
			params, err := parsePagination(request)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.page, params.Page)
			require.Equal(t, test.perPage, params.PerPage)
			require.Equal(t, test.search, params.Search)
		})
	}
}

func TestParseFilter(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/admin/sources?status=active", nil)
	value, err := parseFilter(request, "status", "active", "held")
	require.NoError(t, err)
	require.Equal(t, "active", value)

	request = httptest.NewRequest("GET", "/api/admin/sources?status=archived", nil)
	_, err = parseFilter(request, "status", "active", "held")
	require.Error(t, err)
}
