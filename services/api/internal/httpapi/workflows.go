package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
)

type workflowAdminHandler struct {
	gateway *llm.Gateway
}

func (handler workflowAdminHandler) workflows(w http.ResponseWriter, request *http.Request) {
	if !requireAdministrator(w, request) {
		return
	}
	items, err := handler.gateway.ListWorkflows(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not load agent workflows.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler workflowAdminHandler) workflow(w http.ResponseWriter, request *http.Request) {
	if !requireAdministrator(w, request) {
		return
	}
	var input llm.WorkflowInput
	if err := decodeJSON(request, &input); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	item, err := handler.gateway.UpdateWorkflow(request.Context(), request.PathValue("task"), input, currentUser(request).ID)
	switch {
	case errors.Is(err, llm.ErrWorkflowMissing):
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", err.Error())
	case errors.Is(err, llm.ErrWorkflowInvalid):
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
	case err != nil:
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not update the agent workflow.")
	default:
		writeJSON(w, http.StatusOK, item)
	}
}

func (handler workflowAdminHandler) feedback(w http.ResponseWriter, request *http.Request) {
	if !requireAdministrator(w, request) {
		return
	}
	if request.Method == http.MethodGet {
		items, err := handler.gateway.ListWorkflowFeedback(request.Context())
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not load agent feedback.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	var input llm.WorkflowFeedbackInput
	if err := decodeJSON(request, &input); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	item, err := handler.gateway.CreateWorkflowFeedback(request.Context(), input, currentUser(request).ID)
	if errors.Is(err, llm.ErrWorkflowMissing) || errors.Is(err, llm.ErrWorkflowInvalid) {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not save agent feedback.")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (handler workflowAdminHandler) reviewFeedback(w http.ResponseWriter, request *http.Request) {
	if !requireAdministrator(w, request) {
		return
	}
	id, err := uuid.Parse(request.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "Feedback id must be a UUID.")
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(request, &input); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	item, err := handler.gateway.ReviewWorkflowFeedback(request.Context(), id, input.Status, currentUser(request).ID)
	if errors.Is(err, llm.ErrWorkflowFeedbackMissing) {
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", err.Error())
		return
	}
	if errors.Is(err, llm.ErrWorkflowInvalid) {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not review agent feedback.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}
