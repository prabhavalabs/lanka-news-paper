package desk

import (
	"context"
	"fmt"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/jobs"
)

const cronHistoryWindow = 24 * time.Hour

// CronJobStatistic combines configured cadence with recent River execution telemetry.
type CronJobStatistic struct {
	Kind               string     `json:"kind"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	Queue              string     `json:"queue"`
	IntervalSeconds    int64      `json:"interval_seconds"`
	RunOnStart         bool       `json:"run_on_start"`
	Health             string     `json:"health"`
	State              string     `json:"state"`
	CurrentlyRunning   int        `json:"currently_running"`
	LastJobID          *int64     `json:"last_job_id"`
	LastRunAt          *time.Time `json:"last_run_at"`
	LastFinishedAt     *time.Time `json:"last_finished_at"`
	NextRunAt          *time.Time `json:"next_run_at"`
	LastDurationMS     *int64     `json:"last_duration_ms"`
	AverageDurationMS  *int64     `json:"average_duration_ms"`
	Runs24Hours        int        `json:"runs_24h"`
	SuccessfulRuns24H  int        `json:"successful_runs_24h"`
	FailedRuns24Hours  int        `json:"failed_runs_24h"`
	SuccessRate24Hours *float64   `json:"success_rate_24h"`
	Attempt            int        `json:"attempt"`
	MaxAttempts        int        `json:"max_attempts"`
	WorkerID           *string    `json:"worker_id"`
	LastError          *string    `json:"last_error"`
}

// CronQueueWorker describes one configured River worker pool.
type CronQueueWorker struct {
	Name       string `json:"name"`
	MaxWorkers int    `json:"max_workers"`
	Paused     bool   `json:"paused"`
}

// CronWorkerStatus reports the elected scheduler and queue concurrency.
type CronWorkerStatus struct {
	Status         string            `json:"status"`
	LeaderID       *string           `json:"leader_id"`
	ElectedAt      *time.Time        `json:"elected_at"`
	LeaseExpiresAt *time.Time        `json:"lease_expires_at"`
	MaxConcurrency int               `json:"max_concurrency"`
	Queues         []CronQueueWorker `json:"queues"`
}

// CronMonitorSummary gives at-a-glance recurring job counts.
type CronMonitorSummary struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Healthy   int `json:"healthy"`
	Attention int `json:"attention"`
}

// CronMonitor is the cron monitoring API response.
type CronMonitor struct {
	Items     []CronJobStatistic `json:"items"`
	Summary   CronMonitorSummary `json:"summary"`
	Worker    CronWorkerStatus   `json:"worker"`
	CheckedAt time.Time          `json:"checked_at"`
}

type cronHistory struct {
	kind              string
	lastJobID         *int64
	state             *string
	createdAt         *time.Time
	attemptedAt       *time.Time
	finalizedAt       *time.Time
	lastDurationMS    *int64
	averageDurationMS *int64
	attempt           *int
	maxAttempts       *int
	workerID          *string
	lastError         *string
	runs24Hours       int
	successfulRuns24H int
	failedRuns24Hours int
	currentlyRunning  int
}

// CronJobs returns configured schedules joined with bounded River history statistics.
func (store *Store) CronJobs(ctx context.Context) (CronMonitor, error) {
	now := time.Now().UTC()
	catalog := jobs.PeriodicJobCatalog()
	kinds := make([]string, 0, len(catalog))
	for _, item := range catalog {
		kinds = append(kinds, item.Kind)
	}

	history, err := store.cronHistory(ctx, kinds)
	if err != nil {
		return CronMonitor{}, err
	}
	worker, err := store.cronWorker(ctx, now)
	if err != nil {
		return CronMonitor{}, err
	}

	result := CronMonitor{
		Items:     make([]CronJobStatistic, 0, len(catalog)),
		Summary:   CronMonitorSummary{Total: len(catalog)},
		Worker:    worker,
		CheckedAt: now,
	}
	for _, metadata := range catalog {
		recent := history[metadata.Kind]
		state := "not_started"
		if recent.state != nil {
			state = *recent.state
		}
		lastRunAt := recent.attemptedAt
		if lastRunAt == nil {
			lastRunAt = recent.createdAt
		}
		scheduleAnchor := cronScheduleAnchor(recent.createdAt, worker.ElectedAt, metadata.RunOnStart)
		var nextRunAt *time.Time
		if metadata.Schedule != nil {
			next := metadata.Schedule.Next(now)
			nextRunAt = &next
		} else if scheduleAnchor != nil {
			next := scheduleAnchor.Add(metadata.Interval)
			nextRunAt = &next
		}
		var successRate *float64
		terminalRuns := recent.successfulRuns24H + recent.failedRuns24Hours
		if terminalRuns > 0 {
			rate := float64(recent.successfulRuns24H) / float64(terminalRuns)
			successRate = &rate
		}
		health := cronJobHealth(
			now,
			metadata.Interval,
			recent.currentlyRunning,
			scheduleAnchor,
			state,
			recent.failedRuns24Hours,
		)
		item := CronJobStatistic{
			Kind:               metadata.Kind,
			Name:               metadata.Name,
			Description:        metadata.Description,
			Queue:              metadata.Queue,
			IntervalSeconds:    int64(metadata.Interval.Seconds()),
			RunOnStart:         metadata.RunOnStart,
			Health:             health,
			State:              state,
			CurrentlyRunning:   recent.currentlyRunning,
			LastJobID:          recent.lastJobID,
			LastRunAt:          lastRunAt,
			LastFinishedAt:     recent.finalizedAt,
			NextRunAt:          nextRunAt,
			LastDurationMS:     recent.lastDurationMS,
			AverageDurationMS:  recent.averageDurationMS,
			Runs24Hours:        recent.runs24Hours,
			SuccessfulRuns24H:  recent.successfulRuns24H,
			FailedRuns24Hours:  recent.failedRuns24Hours,
			SuccessRate24Hours: successRate,
			WorkerID:           recent.workerID,
			LastError:          recent.lastError,
		}
		if recent.attempt != nil {
			item.Attempt = *recent.attempt
		}
		if recent.maxAttempts != nil {
			item.MaxAttempts = *recent.maxAttempts
		}
		result.Items = append(result.Items, item)
		switch health {
		case "running":
			result.Summary.Running++
		case "healthy":
			result.Summary.Healthy++
		default:
			result.Summary.Attention++
		}
	}
	return result, nil
}

func (store *Store) cronHistory(ctx context.Context, kinds []string) (map[string]cronHistory, error) {
	rows, err := store.pool.Query(ctx, `
		WITH kinds AS (
			SELECT kind, position
			FROM unnest($1::text[]) WITH ORDINALITY AS configured(kind, position)
		)
		SELECT kinds.kind,
		       latest.id,
		       latest.state,
		       latest.created_at,
		       latest.attempted_at,
		       latest.finalized_at,
		       latest.duration_ms,
		       latest.attempt,
		       latest.max_attempts,
		       latest.worker_id,
		       latest.last_error,
		       history.runs_24h,
		       history.successful_runs_24h,
		       history.failed_runs_24h,
		       active.currently_running,
		       history.average_duration_ms
		FROM kinds
		LEFT JOIN LATERAL (
			SELECT job.id,
			       job.state::text AS state,
			       job.created_at,
			       job.attempted_at,
			       job.finalized_at,
			       CASE WHEN job.attempted_at IS NULL THEN NULL ELSE
			         GREATEST(0, (extract(epoch FROM COALESCE(job.finalized_at, clock_timestamp()) - job.attempted_at) * 1000)::bigint)
			       END AS duration_ms,
			       job.attempt::integer AS attempt,
			       job.max_attempts::integer AS max_attempts,
			       CASE WHEN cardinality(job.attempted_by) > 0 THEN job.attempted_by[cardinality(job.attempted_by)] END AS worker_id,
			       CASE WHEN cardinality(job.errors) > 0 THEN job.errors[cardinality(job.errors)]->>'error' END AS last_error
			FROM river_job AS job
			WHERE job.kind = kinds.kind
			ORDER BY job.created_at DESC, job.id DESC
			LIMIT 1
		) AS latest ON true
		LEFT JOIN LATERAL (
			SELECT count(*)::integer AS runs_24h,
			       count(*) FILTER (
			         WHERE job.state = 'completed'
			       )::integer AS successful_runs_24h,
			       count(*) FILTER (
			         WHERE job.state IN ('discarded', 'cancelled')
			       )::integer AS failed_runs_24h,
			       (avg(extract(epoch FROM job.finalized_at - job.attempted_at) * 1000) FILTER (
			         WHERE job.attempted_at IS NOT NULL
			           AND job.finalized_at IS NOT NULL
			       ))::bigint AS average_duration_ms
			FROM river_job AS job
			WHERE job.kind = kinds.kind
			  AND job.created_at >= clock_timestamp() - make_interval(secs => $2)
		) AS history ON true
		LEFT JOIN LATERAL (
			SELECT count(*)::integer AS currently_running
			FROM river_job AS job
			WHERE job.kind = kinds.kind AND job.state = 'running'
		) AS active ON true
		ORDER BY kinds.position
	`, kinds, int64(cronHistoryWindow.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("query cron history: %w", err)
	}
	defer rows.Close()

	result := make(map[string]cronHistory, len(kinds))
	for rows.Next() {
		var item cronHistory
		if err := rows.Scan(
			&item.kind,
			&item.lastJobID,
			&item.state,
			&item.createdAt,
			&item.attemptedAt,
			&item.finalizedAt,
			&item.lastDurationMS,
			&item.attempt,
			&item.maxAttempts,
			&item.workerID,
			&item.lastError,
			&item.runs24Hours,
			&item.successfulRuns24H,
			&item.failedRuns24Hours,
			&item.currentlyRunning,
			&item.averageDurationMS,
		); err != nil {
			return nil, fmt.Errorf("scan cron history: %w", err)
		}
		result[item.kind] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cron history: %w", err)
	}
	return result, nil
}

func (store *Store) cronWorker(ctx context.Context, now time.Time) (CronWorkerStatus, error) {
	var leaderID *string
	var electedAt, expiresAt *time.Time
	if err := store.pool.QueryRow(ctx, `
		SELECT
		  (SELECT leader_id FROM river_leader WHERE name = 'default'),
		  (SELECT elected_at FROM river_leader WHERE name = 'default'),
		  (SELECT expires_at FROM river_leader WHERE name = 'default')
	`).Scan(&leaderID, &electedAt, &expiresAt); err != nil {
		return CronWorkerStatus{}, fmt.Errorf("query cron worker leader: %w", err)
	}

	queueCatalog := jobs.QueueCatalog()
	paused := make(map[string]bool)
	queueNames := make([]string, 0, len(queueCatalog))
	for _, queue := range queueCatalog {
		queueNames = append(queueNames, queue.Name)
	}
	rows, err := store.pool.Query(ctx, `
		SELECT name, paused_at IS NOT NULL
		FROM river_queue
		WHERE name = ANY($1::text[])
	`, queueNames)
	if err != nil {
		return CronWorkerStatus{}, fmt.Errorf("query cron worker queues: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var isPaused bool
		if err := rows.Scan(&name, &isPaused); err != nil {
			return CronWorkerStatus{}, fmt.Errorf("scan cron worker queue: %w", err)
		}
		paused[name] = isPaused
	}
	if err := rows.Err(); err != nil {
		return CronWorkerStatus{}, fmt.Errorf("iterate cron worker queues: %w", err)
	}

	worker := CronWorkerStatus{
		Status:         cronWorkerHealth(now, expiresAt),
		LeaderID:       leaderID,
		ElectedAt:      electedAt,
		LeaseExpiresAt: expiresAt,
		Queues:         make([]CronQueueWorker, 0, len(queueNames)),
	}
	for _, queue := range queueCatalog {
		worker.Queues = append(worker.Queues, CronQueueWorker{
			Name:       queue.Name,
			MaxWorkers: queue.MaxWorkers,
			Paused:     paused[queue.Name],
		})
		worker.MaxConcurrency += queue.MaxWorkers
	}
	return worker, nil
}

func cronJobHealth(now time.Time, interval time.Duration, running int, latestAt *time.Time, latestState string, failures24Hours int) string {
	if running > 0 {
		return "running"
	}
	if latestAt == nil {
		return "unknown"
	}
	grace := interval / 2
	if grace < 30*time.Second {
		grace = 30 * time.Second
	}
	if now.Sub(*latestAt) > interval+grace {
		return "overdue"
	}
	if latestState == "discarded" || latestState == "cancelled" {
		return "failed"
	}
	if latestState == "retryable" || failures24Hours > 0 {
		return "degraded"
	}
	return "healthy"
}

func cronScheduleAnchor(lastCreatedAt, electedAt *time.Time, runOnStart bool) *time.Time {
	if runOnStart || electedAt == nil || (lastCreatedAt != nil && !electedAt.After(*lastCreatedAt)) {
		return lastCreatedAt
	}
	anchor := *electedAt
	return &anchor
}

func cronWorkerHealth(now time.Time, expiresAt *time.Time) string {
	if expiresAt == nil {
		return "offline"
	}
	if expiresAt.After(now) {
		return "online"
	}
	return "stale"
}
