package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/desk"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/ingest"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/registry"
)

const maxSourceLogoBytes = 768 << 10

type adminHandler struct {
	registry           *registry.Store
	poller             *ingest.Poller
	llm                *llm.Gateway
	desk               *desk.Store
	monitor            monitorService
	media              *media.Store
	runPipeline        func(context.Context, string, string) error
	runContentBackfill func(context.Context) error
}

func (handler adminHandler) sourceLogo(w http.ResponseWriter, request *http.Request) {
	source, err := handler.registry.GetSource(request.Context(), request.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", "Source was not found.")
		return
	}

	if request.Method == http.MethodDelete {
		if err := handler.registry.SetSourceIconURL(request.Context(), source.ID, ""); err != nil {
			writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not remove the source logo.")
			return
		}
		handler.deleteManagedMedia(request, source.IconURL)
		handler.registry.Audit(request.Context(), currentUser(request).Email, "remove_source_logo", source.ID, "ok")
		writeJSON(w, http.StatusOK, map[string]string{"icon_url": ""})
		return
	}

	request.Body = http.MaxBytesReader(w, request.Body, maxSourceLogoBytes+(64<<10))
	if err := request.ParseMultipartForm(maxSourceLogoBytes); err != nil {
		writeProblem(w, http.StatusRequestEntityTooLarge, "https://snap.local/problems/invalid", "Image too large", "Choose a PNG or JPEG smaller than 768 KB.")
		return
	}
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}
	file, _, err := request.FormFile("file")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "A logo file is required.")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSourceLogoBytes+1))
	if err != nil || len(data) > maxSourceLogoBytes {
		writeProblem(w, http.StatusRequestEntityTooLarge, "https://snap.local/problems/invalid", "Image too large", "Choose a PNG or JPEG smaller than 768 KB.")
		return
	}
	contentType, extension, err := validateSourceLogo(data)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid image", err.Error())
		return
	}

	key := fmt.Sprintf("source-logos/%s/%s.%s", source.ID, uuid.NewString(), extension)
	if err := handler.media.Put(request.Context(), key, contentType, data); err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Upload failed", "Could not store the source logo.")
		return
	}
	iconURL := media.URL(key)
	if err := handler.registry.SetSourceIconURL(request.Context(), source.ID, iconURL); err != nil {
		_ = handler.media.Delete(request.Context(), key)
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Upload failed", "Could not attach the source logo.")
		return
	}
	handler.deleteManagedMedia(request, source.IconURL)
	handler.registry.Audit(request.Context(), currentUser(request).Email, "update_source_logo", source.ID, "ok")
	writeJSON(w, http.StatusOK, map[string]string{"icon_url": iconURL})
}

func (handler adminHandler) mediaFile(w http.ResponseWriter, request *http.Request) {
	body, contentType, err := handler.media.Open(request.Context(), request.PathValue("key"))
	if err != nil {
		http.NotFound(w, request)
		return
	}
	defer body.Close()
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	if _, err := io.Copy(w, body); err != nil {
		slog.WarnContext(request.Context(), "stream media", "error", err)
	}
}

func (handler adminHandler) deleteManagedMedia(request *http.Request, value string) {
	key, managed := media.KeyFromURL(value)
	if managed {
		if err := handler.media.Delete(request.Context(), key); err != nil {
			slog.WarnContext(request.Context(), "delete replaced media", "key", key, "error", err)
		}
	}
}

func validateSourceLogo(data []byte) (string, string, error) {
	contentType := http.DetectContentType(data)
	extensions := map[string]string{"image/jpeg": "jpg", "image/png": "png"}
	extension, ok := extensions[contentType]
	if !ok {
		return "", "", fmt.Errorf("only PNG and JPEG logos are supported")
	}
	configuration, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", "", fmt.Errorf("the uploaded file is not a valid image")
	}
	if configuration.Width < 64 || configuration.Height < 64 {
		return "", "", fmt.Errorf("logo dimensions must be at least 64 × 64 pixels")
	}
	if configuration.Width > 4096 || configuration.Height > 4096 {
		return "", "", fmt.Errorf("logo dimensions cannot exceed 4096 × 4096 pixels")
	}
	return contentType, extension, nil
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
	params, err := parsePagination(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	sourceType, err := parseFilter(request, "type", "private_media", "state_owned", "government", "independent", "international", "other")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	status, err := parseFilter(request, "status", "active", "held")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	items, total, err := handler.registry.ListSources(request.Context(), params, sourceType, status)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not list sources.")
		return
	}
	writePage(w, items, total, params)
}

