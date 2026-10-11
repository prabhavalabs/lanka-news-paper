package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/retention"
)

type logRetentionHandler struct {
	store *retention.Store
	run   func(context.Context) error
}

func (handler logRetentionHandler) settings(w http.ResponseWriter, request *http.Request) {
	if !handler.ready(w, request) {
		return
	}
	if request.Method == http.MethodGet {
		settings, err := handler.store.Settings(request.Context())
		if err != nil {
			handler.internal(w)
			return
		}
		writeJSON(w, http.StatusOK, settings)
		return
	}
	var body struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "Retention days are required.")
		return
	}
	settings, err := handler.store.UpdateSettings(request.Context(), currentUser(request).ID, body.RetentionDays)
	if errors.Is(err, retention.ErrInvalid) {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid retention period", "Keep logs for 1 to 365 days.")
		return
	}
	if err != nil {
		handler.internal(w)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (handler logRetentionHandler) runNow(w http.ResponseWriter, request *http.Request) {
	if !handler.ready(w, request) {
		return
	}
	if handler.run == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/unavailable", "Cleanup unavailable", "The job queue is not configured.")
		return
	}
	if err := handler.run(request.Context()); err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Could not queue cleanup", "The log cleanup could not be queued.")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

func (handler logRetentionHandler) ready(w http.ResponseWriter, request *http.Request) bool {
	if handler.store == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/unavailable", "Log retention unavailable", "Log retention is not configured.")
		return false
	}
	return requireAdministrator(w, request)
}

func (handler logRetentionHandler) internal(w http.ResponseWriter) {
	writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Could not load log retention", "Log retention settings are temporarily unavailable.")
}
