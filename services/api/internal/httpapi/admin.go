package httpapi

import (
	"net/http"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/desk"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/registry"
)

type adminHandler struct {
	registry *registry.Store
	poller   *ingest.Poller
	llm      *llm.Gateway
	desk     *desk.Store
}

func (handler adminHandler) sources(w http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		var body registry.Source
		if err := decodeJSON(request, &body); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
			return
		}
		item, err := handler.registry.CreateSource(request.Context(), body)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
			return
		}
		handler.registry.Audit(request.Context(), currentUser(request).Email, "create_source", item.ID, "ok")
		writeJSON(w, http.StatusCreated, item)
		return
	}
	items, err := handler.registry.ListSources(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not list sources.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler adminHandler) source(w http.ResponseWriter, request *http.Request) {
	item, err := handler.registry.GetSource(request.Context(), request.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", "Source was not found.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler adminHandler) setActive(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Active bool `json:"active"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	if err := handler.registry.SetActive(request.Context(), request.PathValue("id"), body.Active); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) endpoints(w http.ResponseWriter, request *http.Request) {
	sourceID := request.PathValue("id")
	if request.Method == http.MethodPost {
		var body registry.Endpoint
		if err := decodeJSON(request, &body); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
			return
		}
		body.SourceID = sourceID
		item, err := handler.registry.CreateEndpoint(request.Context(), body)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
		return
	}
	items, err := handler.registry.ListEndpoints(request.Context(), sourceID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not list endpoints.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler adminHandler) rights(w http.ResponseWriter, request *http.Request) {
	sourceID := request.PathValue("id")
	if request.Method == http.MethodPost {
		var body registry.Rights
		if err := decodeJSON(request, &body); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
			return
		}
		body.SourceID = sourceID
		item, err := handler.registry.CreateRights(request.Context(), body)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)
		return
	}
	items, err := handler.registry.ListRights(request.Context(), sourceID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not list rights.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler adminHandler) pause(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Paused bool `json:"paused"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	if err := handler.registry.SetPaused(request.Context(), request.PathValue("endpointId"), body.Paused); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) test(w http.ResponseWriter, request *http.Request) {
	result, err := handler.registry.TestEndpoint(request.Context(), request.PathValue("endpointId"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (handler adminHandler) runNow(w http.ResponseWriter, request *http.Request) {
	if err := handler.poller.PollEndpoint(request.Context(), request.PathValue("endpointId")); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) providers(w http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		var body struct {
			llm.Provider
			KeyRef string `json:"api_key_ref"`
		}
		if err := decodeJSON(request, &body); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
			return
		}
		if err := handler.llm.UpsertProvider(request.Context(), body.Provider, body.KeyRef); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	items, err := handler.llm.ListProviders(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not list providers.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler adminHandler) overview(w http.ResponseWriter, request *http.Request) {
	item, err := handler.desk.Overview(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler adminHandler) queue(w http.ResponseWriter, request *http.Request) {
	items, err := handler.desk.Queue(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler adminHandler) quarantine(w http.ResponseWriter, request *http.Request) {
	items, err := handler.desk.Quarantine(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler adminHandler) articleStatus(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := decodeJSON(request, &body); err != nil || body.Status == "" {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "Status is required.")
		return
	}
	if err := handler.desk.SetStatus(request.Context(), request.PathValue("id"), body.Status, currentUser(request).Email, body.Reason); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) articleCategory(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Slug string `json:"slug"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	if err := handler.desk.SetCategory(request.Context(), request.PathValue("id"), body.Slug); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) articleNote(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Note string `json:"note"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	if err := handler.desk.SetNote(request.Context(), request.PathValue("id"), body.Note); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) complaints(w http.ResponseWriter, request *http.Request) {
	items, err := handler.desk.Complaints(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler adminHandler) resolveComplaint(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Status     string `json:"status"`
		Resolution string `json:"resolution"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	if err := handler.desk.ResolveComplaint(request.Context(), request.PathValue("id"), body.Status, body.Resolution); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) holdCategory(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Held bool `json:"held"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	if err := handler.desk.HoldCategory(request.Context(), request.PathValue("slug"), body.Held); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) lockEvent(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Locked bool `json:"locked"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	if err := handler.desk.LockCluster(request.Context(), request.PathValue("id"), body.Locked); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) archive(w http.ResponseWriter, request *http.Request) {
	if err := handler.registry.Archive(request.Context(), request.PathValue("id")); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) updateSource(w http.ResponseWriter, request *http.Request) {
	var body registry.Source
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	body.ID = request.PathValue("id")
	if err := handler.registry.UpdateSource(request.Context(), body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) updateEndpoint(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Interval int  `json:"polling_interval_seconds"`
		Verified bool `json:"verified_official"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	if err := handler.registry.UpdateEndpoint(request.Context(), request.PathValue("endpointId"), body.Interval, body.Verified); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler adminHandler) profiles(w http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		var body llm.TaskProfile
		if err := decodeJSON(request, &body); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
			return
		}
		if err := handler.llm.UpsertProfile(request.Context(), body); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	items, err := handler.llm.ListProfiles(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
