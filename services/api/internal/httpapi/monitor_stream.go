package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/desk"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
)

type queueMonitorQuery struct {
	Params pagination.Params
	Status string
	Queue  string
	Kind   string
	Window time.Duration
}

type monitorSnapshot struct {
	Queue desk.QueueMonitor `json:"queue"`
	Cron  desk.CronMonitor  `json:"cron"`
}

type monitorService interface {
	Snapshot(context.Context, queueMonitorQuery) (monitorSnapshot, error)
	Subscribe() (<-chan struct{}, func())
}

type storeMonitorService struct {
	store  *desk.Store
	broker *desk.MonitorBroker
}

func newMonitorService(store *desk.Store, broker *desk.MonitorBroker) monitorService {
	if store == nil || broker == nil {
		return nil
	}
	return &storeMonitorService{store: store, broker: broker}
}

func (service *storeMonitorService) Snapshot(ctx context.Context, query queueMonitorQuery) (monitorSnapshot, error) {
	since := time.Now().UTC().Add(-query.Window)
	queue, err := service.store.QueueJobs(ctx, query.Params, query.Status, query.Queue, query.Kind, &since)
	if err != nil {
		return monitorSnapshot{}, err
	}
	cron, err := service.store.CronJobs(ctx)
	if err != nil {
		return monitorSnapshot{}, err
	}
	return monitorSnapshot{Queue: queue, Cron: cron}, nil
}

func (service *storeMonitorService) Subscribe() (<-chan struct{}, func()) {
	return service.broker.Subscribe()
}

func parseQueueMonitorQuery(request *http.Request) (queueMonitorQuery, error) {
	params, err := parsePagination(request)
	if err != nil {
		return queueMonitorQuery{}, err
	}
	status, err := parseFilter(request, "status", "queued", "processing", "completed", "partially_completed", "failed")
	if err != nil {
		return queueMonitorQuery{}, err
	}
	queue, err := parseFilter(request, "queue", "default", "analysis", "crawl", "admin-analysis-dispatch", "admin-analysis")
	if err != nil {
		return queueMonitorQuery{}, err
	}
	kind, err := parseFilter(request, "kind", "article.pipeline", "article.content", "article.content.backfill", "article.content.cleanup", "article.pipeline.dispatch", "admin.analysis.backfill.dispatch", "admin.article.analysis", "ingest.poll", "brief.daily", "intelligence.narration", "queue.history.cleanup")
	if err != nil {
		return queueMonitorQuery{}, err
	}
	window, err := parseFilter(request, "window", "24h", "7d")
	if err != nil {
		return queueMonitorQuery{}, err
	}
	duration := 7 * 24 * time.Hour
	if window == "24h" {
		duration = 24 * time.Hour
	}
	return queueMonitorQuery{Params: params, Status: status, Queue: queue, Kind: kind, Window: duration}, nil
}

func (handler adminHandler) monitorStream(w http.ResponseWriter, request *http.Request) {
	query, err := parseQueueMonitorQuery(request)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "https://snap.local/problems/invalid", "Invalid request", err.Error())
		return
	}
	if handler.monitor == nil {
		writeProblem(w, http.StatusServiceUnavailable, "https://snap.local/problems/unavailable", "Service unavailable", "Live queue telemetry is not configured.")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Streaming is unavailable.")
		return
	}

	changes, unsubscribe := handler.monitor.Subscribe()
	defer unsubscribe()
	snapshot, err := handler.monitor.Snapshot(request.Context(), query)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", err.Error())
		return
	}
	lastFingerprint, err := monitorFingerprint(snapshot)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Live telemetry could not be encoded.")
		return
	}

	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		writeProblem(w, http.StatusInternalServerError, "https://snap.local/problems/internal", "Internal server error", "Streaming could not be initialized.")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, "retry: 3000\n\n"); err != nil {
		return
	}
	if err := writeMonitorEvent(w, snapshot); err != nil {
		return
	}
	flusher.Flush()

	for {
		select {
		case <-request.Context().Done():
			return
		case _, open := <-changes:
			if !open {
				return
			}
			snapshot, err := handler.monitor.Snapshot(request.Context(), query)
			if err != nil {
				if request.Context().Err() != nil {
					return
				}
				slog.ErrorContext(request.Context(), "refresh live queue monitor", "error", err)
				if writeErr := writeMonitorError(w); writeErr == nil {
					flusher.Flush()
				}
				return
			}
			fingerprint, err := monitorFingerprint(snapshot)
			if err != nil {
				slog.ErrorContext(request.Context(), "fingerprint live queue monitor", "error", err)
				return
			}
			if fingerprint == lastFingerprint {
				continue
			}
			if err := writeMonitorEvent(w, snapshot); err != nil {
				return
			}
			lastFingerprint = fingerprint
			flusher.Flush()
		}
	}
}

func monitorFingerprint(snapshot monitorSnapshot) ([sha256.Size]byte, error) {
	snapshot.Cron.CheckedAt = time.Time{}
	snapshot.Cron.Worker.LeaseExpiresAt = nil
	for index := range snapshot.Queue.Items {
		if snapshot.Queue.Items[index].Status == "processing" {
			snapshot.Queue.Items[index].DurationMS = nil
		}
	}
	for index := range snapshot.Cron.Items {
		if snapshot.Cron.Items[index].CurrentlyRunning > 0 {
			snapshot.Cron.Items[index].LastDurationMS = nil
		}
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("encode monitor fingerprint: %w", err)
	}
	return sha256.Sum256(data), nil
}

func writeMonitorEvent(w http.ResponseWriter, snapshot monitorSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode monitor event: %w", err)
	}
	if _, err := fmt.Fprintf(w, "id: %d\nevent: monitor\ndata: %s\n\n", time.Now().UTC().UnixMilli(), data); err != nil {
		return fmt.Errorf("write monitor event: %w", err)
	}
	return nil
}

func writeMonitorError(w http.ResponseWriter) error {
	if _, err := fmt.Fprint(w, "event: monitor-error\ndata: {\"message\":\"Live telemetry will reconnect.\"}\n\n"); err != nil {
		return fmt.Errorf("write monitor error: %w", err)
	}
	return nil
}
