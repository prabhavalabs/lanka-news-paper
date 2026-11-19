package trends

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("no trend radar run yet")

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type channel struct {
	ID, Title string
	Baselined bool // has enough videos older than two days for a 30-day baseline
}

func (store *Store) channels(ctx context.Context) ([]channel, error) {
	rows, err := store.pool.Query(ctx, `SELECT c.id, c.title, (SELECT count(*) FROM trend_youtube_videos v
		WHERE v.channel_id = c.id AND v.published_at < now() - interval '2 days') >= 20
		FROM trend_youtube_channels c WHERE c.enabled ORDER BY c.title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var channels []channel
	for rows.Next() {
		var c channel
		if err := rows.Scan(&c.ID, &c.Title, &c.Baselined); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

// saveVideos upserts statistics; chart ranks are cleared first so only the latest chart counts.
func (store *Store) saveVideos(ctx context.Context, videos []video, chartRanks map[string]int) error {
	return pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE trend_youtube_videos SET chart_rank = NULL WHERE chart_rank IS NOT NULL`); err != nil {
			return err
		}
		for _, v := range videos {
			var rank *int
			if r, ok := chartRanks[v.ID]; ok {
				rank = &r
			}
			if _, err := tx.Exec(ctx, `INSERT INTO trend_youtube_videos (id, channel_id, channel_title, title, published_at, views, likes, comments, chart_rank)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				ON CONFLICT (id) DO UPDATE SET channel_title = EXCLUDED.channel_title, title = EXCLUDED.title, views = EXCLUDED.views,
				  likes = EXCLUDED.likes, comments = EXCLUDED.comments, chart_rank = EXCLUDED.chart_rank, updated_at = clock_timestamp()`,
				v.ID, v.ChannelID, v.ChannelTitle, v.Title, v.PublishedAt, v.Views, v.Likes, v.Comments, rank); err != nil {
				return fmt.Errorf("save video %s: %w", v.ID, err)
			}
		}
		_, err := tx.Exec(ctx, `DELETE FROM trend_youtube_videos WHERE published_at < now() - interval '35 days'`)
		return err
	})
}

