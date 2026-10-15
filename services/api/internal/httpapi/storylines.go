package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/storyline"
)

type storylineHandler struct {
	store   *storyline.Store
	refresh func(context.Context) error
}

func (h storylineHandler) public(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeStorylineProblem(w, errors.New("unavailable"))
		return
	}
	if key := r.PathValue("idOrSlug"); key != "" {
		item, err := h.store.Get(r.Context(), key, false)
		if err != nil {
			writeStorylineProblem(w, err)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
		writeJSON(w, http.StatusOK, item.Public())
	} else {
		items, err := h.store.List(r.Context(), false)
		if err != nil {
			writeStorylineProblem(w, err)
			return
		}
		// Trending cards only need the latest development, not the whole timeline.
		public := make([]storyline.PublicStoryline, len(items))
		for i := range items {
			public[i] = items[i].Public()
			public[i].Events = nil
		}
		w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
		writeJSON(w, http.StatusOK, map[string]any{"items": public})
	}
}
func (h storylineHandler) admin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !requireAdministrator(w, r) {
		return
	}
	if h.store == nil {
		writeStorylineProblem(w, errors.New("unavailable"))
		return
	}
	var result any
	var err error
	code := http.StatusOK
	switch {
	case r.URL.Path == "/api/admin/storylines/refresh":
		if h.refresh == nil {
			writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/storyline", "Unavailable", "Storyline refresh is unavailable.")
			return
		}
		err = h.refresh(r.Context())
		result = map[string]bool{"ok": true}
		code = http.StatusAccepted
	case r.PathValue("eventID") != "":
		err = h.store.Membership(r.Context(), r.PathValue("id"), r.PathValue("eventID"), r.Method == http.MethodDelete)
		result = map[string]bool{"ok": true}
	case r.Method == http.MethodPatch:
		var input storyline.Patch
		if decodeJSON(r, &input) != nil {
			writeStorylineProblem(w, storyline.ErrInvalid)
			return
		}
		result, err = h.store.Update(r.Context(), r.PathValue("id"), input)
	default:
		var items []storyline.Storyline
		items, err = h.store.List(r.Context(), true)
		result = map[string]any{"items": items}
	}
	if err != nil {
		writeStorylineProblem(w, err)
		return
	}
	writeJSON(w, code, result)
}
func writeStorylineProblem(w http.ResponseWriter, err error) {
	status, title, detail := http.StatusInternalServerError, "Storyline request failed", "Could not complete the storyline request."
	switch {
	case errors.Is(err, storyline.ErrInvalid):
		status, title, detail = http.StatusBadRequest, "Invalid request", err.Error()
	case errors.Is(err, storyline.ErrNotFound):
		status, title, detail = http.StatusNotFound, "Not found", err.Error()
	case errors.Is(err, storyline.ErrConflict):
		status, title, detail = http.StatusConflict, "Membership conflict", err.Error()
	}
	writeProblem(w, status, "https://snap.local/problems/storyline", title, detail)
}
