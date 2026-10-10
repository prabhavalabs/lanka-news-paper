package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/social"
)

type socialHandler struct{ service *social.Service }

func (handler socialHandler) serve(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !requireAdministrator(w, request) {
		return
	}
	if handler.service == nil {
		writeSocialProblem(w, social.ErrNotConfigured)
		return
	}
	actor := currentUser(request).ID
	path := request.URL.Path
	var result any
	var err error
	code := http.StatusOK
	switch path {
	case "/api/admin/social/status":
		result, err = handler.service.Status(request.Context())
	case "/api/admin/social/settings":
		if request.Method == http.MethodGet {
			result, err = handler.service.Store().GetSettings(request.Context())
		} else {
			var input social.Settings
			if decodeJSON(request, &input) != nil {
				writeSocialProblem(w, social.ErrInvalid)
				return
			}
			result, err = handler.service.Store().UpdateSettings(request.Context(), input, actor)
		}
	case "/api/admin/social/facebook/connect":
		var input struct{}
		if decodeJSON(request, &input) != nil {
			writeSocialProblem(w, social.ErrInvalid)
			return
		}
		var authorize string
		authorize, err = handler.service.Connect(request.Context(), actor)
		result = map[string]string{"authorize_url": authorize}
	case "/api/admin/social/facebook/pending-pages":
		var pages []social.PendingPage
		pages, err = handler.service.PendingPages(request.Context(), request.URL.Query().Get("state_id"), actor)
		result = map[string]any{"items": pages}
	case "/api/admin/social/facebook/select":
		var input struct {
			StateID string `json:"state_id"`
			PageID  string `json:"page_id"`
		}
		if decodeJSON(request, &input) != nil {
			writeSocialProblem(w, social.ErrInvalid)
			return
		}
		err = handler.service.SelectPage(request.Context(), input.StateID, input.PageID, actor)
		result = map[string]bool{"ok": true}
	case "/api/admin/social/facebook/disconnect":
		var input struct{}
		if decodeJSON(request, &input) != nil {
			writeSocialProblem(w, social.ErrInvalid)
			return
		}
		err = handler.service.Disconnect(request.Context(), actor)
		result = map[string]bool{"ok": true}
	case "/api/admin/social/run-selection":
		var input struct{}
		if decodeJSON(request, &input) != nil {
			writeSocialProblem(w, social.ErrInvalid)
			return
		}
		err = handler.service.RunSelection(request.Context(), actor)
		result = map[string]bool{"ok": true}
		code = http.StatusAccepted
	case "/api/admin/social/posts":
		if request.Method == http.MethodGet {
			limit := 30
			if value := request.URL.Query().Get("limit"); value != "" {
				limit, err = strconv.Atoi(value)
				if err != nil {
					writeSocialProblem(w, social.ErrInvalid)
					return
				}
			}
			result, err = handler.service.Store().ListPosts(request.Context(), request.URL.Query().Get("status"), limit, request.URL.Query().Get("cursor"))
		} else {
			var input struct {
				EventID   string `json:"event_id"`
				ArticleID string `json:"article_id"`
			}
			if decodeJSON(request, &input) != nil {
				writeSocialProblem(w, social.ErrInvalid)
				return
			}
			request, cancel := inferenceRequest(w, request)
			defer cancel()
			result, err = handler.service.ManualPost(request.Context(), input.EventID, input.ArticleID, actor)
			code = http.StatusCreated
		}
	default:
		id := request.PathValue("id")
		action := request.PathValue("action")
		if request.Method == http.MethodGet {
			result, err = handler.service.Store().GetPost(request.Context(), id)
		} else if request.Method == http.MethodPatch {
			var input social.EditInput
			if decodeJSON(request, &input) != nil {
				writeSocialProblem(w, social.ErrInvalid)
				return
			}
			result, err = handler.service.Edit(request.Context(), id, input, actor)
		} else {
			switch action {
			case "render":
				var input struct{}
				if decodeJSON(request, &input) != nil {
					writeSocialProblem(w, social.ErrInvalid)
					return
				}
				result, err = handler.service.Render(request.Context(), id, actor)
			case "approve":
				var input struct {
					ScheduledFor *time.Time `json:"scheduled_for"`
				}
				if decodeJSON(request, &input) != nil {
					writeSocialProblem(w, social.ErrInvalid)
					return
				}
				result, err = handler.service.Transition(request.Context(), id, action, actor, input.ScheduledFor, false)
			case "retry":
				var input struct {
					Confirm bool `json:"confirm_not_published"`
				}
				if decodeJSON(request, &input) != nil {
					writeSocialProblem(w, social.ErrInvalid)
					return
				}
				result, err = handler.service.Transition(request.Context(), id, action, actor, nil, input.Confirm)
			case "reject":
				var input struct{}
				if decodeJSON(request, &input) != nil {
					writeSocialProblem(w, social.ErrInvalid)
					return
				}
				result, err = handler.service.Transition(request.Context(), id, action, actor, nil, false)
			default:
				writeSocialProblem(w, social.ErrNotFound)
				return
			}
		}
	}
	if err != nil {
		writeSocialProblem(w, err)
		return
	}
	writeJSON(w, code, result)
}

// The OAuth state authenticates this cross-site return; Strict session cookies are absent.
func (handler socialHandler) callback(w http.ResponseWriter, request *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if handler.service == nil {
		http.Redirect(w, request, "/social?error=not_configured", http.StatusSeeOther)
		return
	}
	request, cancel := inferenceRequest(w, request)
	defer cancel()
	outcome, stateID, callbackErr := handler.service.Callback(request.Context(), request.URL.Query().Get("code"), request.URL.Query().Get("state"), request.URL.Query().Get("error"))
	query := url.Values{}
	if callbackErr != nil {
		errorCode := "connection_failed"
		if errors.Is(callbackErr, social.ErrOAuthState) {
			errorCode = "invalid_state"
		} else if errors.Is(callbackErr, social.ErrNotConfigured) {
			errorCode = "not_configured"
		} else if errors.Is(callbackErr, social.ErrInvalid) {
			errorCode = "authorization_failed"
		}
		query.Set("error", errorCode)
	} else {
		query.Set("facebook", outcome)
		if stateID != "" {
			query.Set("state_id", stateID)
		}
	}
	http.Redirect(w, request, "/social?"+query.Encode(), http.StatusSeeOther)
	return
}

func writeSocialProblem(w http.ResponseWriter, err error) {
	status, title, detail := http.StatusInternalServerError, "Social publishing failed", "Could not complete the social publishing request."
	switch {
	case errors.Is(err, social.ErrInvalid):
		status, title, detail = http.StatusBadRequest, "Invalid request", err.Error()
	case errors.Is(err, social.ErrNotFound):
		status, title, detail = http.StatusNotFound, "Not found", err.Error()
	case errors.Is(err, social.ErrConflict):
		status, title, detail = http.StatusConflict, "Queue conflict", err.Error()
	case errors.Is(err, social.ErrOAuthState):
		status, title, detail = http.StatusBadRequest, "Connection expired", err.Error()
	case errors.Is(err, social.ErrNotConfigured), errors.Is(err, social.ErrModelUnavailable):
		status, title, detail = http.StatusServiceUnavailable, "Social publishing unavailable", err.Error()
	}
	writeProblem(w, status, "https://snap.local/problems/social", title, detail)
}
