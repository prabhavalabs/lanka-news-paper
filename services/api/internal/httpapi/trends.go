package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/trends"
)

// trendsHandler shows the latest trend radar and starts a fresh pass on the worker.
type trendsHandler struct {
	store   *trends.Store
	enqueue func(context.Context) error
}

func (handler trendsHandler) radar(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !requireAdministrator(w, request) {
		return
	}
	if handler.store == nil || handler.enqueue == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/trends", "Trend radar unavailable", "The trend radar is not configured on this server.")
		return
	}
	if request.Method == http.MethodPost {
		var input struct{}
		if err := decodeJSON(request, &input); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/trends", "Invalid request", "Send an empty JSON object.")
			return
		}
		if err := handler.enqueue(request.Context()); err != nil {
			slog.ErrorContext(request.Context(), "enqueue trend radar", "error", err)
			writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/trends", "Could not start the radar", "The radar refresh could not be queued.")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
		return
	}
	run, err := handler.store.Latest(request.Context())
	if errors.Is(err, trends.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"run": nil})
		return
	}
	if err != nil {
		slog.ErrorContext(request.Context(), "load trend radar", "error", err)
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/trends", "Could not load the radar", "The trend radar could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run})
}
