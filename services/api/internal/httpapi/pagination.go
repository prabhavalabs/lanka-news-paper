package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
)

func parsePagination(request *http.Request) (pagination.Params, error) {
	params := pagination.Params{
		Page:    1,
		PerPage: 10,
		Search:  strings.TrimSpace(request.URL.Query().Get("search")),
	}
	if len(params.Search) > 200 {
		return pagination.Params{}, errors.New("search must be 200 characters or fewer")
	}

	var err error
	if value := request.URL.Query().Get("page"); value != "" {
		params.Page, err = strconv.Atoi(value)
		if err != nil || params.Page < 1 {
			return pagination.Params{}, errors.New("page must be a positive integer")
		}
	}
	if value := request.URL.Query().Get("per_page"); value != "" {
		params.PerPage, err = strconv.Atoi(value)
		if err != nil || params.PerPage < 1 || params.PerPage > 100 {
			return pagination.Params{}, errors.New("per_page must be between 1 and 100")
		}
	}
	return params, nil
}

func parseFilter(request *http.Request, name string, allowed ...string) (string, error) {
	value := request.URL.Query().Get(name)
	if value == "" {
		return "", nil
	}
	for _, candidate := range allowed {
		if value == candidate {
			return value, nil
		}
	}
	return "", errors.New(name + " has an unsupported value")
}

func writePage[T any](w http.ResponseWriter, items []T, total int, params pagination.Params) {
	writeJSON(w, http.StatusOK, pagination.Page[T]{
		Items:      items,
		Pagination: pagination.NewMeta(params, total),
	})
}
