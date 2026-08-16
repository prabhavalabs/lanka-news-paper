package httpapi

import (
	"net/http"
	"strconv"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/publish"
)

type newsHandler struct {
	reader *publish.Store
}

func (handler newsHandler) list(w http.ResponseWriter, request *http.Request) {
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	page, err := handler.reader.ListFiltered(request.Context(), publish.Filter{
		Category:   request.URL.Query().Get("category"),
		SourceID:   request.URL.Query().Get("source"),
		Query:      request.URL.Query().Get("q"),
		SourceType: request.URL.Query().Get("type"),
		From:       request.URL.Query().Get("from"),
		To:         request.URL.Query().Get("to"),
		Cursor:     request.URL.Query().Get("cursor"),
		Limit:      limit,
	})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "The request could not be completed.")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (handler newsHandler) one(w http.ResponseWriter, request *http.Request) {
	item, err := handler.reader.Get(request.Context(), request.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", "Article is not public.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler newsHandler) categories(w http.ResponseWriter, request *http.Request) {
	items, err := handler.reader.Categories(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "The request could not be completed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler newsHandler) sources(w http.ResponseWriter, request *http.Request) {
	items, err := handler.reader.Sources(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "The request could not be completed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler newsHandler) source(w http.ResponseWriter, request *http.Request) {
	item, err := handler.reader.GetSource(request.Context(), request.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", "Source is not public.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler newsHandler) events(w http.ResponseWriter, request *http.Request) {
	items, err := handler.reader.ListEvents(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "The request could not be completed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler newsHandler) event(w http.ResponseWriter, request *http.Request) {
	item, err := handler.reader.GetEvent(request.Context(), request.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", "Event is not public.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler newsHandler) breaking(w http.ResponseWriter, request *http.Request) {
	items, err := handler.reader.Breaking(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "The request could not be completed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler newsHandler) brief(w http.ResponseWriter, request *http.Request) {
	item, err := handler.reader.LatestBrief(request.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"title_si": "උදෑසන සංග්‍රහය", "body_si": "සංග්‍රහයක් තවම නැත.", "model": ""})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler newsHandler) complain(w http.ResponseWriter, request *http.Request) {
	var body struct {
		EntityType string `json:"entity_type"`
		EntityID   string `json:"entity_id"`
		Reason     string `json:"reason"`
		Contact    string `json:"contact"`
	}
	if err := decodeJSON(request, &body); err != nil || body.Reason == "" {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "A reason is required.")
		return
	}
	if err := handler.reader.FileComplaint(request.Context(), body.EntityType, body.EntityID, body.Reason, body.Contact); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
}
