package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/desk"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/pagination"
	"github.com/stretchr/testify/require"
)

type monitorServiceStub struct {
	changes chan struct{}
	calls   int
	query   queueMonitorQuery
	stable  bool
}

func (stub *monitorServiceStub) Snapshot(_ context.Context, query queueMonitorQuery) (monitorSnapshot, error) {
	stub.calls++
	stub.query = query
	total := stub.calls
	if stub.stable {
		total = 1
	}
	return monitorSnapshot{
		Queue: desk.QueueMonitor{
			Pagination: pagination.Meta{Page: query.Params.Page, PerPage: query.Params.PerPage, TotalPages: 1},
			Summary:    desk.QueueSummary{Total: total},
		},
		Cron: desk.CronMonitor{Summary: desk.CronMonitorSummary{Total: 4}},
	}, nil
}

func (stub *monitorServiceStub) Subscribe() (<-chan struct{}, func()) {
	return stub.changes, func() {}
}

func TestMonitorStreamSendsInitialSnapshot(t *testing.T) {
	changes := make(chan struct{})
	close(changes)
	service := &monitorServiceStub{changes: changes}
	handler := adminHandler{monitor: service}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/jobs/stream?page=2&per_page=25&status=completed&queue=analysis&kind=article.pipeline&window=24h", nil)

	handler.monitorStream(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	require.Equal(t, "no-cache, no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "no", recorder.Header().Get("X-Accel-Buffering"))
	require.Contains(t, recorder.Body.String(), "retry: 3000")
	require.Contains(t, recorder.Body.String(), "event: monitor")
	require.Contains(t, recorder.Body.String(), `"queue":{"items":null,"pagination":{"page":2,"per_page":25`)
	require.Equal(t, 2, service.query.Params.Page)
	require.Equal(t, 25, service.query.Params.PerPage)
	require.Equal(t, "completed", service.query.Status)
	require.Equal(t, "analysis", service.query.Queue)
	require.Equal(t, "article.pipeline", service.query.Kind)
}

func TestMonitorStreamPushesChangedSnapshot(t *testing.T) {
	changes := make(chan struct{}, 1)
	changes <- struct{}{}
	close(changes)
	service := &monitorServiceStub{changes: changes}
	handler := adminHandler{monitor: service}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/jobs/stream", nil)

	handler.monitorStream(recorder, request)

	require.Equal(t, 2, service.calls)
	require.Equal(t, 2, strings.Count(recorder.Body.String(), "event: monitor"))
	require.Contains(t, recorder.Body.String(), `"total":2`)
}

func TestMonitorStreamSkipsUnchangedSnapshot(t *testing.T) {
	changes := make(chan struct{}, 1)
	changes <- struct{}{}
	close(changes)
	service := &monitorServiceStub{changes: changes, stable: true}
	handler := adminHandler{monitor: service}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/jobs/stream", nil)

	handler.monitorStream(recorder, request)

	require.Equal(t, 2, service.calls)
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "event: monitor"))
}

func TestMonitorFingerprintIgnoresHeartbeatTimestamps(t *testing.T) {
	now := time.Now().UTC()
	first := monitorSnapshot{Cron: desk.CronMonitor{CheckedAt: now, Worker: desk.CronWorkerStatus{Status: "online", LeaseExpiresAt: &now}}}
	later := now.Add(time.Minute)
	second := monitorSnapshot{Cron: desk.CronMonitor{CheckedAt: later, Worker: desk.CronWorkerStatus{Status: "online", LeaseExpiresAt: &later}}}

	firstFingerprint, firstErr := monitorFingerprint(first)
	secondFingerprint, secondErr := monitorFingerprint(second)

	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	require.Equal(t, firstFingerprint, secondFingerprint)
}

func TestMonitorStreamRejectsInvalidFilter(t *testing.T) {
	service := &monitorServiceStub{changes: make(chan struct{})}
	handler := adminHandler{monitor: service}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/admin/jobs/stream?status=broken", nil)

	handler.monitorStream(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, 0, service.calls)
}

func TestQueueMonitorAcceptsAdministrativeAnalysisFilters(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/admin/jobs?queue=admin-analysis&kind=admin.article.analysis", nil)

	query, err := parseQueueMonitorQuery(request)

	require.NoError(t, err)
	require.Equal(t, "admin-analysis", query.Queue)
	require.Equal(t, "admin.article.analysis", query.Kind)
}