// videoSignals returns videos from the last window with their channel's 30-day median views
// (videos older than two days, so it reflects near-final counts), strongest first.
func (store *Store) videoSignals(ctx context.Context, window time.Duration, limit int) ([]VideoSignal, error) {
	rows, err := store.pool.Query(ctx, `
		WITH baseline AS (
		  SELECT channel_id, percentile_cont(0.5) WITHIN GROUP (ORDER BY views) AS median, count(*) AS samples
		  FROM trend_youtube_videos
		  WHERE published_at BETWEEN now() - interval '30 days' AND now() - interval '2 days'
		  GROUP BY channel_id
		)
		SELECT v.id, v.title, v.channel_title, v.views, extract(epoch FROM now() - v.published_at) / 3600,
		       COALESCE(CASE WHEN b.samples >= 5 THEN b.median END, 0), v.chart_rank
		FROM trend_youtube_videos v LEFT JOIN baseline b ON b.channel_id = v.channel_id
		WHERE v.published_at >= now() - $1::interval OR (v.chart_rank IS NOT NULL AND v.published_at >= now() - interval '72 hours')`, fmt.Sprintf("%d seconds", int(window.Seconds())))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var signals []VideoSignal
	for rows.Next() {
		var s VideoSignal
		if err := rows.Scan(&s.ID, &s.Title, &s.Channel, &s.Views, &s.AgeHours, &s.Baseline, &s.ChartRank); err != nil {
			return nil, err
		}
		s.Outlier = outlierRatio(s.Views, s.AgeHours, s.Baseline)
		s.URL = "https://www.youtube.com/watch?v=" + s.ID
		signals = append(signals, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return strongestVideos(signals, limit), nil
}

// Run is one trend radar pass and its ranked topics.
type Run struct {
	ID         string          `json:"id"`
	Status     string          `json:"status"`
	Research   []WebTopic      `json:"research"`
	Signals    json.RawMessage `json:"signals"`
	Error      *string         `json:"error"`
	CreatedAt  time.Time       `json:"created_at"`
	FinishedAt *time.Time      `json:"finished_at"`
	Topics     []Topic         `json:"topics"`
}

// Topic is one ranked story on the radar. Ready topics are covered by a storyline and may become
// reels; watch topics are hot but not yet covered by our outlets; skip topics are unsuitable.
type Topic struct {
	ID          string     `json:"id"`
	Rank        int        `json:"rank"`
	TitleSI     string     `json:"title_si"`
	TitleEN     string     `json:"title_en"`
	Category    string     `json:"category"`
	StorylineID *string    `json:"storyline_id"`
	Status      string     `json:"status"`
	Score       float64    `json:"score"`
	Components  Components `json:"components"`
	Evidence    Evidence   `json:"evidence"`
	Reason      string     `json:"reason"`
}

type Components struct {
	YouTube   float64 `json:"youtube"`
	Search    float64 `json:"search"`
	Web       float64 `json:"web"`
	Coverage  float64 `json:"coverage"`
	Agreement float64 `json:"agreement"`
}

type Evidence struct {
	YouTube   []VideoSignal `json:"youtube"`
	Google    []GoogleTrend `json:"google"`
	Web       []WebTopic    `json:"web"`
	Storyline *StorySignal  `json:"storyline,omitempty"`
}

func (store *Store) startRun(ctx context.Context) (string, error) {
	var id string
	err := store.pool.QueryRow(ctx, `INSERT INTO trend_radar_runs DEFAULT VALUES RETURNING id::text`).Scan(&id)
	return id, err
}

func (store *Store) failRun(ctx context.Context, id string, cause error) error {
	_, err := store.pool.Exec(ctx, `UPDATE trend_radar_runs SET status = 'failed', error = $2, finished_at = clock_timestamp() WHERE id = $1`, id, cause.Error())
	return err
}

func (store *Store) finishRun(ctx context.Context, id string, research []WebTopic, signals map[string]int, topics []Topic) error {
	researchData, _ := json.Marshal(research)
	signalData, _ := json.Marshal(signals)
	return pgx.BeginFunc(ctx, store.pool, func(tx pgx.Tx) error {
		for _, topic := range topics {
			components, _ := json.Marshal(topic.Components)
			evidence, _ := json.Marshal(topic.Evidence)
			if _, err := tx.Exec(ctx, `INSERT INTO trend_topics (run_id, rank, title_si, title_en, category, storyline_id, status, score, components, evidence, reason)
				VALUES ($1, $2, $3, $4, $5, $6::uuid, $7, $8, $9, $10, $11)`,
				id, topic.Rank, topic.TitleSI, topic.TitleEN, topic.Category, topic.StorylineID, topic.Status, topic.Score, components, evidence, topic.Reason); err != nil {
				return fmt.Errorf("save topic %d: %w", topic.Rank, err)
			}
		}
		_, err := tx.Exec(ctx, `UPDATE trend_radar_runs SET status = 'ready', research = $2, signals = $3, finished_at = clock_timestamp() WHERE id = $1`, id, researchData, signalData)
		return err
	})
}

// Latest returns the newest radar run (any status) with its topics.
func (store *Store) Latest(ctx context.Context) (Run, error) {
	return store.latest(ctx, false)
}

func (store *Store) latest(ctx context.Context, readyOnly bool) (Run, error) {
	var run Run
	var research []byte
	query := `SELECT id::text, status, research, signals, error, created_at, finished_at FROM trend_radar_runs`
	if readyOnly {
		query += ` WHERE status = 'ready'`
	}
	err := store.pool.QueryRow(ctx, query+` ORDER BY created_at DESC LIMIT 1`).Scan(&run.ID, &run.Status, &research, &run.Signals, &run.Error, &run.CreatedAt, &run.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return run, ErrNotFound
	}
	if err != nil {
		return run, err
	}
	if err := json.Unmarshal(research, &run.Research); err != nil {
		return run, err
	}
	rows, err := store.pool.Query(ctx, `SELECT id::text, rank, title_si, title_en, category, storyline_id::text, status, score::float8, components, evidence, reason
		FROM trend_topics WHERE run_id = $1 ORDER BY rank`, run.ID)
	if err != nil {
		return run, err
	}
	defer rows.Close()
	run.Topics = []Topic{}
	for rows.Next() {
		var topic Topic
		var components, evidence []byte
		if err := rows.Scan(&topic.ID, &topic.Rank, &topic.TitleSI, &topic.TitleEN, &topic.Category, &topic.StorylineID, &topic.Status,
			&topic.Score, &components, &evidence, &topic.Reason); err != nil {
			return run, err
		}
		if err := json.Unmarshal(components, &topic.Components); err != nil {
			return run, err
		}
		if err := json.Unmarshal(evidence, &topic.Evidence); err != nil {
			return run, err
		}
		run.Topics = append(run.Topics, topic)
	}
	if run.Research == nil {
		run.Research = []WebTopic{}
	}
	return run, rows.Err()
}

// running reports whether a radar pass started within the last 20 minutes is still going.
func (store *Store) running(ctx context.Context) (bool, error) {
	var running bool
	err := store.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM trend_radar_runs WHERE status = 'running' AND created_at > now() - interval '20 minutes')`).Scan(&running)
	return running, err
}

// expireStale marks runs that never finished (a worker restart) as failed.
func (store *Store) expireStale(ctx context.Context) error {
	_, err := store.pool.Exec(ctx, `UPDATE trend_radar_runs SET status = 'failed', error = 'The radar run did not finish (the worker restarted).', finished_at = clock_timestamp()
		WHERE status = 'running' AND created_at <= now() - interval '20 minutes'`)
	return err
}
