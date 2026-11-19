package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/reels"
)

// reelsHandler serves Facebook Reel drafts for review. Building and rendering happen on the worker.
type reelsHandler struct {
	store   *reels.Store
	media   *media.Store
	enqueue func(ctx context.Context, reelID string, rerender bool) error
}

func (handler reelsHandler) ready(w http.ResponseWriter, request *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if !requireAdministrator(w, request) {
		return false
	}
	if handler.store == nil || handler.enqueue == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/reels", "Reels unavailable", "Reel drafts are not configured on this server.")
		return false
	}
	return true
}

func (handler reelsHandler) list(w http.ResponseWriter, request *http.Request) {
	if !handler.ready(w, request) {
		return
	}
	items, err := handler.store.List(request.Context(), 40)
	if err != nil {
		writeReelProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler reelsHandler) create(w http.ResponseWriter, request *http.Request) {
	if !handler.ready(w, request) {
		return
	}
	var input struct {
		StorylineID string `json:"storyline_id"`
	}
	if err := decodeJSON(request, &input); err != nil {
		writeReelProblem(w, reels.ErrInvalid)
		return
	}
	if input.StorylineID != "" {
		if _, err := uuid.Parse(input.StorylineID); err != nil {
			writeReelProblem(w, reels.ErrInvalid)
			return
		}
	}
	handler.queueNew(w, request, input.StorylineID)
}

func (handler reelsHandler) queueNew(w http.ResponseWriter, request *http.Request, storylineID string) {
	actor := currentUser(request).ID.String()
	id, err := handler.store.Create(request.Context(), "manual", storylineID, &actor, reels.Trend{GoogleTrends: []string{}})
	if err == nil {
		err = handler.enqueue(request.Context(), id, false)
	}
	if err != nil {
		writeReelProblem(w, err)
		return
	}
	reel, err := handler.store.Get(request.Context(), id)
	if err != nil {
		writeReelProblem(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, reel)
}

func (handler reelsHandler) get(w http.ResponseWriter, request *http.Request) {
	if !handler.ready(w, request) {
		return
	}
	reel, err := handler.store.Get(request.Context(), request.PathValue("id"))
	if err != nil {
		writeReelProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reel)
}

func (handler reelsHandler) edit(w http.ResponseWriter, request *http.Request) {
	if !handler.ready(w, request) {
		return
	}
	var input reels.Edit
	if err := decodeJSON(request, &input); err != nil {
		writeReelProblem(w, reels.ErrInvalid)
		return
	}
	reel, rerender, err := handler.store.Edit(request.Context(), request.PathValue("id"), input)
	if err == nil && rerender {
		err = handler.enqueue(request.Context(), reel.ID, true)
	}
	if err != nil {
		writeReelProblem(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reel)
}

func (handler reelsHandler) action(w http.ResponseWriter, request *http.Request) {
	if !handler.ready(w, request) {
		return
	}
	var input struct{}
	if err := decodeJSON(request, &input); err != nil {
		writeReelProblem(w, reels.ErrInvalid)
		return
	}
	id := request.PathValue("id")
	actor := currentUser(request).ID.String()
	switch action := request.PathValue("action"); action {
	case "approve", "reject", "posted", "reopen":
		reel, err := handler.store.Review(request.Context(), id, action, &actor)
		if err != nil {
			writeReelProblem(w, err)
			return
		}
		writeJSON(w, http.StatusOK, reel)
	case "rerender":
		err := handler.store.Queue(request.Context(), id)
		if err == nil {
			err = handler.enqueue(request.Context(), id, true)
		}
		if err != nil {
			writeReelProblem(w, err)
			return
		}
		reel, err := handler.store.Get(request.Context(), id)
		if err != nil {
			writeReelProblem(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, reel)
	case "regenerate":
		// A fresh draft for the same storyline: new script, images and music.
		reel, err := handler.store.Get(request.Context(), id)
		if err != nil {
			writeReelProblem(w, err)
			return
		}
		storylineID := ""
		if reel.StorylineID != nil {
			storylineID = *reel.StorylineID
		}
		handler.queueNew(w, request, storylineID)
	default:
		writeReelProblem(w, reels.ErrInvalid)
	}
}

// file streams a draft's video or cover; videos support range requests so the player can seek.
func (handler reelsHandler) file(w http.ResponseWriter, request *http.Request) {
	if !requireAdministrator(w, request) {
		return
	}
	if handler.store == nil || handler.media == nil {
		http.NotFound(w, request)
		return
	}
	reel, err := handler.store.Get(request.Context(), request.PathValue("id"))
	if err != nil {
		http.NotFound(w, request)
		return
	}
	key, name, contentType := reel.VideoKey, "thulawa-reel-"+reel.CreatedAt.Format("2006-01-02")+".mp4", "video/mp4"
	if request.PathValue("kind") == "cover" {
		key, name, contentType = reel.CoverKey, "thulawa-reel-"+reel.CreatedAt.Format("2006-01-02")+"-cover.jpg", "image/jpeg"
	}
	if key == nil || *key == "" {
		http.NotFound(w, request)
		return
	}
	body, _, err := handler.media.Open(request.Context(), *key)
	if err != nil {
		http.NotFound(w, request)
		return
	}
	data, err := io.ReadAll(io.LimitReader(body, 200<<20))
	body.Close()
	if err != nil {
		slog.WarnContext(request.Context(), "read reel file", "error", err)
		http.Error(w, "could not read the file", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-cache")
	if request.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	}
	modified := reel.UpdatedAt
	if reel.RenderedAt != nil {
		modified = *reel.RenderedAt
	}
	http.ServeContent(w, request, name, modified.Truncate(time.Second), bytes.NewReader(data))
}

func writeReelProblem(w http.ResponseWriter, err error) {
	status, title, detail := http.StatusInternalServerError, "Reel request failed", "Could not complete the reel request."
	switch {
	case errors.Is(err, reels.ErrInvalid):
		status, title, detail = http.StatusBadRequest, "Invalid request", err.Error()
	case errors.Is(err, reels.ErrNotFound):
		status, title, detail = http.StatusNotFound, "Not found", "The reel draft does not exist."
	case errors.Is(err, reels.ErrConflict):
		status, title, detail = http.StatusConflict, "Reel is busy", "The reel draft cannot change while it is being built, or from its current status."
	default:
		slog.Error("reel request failed", "error", err)
	}
	writeProblem(w, status, "https://snap.local/problems/reels", title, detail)
}