func (handler adminHandler) source(w http.ResponseWriter, request *http.Request) {
	item, err := handler.registry.GetSource(request.Context(), request.PathValue("id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", "Source was not found.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler adminHandler) sourcePerformance(w http.ResponseWriter, request *http.Request) {
	days := 30
	if value := request.URL.Query().Get("days"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || (parsed != 7 && parsed != 30 && parsed != 90) {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "days must be 7, 30, or 90.")
			return
		}
		days = parsed
	}
	item, err := handler.registry.GetSourcePerformance(request.Context(), request.PathValue("id"), days)
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
	params, err := parsePagination(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	health, err := parseFilter(request, "health", "unknown", "healthy", "stale", "failed", "auth_denied")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	status, err := parseFilter(request, "status", "active", "paused")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	items, total, err := handler.registry.ListEndpoints(request.Context(), sourceID, params, health, status)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not list endpoints.")
		return
	}
	writePage(w, items, total, params)
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
	params, err := parsePagination(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	mode, err := parseFilter(request, "mode", "discovery_only", "licensed_excerpt", "licensed_media", "full_syndication", "internal_verification", "disabled")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	items, total, err := handler.registry.ListRights(request.Context(), sourceID, params, mode)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not list rights.")
		return
	}
	writePage(w, items, total, params)
}

func (handler adminHandler) collection(w http.ResponseWriter, request *http.Request) {
	sourceID := request.PathValue("id")
	if request.Method == http.MethodPost {
		var body registry.CollectionProfile
		if err := decodeJSON(request, &body); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
			return
		}
		item, err := handler.registry.SaveCollectionProfile(request.Context(), sourceID, currentUser(request).Email, body)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid collection profile", err.Error())
			return
		}
		handler.registry.Audit(request.Context(), currentUser(request).Email, "update_source_collection", sourceID, "ok")
		if item.ArticleMethod == "html_static" && handler.runContentBackfill != nil {
			if err := handler.runContentBackfill(request.Context()); err != nil {
				slog.WarnContext(request.Context(), "queue article content backfill after collection update", "source", sourceID, "error", err)
			}
		}
		writeJSON(w, http.StatusCreated, item)
		return
	}
	items, err := handler.registry.ListCollectionProfiles(request.Context(), sourceID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not load collection profiles.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler adminHandler) compliance(w http.ResponseWriter, request *http.Request) {
	sourceID := request.PathValue("id")
	if request.Method == http.MethodPost {
		var body registry.ComplianceReview
		if err := decodeJSON(request, &body); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
			return
		}
		item, err := handler.registry.SaveComplianceReview(request.Context(), sourceID, currentUser(request).Email, body)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid compliance review", err.Error())
			return
		}
		handler.registry.Audit(request.Context(), currentUser(request).Email, "update_source_compliance", sourceID, "ok")
		if item.AllowFullTextStorage && handler.runContentBackfill != nil {
			if err := handler.runContentBackfill(request.Context()); err != nil {
				slog.WarnContext(request.Context(), "queue article content backfill after compliance update", "source", sourceID, "error", err)
			}
		}
		writeJSON(w, http.StatusCreated, item)
		return
	}
	item, err := handler.registry.GetComplianceReview(request.Context(), sourceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", "No compliance review is configured.")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not load the compliance review.")
		return
	}
	writeJSON(w, http.StatusOK, item)
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
	item, err := handler.llm.ProviderStatus(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Could not check OpenRouter.")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler adminHandler) models(w http.ResponseWriter, request *http.Request) {
	items, err := handler.llm.ListModels(request.Context())
	if err != nil {
		writeProblem(w, http.StatusBadGateway, "https://snap.local/problems/upstream", "OpenRouter unavailable", "Could not load the OpenRouter model catalog.")
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

func (handler adminHandler) trends(w http.ResponseWriter, request *http.Request) {
	days, err := parseTrendDays(request.URL.Query().Get("days"))
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	items, err := handler.desk.Trends(request.Context(), days)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler adminHandler) knowledgeGraph(w http.ResponseWriter, request *http.Request) {
	start, end, err := parseKnowledgeWindow(
		request.URL.Query().Get("days"),
		request.URL.Query().Get("from"),
		request.URL.Query().Get("to"),
		time.Now(),
	)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	category := request.URL.Query().Get("category")
	if len(category) > 50 {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "category is too long")
		return
	}
	item, err := handler.desk.KnowledgeGraph(request.Context(), start, end, category)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func parseKnowledgeWindow(daysValue, fromValue, toValue string, now time.Time) (time.Time, time.Time, error) {
	if fromValue == "" && toValue == "" {
		days, err := parseKnowledgeDays(daysValue)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		end := now.UTC()
		return end.Add(-time.Duration(days) * 24 * time.Hour), end, nil
	}
	if fromValue == "" || toValue == "" {
		return time.Time{}, time.Time{}, errors.New("from and to dates must be provided together")
	}
	start, err := time.Parse(time.DateOnly, fromValue)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("from must use YYYY-MM-DD")
	}
	lastDay, err := time.Parse(time.DateOnly, toValue)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("to must use YYYY-MM-DD")
	}
	if lastDay.Before(start) {
		return time.Time{}, time.Time{}, errors.New("to must be on or after from")
	}
	end := lastDay.AddDate(0, 0, 1)
	if end.Sub(start) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, errors.New("date range cannot exceed 366 days")
	}
	return start, end, nil
}

