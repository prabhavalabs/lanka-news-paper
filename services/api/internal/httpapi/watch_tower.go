package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/watchtower"
)

type watchTowerHandler struct {
	service *watchtower.Service
}

func (handler watchTowerHandler) threads(w http.ResponseWriter, request *http.Request) {
	if handler.service == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/unavailable", "Watch Tower unavailable", "The newsroom agent is not configured.")
		return
	}
	user := currentUser(request)
	switch request.Method {
	case http.MethodGet:
		threads, err := handler.service.ListThreads(request.Context(), user.ID)
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Could not load conversations", "Watch Tower could not load conversation history.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": threads})
	case http.MethodPost:
		var body struct {
			Title string `json:"title"`
		}
		if err := decodeJSON(request, &body); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "A conversation title is required.")
			return
		}
		thread, err := handler.service.CreateThread(request.Context(), user.ID, body.Title)
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Could not create conversation", "Watch Tower could not start a conversation.")
			return
		}
		writeJSON(w, http.StatusCreated, thread)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (handler watchTowerHandler) thread(w http.ResponseWriter, request *http.Request) {
	if handler.service == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/unavailable", "Watch Tower unavailable", "The newsroom agent is not configured.")
		return
	}
	threadID, ok := watchTowerThreadID(w, request)
	if !ok {
		return
	}
	userID := currentUser(request).ID
	switch request.Method {
	case http.MethodGet:
		conversation, err := handler.service.Conversation(request.Context(), userID, threadID)
		if err != nil {
			handler.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, conversation)
	case http.MethodDelete:
		if err := handler.service.DeleteThread(request.Context(), userID, threadID); err != nil {
			handler.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (handler watchTowerHandler) messages(w http.ResponseWriter, request *http.Request) {
	if handler.service == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/unavailable", "Watch Tower unavailable", "The newsroom agent is not configured.")
		return
	}
	threadID, ok := watchTowerThreadID(w, request)
	if !ok {
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "A question is required.")
		return
	}
	exchange, err := handler.service.Ask(request.Context(), currentUser(request).ID, threadID, body.Content)
	if err != nil {
		handler.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, exchange)
}

func (handler watchTowerHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, watchtower.ErrNotFound):
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Conversation not found", "This Watch Tower conversation does not exist.")
	case errors.Is(err, watchtower.ErrInvalidQuestion):
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid question", "Enter a question up to 4,000 characters.")
	default:
		slog.Error("watch tower request failed", "error", err)
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Watch Tower could not answer", "The newsroom agent could not complete this question. Try again.")
	}
}

func watchTowerThreadID(w http.ResponseWriter, request *http.Request) (uuid.UUID, bool) {
	value := strings.TrimSpace(request.PathValue("id"))
	threadID, err := uuid.Parse(value)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid conversation", "The conversation identifier is invalid.")
		return uuid.Nil, false
	}
	return threadID, true
}
