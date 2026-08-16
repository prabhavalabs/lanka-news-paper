package httpapi

import (
	"net/http"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
)

type newsHandler struct {
	reader publish.Reader
}

func (handler newsHandler) list(w http.ResponseWriter, request *http.Request) {
	page, err := handler.reader.ListPublic(request.Context(), 20)
	if err != nil {
		writeProblem(
			w,
			http.StatusInternalServerError,
			"https://snap.local/problems/internal",
			"Internal server error",
			"The request could not be completed.",
		)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
