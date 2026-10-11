package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/frontpage"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
)

type frontPageHandler struct {
	service *frontpage.Service
	media   *media.Store
	rebuild func(context.Context) error
}

func (handler frontPageHandler) available(w http.ResponseWriter) bool {
	if handler.service == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/unavailable", "Front page unavailable", "The front page is not configured.")
		return false
	}
	return true
}

// curator allows the chief editor and administrators.
func (handler frontPageHandler) curator(w http.ResponseWriter, request *http.Request) bool {
	if !handler.available(w) {
		return false
	}
	switch currentUser(request).Role {
	case "administrator", "chief_editor":
		return true
	}
	writeProblem(w, http.StatusForbidden, "https://snap.local/problems/forbidden", "Forbidden", "Only the chief editor or an administrator can change the front page and editorials.")
	return false
}

func (handler frontPageHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, frontpage.ErrInvalid):
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", strings.TrimPrefix(err.Error(), frontpage.ErrInvalid.Error()+": "))
	case errors.Is(err, frontpage.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", "The requested item does not exist.")
	default:
		slog.Error("front page request failed", "error", err)
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Request failed", "The front page is temporarily unavailable.")
	}
}

func (handler frontPageHandler) publicFrontPage(w http.ResponseWriter, request *http.Request) {
	if !handler.available(w) {
		return
	}
	page, err := handler.service.FrontPage(request.Context())
	if err != nil {
		handler.writeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, page)
}

func (handler frontPageHandler) publicEditorials(w http.ResponseWriter, request *http.Request) {
	if !handler.available(w) {
		return
	}
	items, err := handler.service.Editorials(request.Context())
	if err != nil {
		handler.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler frontPageHandler) publicEditorial(w http.ResponseWriter, request *http.Request) {
	if !handler.available(w) {
		return
	}
	item, err := handler.service.Editorial(request.Context(), request.PathValue("id"))
	if err != nil {
		handler.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler frontPageHandler) adminFrontPage(w http.ResponseWriter, request *http.Request) {
	if !handler.curator(w, request) {
		return
	}
	if request.Method == http.MethodGet {
		view, err := handler.service.Admin(request.Context())
		if err != nil {
			handler.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
		return
	}
	var body struct {
		Placements []frontpage.PlacementInput `json:"placements"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "A list of placements is required.")
		return
	}
	view, err := handler.service.Replace(request.Context(), currentUser(request).ID, body.Placements)
	if err != nil {
		handler.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (handler frontPageHandler) adminSource(w http.ResponseWriter, request *http.Request) {
	if !handler.curator(w, request) {
		return
	}
	var body struct {
		FrontPage *bool `json:"front_page"`
	}
	if err := decodeJSON(request, &body); err != nil || body.FrontPage == nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "front_page must be true or false.")
		return
	}
	source, err := handler.service.SetSourceFrontPage(request.Context(), currentUser(request).ID, request.PathValue("id"), *body.FrontPage)
	if err != nil {
		handler.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func (handler frontPageHandler) adminEditorials(w http.ResponseWriter, request *http.Request) {
	if !handler.curator(w, request) {
		return
	}
	if request.Method == http.MethodGet {
		items, err := handler.service.AdminEditorials(request.Context())
		if err != nil {
			handler.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	var input frontpage.EditorialInput
	if err := decodeJSON(request, &input); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "Title, body and author are required.")
		return
	}
	item, err := handler.service.CreateEditorial(request.Context(), currentUser(request).ID, input)
	if err != nil {
		handler.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (handler frontPageHandler) adminEditorial(w http.ResponseWriter, request *http.Request) {
	if !handler.curator(w, request) {
		return
	}
	id := request.PathValue("id")
	if request.Method == http.MethodDelete {
		if err := handler.service.DeleteEditorial(request.Context(), currentUser(request).ID, id); err != nil {
			handler.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	var input frontpage.EditorialInput
	if err := decodeJSON(request, &input); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "Title, body and author are required.")
		return
	}
	item, err := handler.service.UpdateEditorial(request.Context(), currentUser(request).ID, id, input)
	if err != nil {
		handler.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler frontPageHandler) adminEditorialAction(w http.ResponseWriter, request *http.Request) {
	if !handler.curator(w, request) {
		return
	}
	item, err := handler.service.SetEditorialStatus(request.Context(), currentUser(request).ID, request.PathValue("id"), request.PathValue("action"))
	if err != nil {
		handler.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler frontPageHandler) publicReport(w http.ResponseWriter, request *http.Request) {
	if !handler.available(w) {
		return
	}
	report, image, err := handler.service.Report(request.Context(), request.PathValue("kind"), request.PathValue("id"))
	if err != nil {
		handler.writeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, map[string]any{"report": report, "image": image})
}

// illustration serves a generated front-page illustration; only illustration files are public.
func (handler frontPageHandler) illustration(w http.ResponseWriter, request *http.Request) {
	key, ok := frontpage.IllustrationKey(request.PathValue("name"))
	if !ok || handler.media == nil {
		http.NotFound(w, request)
		return
	}
	body, contentType, err := handler.media.Open(request.Context(), key)
	if err != nil {
		http.NotFound(w, request)
		return
	}
	defer body.Close()
	if contentType == "" {
		contentType = "image/jpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if _, err := io.Copy(w, body); err != nil {
		slog.WarnContext(request.Context(), "stream illustration", "error", err)
	}
}

func (handler frontPageHandler) adminRebuild(w http.ResponseWriter, request *http.Request) {
	if !handler.curator(w, request) {
		return
	}
	if handler.rebuild == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/unavailable", "Rebuild unavailable", "The job queue is not configured.")
		return
	}
	if err := handler.rebuild(request.Context()); err != nil {
		handler.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}