func parseKnowledgeDays(value string) (int, error) {
	if value == "" {
		return 1, nil
	}
	days, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New("days must be 1, 7, or 30")
	}
	switch days {
	case 1, 7, 30:
		return days, nil
	default:
		return 0, errors.New("days must be 1, 7, or 30")
	}
}

func parseTrendDays(value string) (int, error) {
	if value == "" {
		return 90, nil
	}
	days, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New("days must be 7, 30, or 90")
	}
	switch days {
	case 7, 30, 90:
		return days, nil
	default:
		return 0, errors.New("days must be 7, 30, or 90")
	}
}

func (handler adminHandler) queue(w http.ResponseWriter, request *http.Request) {
	params, err := parsePagination(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	status, err := parseFilter(request, "status", "held", "quarantined", "low_confidence")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	items, total, err := handler.desk.Queue(request.Context(), params, status)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writePage(w, items, total, params)
}

func (handler adminHandler) jobs(w http.ResponseWriter, request *http.Request) {
	query, err := parseQueueMonitorQuery(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	since := time.Now().UTC().Add(-query.Window)
	result, err := handler.desk.QueueJobs(request.Context(), query.Params, query.Status, query.Queue, query.Kind, &since)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (handler adminHandler) jobArtifacts(w http.ResponseWriter, request *http.Request) {
	result, err := handler.desk.QueueJobArtifacts(request.Context(), request.PathValue("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", "Queue job was not found.")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (handler adminHandler) cronJobs(w http.ResponseWriter, request *http.Request) {
	result, err := handler.desk.CronJobs(request.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (handler adminHandler) articles(w http.ResponseWriter, request *http.Request) {
	params, err := parsePagination(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	status, err := parseFilter(request, "status", "held", "published", "unpublished", "removed", "quarantined")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	pipelineStatus, err := parseFilter(request, "pipeline", "not_started", "queued", "running", "succeeded", "failed")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	items, total, err := handler.desk.Articles(request.Context(), params, status, pipelineStatus)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writePage(w, items, total, params)
}

func (handler adminHandler) article(w http.ResponseWriter, request *http.Request) {
	item, err := handler.desk.Article(request.Context(), request.PathValue("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeProblem(w, http.StatusNotFound, "https://snap.local/problems/not-found", "Not found", "Article was not found.")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (handler adminHandler) runArticlePipeline(w http.ResponseWriter, request *http.Request) {
	var body struct {
		Step string `json:"step"`
	}
	if err := decodeJSON(request, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
		return
	}
	if handler.runPipeline == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/unavailable", "Pipeline unavailable", "The pipeline producer is not available.")
		return
	}
	if err := handler.runPipeline(request.Context(), request.PathValue("id"), body.Step); err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Could not run pipeline", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
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
	params, err := parsePagination(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	status, err := parseFilter(request, "status", "open", "in_review", "resolved", "rejected")
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	items, total, err := handler.desk.Complaints(request.Context(), params, status)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	writePage(w, items, total, params)
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
		var body struct {
			Task  string `json:"task"`
			Model string `json:"model"`
		}
		if err := decodeJSON(request, &body); err != nil {
			writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", "JSON body is required.")
			return
		}
		if err := handler.llm.UpdateProfile(request.Context(), body.Task, body.Model); err != nil {
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
