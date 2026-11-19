// Package reels builds short vertical Facebook Reels for trending storylines and keeps them as
// drafts until an editor approves them.
package reels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("reel not found")
	ErrConflict = errors.New("reel cannot change from its current status")
	ErrInvalid  = errors.New("invalid reel change")
)

const (
	StatusQueued     = "queued"
	StatusGenerating = "generating"
	StatusReady      = "ready"
	StatusApproved   = "approved"
	StatusRejected   = "rejected"
	StatusPosted     = "posted"
	StatusFailed     = "failed"
)

// Question is one candidate debate question with the model's estimate of the yes share.
type Question struct {
	Question    string `json:"question"`
	YesLabel    string `json:"yes_label"`
	NoLabel     string `json:"no_label"`
	ExpectedYes int    `json:"expected_yes_percent"`
}

// Script is everything the reel says on screen, plus the model's post text and image ideas.
type Script struct {
	Tag          string     `json:"tag"`
	Headline     []string   `json:"headline"`
	Facts        []string   `json:"facts"`
	Questions    []Question `json:"questions"`
	Question     string     `json:"question"`
	YesLabel     string     `json:"yes_label"`
	NoLabel      string     `json:"no_label"`
	Caption      string     `json:"caption"`
	Hashtags     []string   `json:"hashtags"`
	ImagePrompts []string   `json:"image_prompts"`
	People       []string   `json:"people"`
	PhotoQueries []string   `json:"photo_queries"`
}

// Photo is one image shown in the reel. Kind is "photo" (freely licensed) or "ai".
type Photo struct {
	Key       string `json:"key"`
	MIME      string `json:"mime"`
	Kind      string `json:"kind"`
	Credit    string `json:"credit"`
	License   string `json:"license,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
}

type Music struct {
	File      string  `json:"file"`
	Title     string  `json:"title"`
	Start     float64 `json:"start"`
	Generated bool    `json:"generated,omitempty"`
}

type Assets struct {
	Photos []Photo `json:"photos"`
	Music  *Music  `json:"music,omitempty"`
}

// Trend records why the storyline was chosen.
type Trend struct {
	Score        int      `json:"score"`
	GoogleTrends []string `json:"google_trends"`
	Reason       string   `json:"reason"`
	Topic        string   `json:"topic,omitempty"`
	YouTube      []string `json:"youtube,omitempty"`
	Web          []string `json:"web,omitempty"`
	Rank         int      `json:"radar_rank,omitempty"`
}

type Reel struct {
	ID             string     `json:"id"`
	StorylineID    *string    `json:"storyline_id"`
	EventID        *string    `json:"event_id"`
	Trigger        string     `json:"trigger"`
	Status         string     `json:"status"`
	Title          string     `json:"title"`
	Script         Script     `json:"script"`
	Caption        string     `json:"caption"`
	Assets         Assets     `json:"assets"`
	Trend          Trend      `json:"trend"`
	VideoKey       *string    `json:"-"`
	CoverKey       *string    `json:"-"`
	HasVideo       bool       `json:"has_video"`
	Error          *string    `json:"error"`
	RequestedBy    *string    `json:"requested_by"`
	ReviewedBy     *string    `json:"reviewed_by"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	RenderedAt     *time.Time `json:"rendered_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	StorylineTitle string     `json:"storyline_title"`
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const reelColumns = `r.id::text, r.storyline_id::text, r.event_id::text, r.trigger, r.status, r.title, r.script, r.caption,
	r.assets, r.trend, r.video_key, r.cover_key, r.error, r.requested_by::text, r.reviewed_by::text, r.reviewed_at,
	r.rendered_at, r.created_at, r.updated_at, COALESCE(s.title_si, '')`

const reelFrom = ` FROM social_reels r LEFT JOIN storylines s ON s.id = r.storyline_id`

func scanReel(row pgx.Row) (Reel, error) {
	var reel Reel
	var script, assets, trend []byte
	err := row.Scan(&reel.ID, &reel.StorylineID, &reel.EventID, &reel.Trigger, &reel.Status, &reel.Title, &script, &reel.Caption,
		&assets, &trend, &reel.VideoKey, &reel.CoverKey, &reel.Error, &reel.RequestedBy, &reel.ReviewedBy, &reel.ReviewedAt,
		&reel.RenderedAt, &reel.CreatedAt, &reel.UpdatedAt, &reel.StorylineTitle)
	if errors.Is(err, pgx.ErrNoRows) {
		return reel, ErrNotFound
	}
	if err != nil {
		return reel, err
	}
	for _, part := range []struct {
		data []byte
		into any
	}{{script, &reel.Script}, {assets, &reel.Assets}, {trend, &reel.Trend}} {
		if err := json.Unmarshal(part.data, part.into); err != nil {
			return reel, fmt.Errorf("decode reel %s: %w", reel.ID, err)
		}
	}
	reel.HasVideo = reel.VideoKey != nil && *reel.VideoKey != ""
	if reel.Assets.Photos == nil {
		reel.Assets.Photos = []Photo{}
	}
	return reel, nil
}

func (store *Store) Get(ctx context.Context, id string) (Reel, error) {
	return scanReel(store.pool.QueryRow(ctx, `SELECT `+reelColumns+reelFrom+` WHERE r.id::text = $1`, id))
}

func (store *Store) List(ctx context.Context, limit int) ([]Reel, error) {
	rows, err := store.pool.Query(ctx, `SELECT `+reelColumns+reelFrom+` ORDER BY r.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Reel, 0)
	for rows.Next() {
		reel, err := scanReel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, reel)
	}
	return items, rows.Err()
}

// Create records a queued draft. storylineID may be empty to let the builder pick the story.
func (store *Store) Create(ctx context.Context, trigger, storylineID string, requestedBy *string, trend Trend) (string, error) {
	data, _ := json.Marshal(trend)
	var id string
	err := store.pool.QueryRow(ctx, `INSERT INTO social_reels (trigger, storyline_id, requested_by, trend)
		VALUES ($1, NULLIF($2, '')::uuid, $3::uuid, $4) RETURNING id::text`, trigger, storylineID, requestedBy, data).Scan(&id)
	return id, err
}

// Start moves a queued or failed draft to generating; it fails when the draft is not waiting.
func (store *Store) Start(ctx context.Context, id string) error {
	tag, err := store.pool.Exec(ctx, `UPDATE social_reels SET status = 'generating', error = NULL, updated_at = clock_timestamp()
		WHERE id = $1 AND status IN ('queued', 'failed')`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

// SetStory records the storyline chosen by the builder and why.
func (store *Store) SetStory(ctx context.Context, id, storylineID string, trend Trend) error {
	data, _ := json.Marshal(trend)
	_, err := store.pool.Exec(ctx, `UPDATE social_reels SET storyline_id = $2::uuid, trend = $3, updated_at = clock_timestamp() WHERE id = $1`, id, storylineID, data)
	return err
}

func (store *Store) SaveDraft(ctx context.Context, id, eventID, title string, script Script, caption string, assets Assets) error {
	scriptData, _ := json.Marshal(script)
	assetData, _ := json.Marshal(assets)
	_, err := store.pool.Exec(ctx, `UPDATE social_reels SET event_id = NULLIF($2, '')::uuid, title = $3, script = $4, caption = $5,
		assets = $6, updated_at = clock_timestamp() WHERE id = $1`, id, eventID, title, scriptData, caption, assetData)
	return err
}

func (store *Store) SaveRendered(ctx context.Context, id, videoKey, coverKey string) error {
	_, err := store.pool.Exec(ctx, `UPDATE social_reels SET status = 'ready', video_key = $2, cover_key = $3, error = NULL,
		rendered_at = clock_timestamp(), updated_at = clock_timestamp() WHERE id = $1`, id, videoKey, coverKey)
	return err
}

func (store *Store) Fail(ctx context.Context, id string, cause error) error {
	message := cause.Error()
	if len(message) > 1000 {
		message = message[:1000]
	}
	_, err := store.pool.Exec(ctx, `UPDATE social_reels SET status = 'failed', error = $2, updated_at = clock_timestamp() WHERE id = $1`, id, message)
	return err
}

// Delete removes a draft that never got a story (a scheduled run with nothing new to cover).
func (store *Store) Delete(ctx context.Context, id string) error {
	_, err := store.pool.Exec(ctx, `DELETE FROM social_reels WHERE id = $1`, id)
	return err
}

// Edit changes the post text or the on-screen script. On-screen changes send a rendered draft
// back to the queue for a re-render; it reports whether that happened.
type Edit struct {
	Caption  *string   `json:"caption"`
	Tag      *string   `json:"tag"`
	Headline *[]string `json:"headline"`
	Facts    *[]string `json:"facts"`
	Question *string   `json:"question"`
	YesLabel *string   `json:"yes_label"`
	NoLabel  *string   `json:"no_label"`
}

func (edit Edit) onScreen() bool {
	return edit.Tag != nil || edit.Headline != nil || edit.Facts != nil || edit.Question != nil || edit.YesLabel != nil || edit.NoLabel != nil
}

func (store *Store) Edit(ctx context.Context, id string, edit Edit) (Reel, bool, error) {
	reel, err := store.Get(ctx, id)
	if err != nil {
		return reel, false, err
	}
	if reel.Status == StatusGenerating || reel.Status == StatusQueued || reel.Status == StatusPosted {
		return reel, false, ErrConflict
	}
	script := reel.Script
	if edit.Tag != nil {
		script.Tag = *edit.Tag
	}
	if edit.Headline != nil {
		script.Headline = *edit.Headline
	}
	if edit.Facts != nil {
		script.Facts = *edit.Facts
	}
	if edit.Question != nil {
		script.Question = *edit.Question
	}
	if edit.YesLabel != nil {
		script.YesLabel = *edit.YesLabel
	}
	if edit.NoLabel != nil {
		script.NoLabel = *edit.NoLabel
	}
	if err := validScript(script); err != nil {
		return reel, false, err
	}
	caption := reel.Caption
	if edit.Caption != nil {
		caption = *edit.Caption
	}
	if len([]rune(caption)) > 4000 {
		return reel, false, fmt.Errorf("%w: the description is longer than 4000 characters", ErrInvalid)
	}
	rerender := edit.onScreen() && reel.Status != StatusFailed
	status := reel.Status
	if rerender {
		status = StatusQueued
	}
	data, _ := json.Marshal(script)
	if _, err := store.pool.Exec(ctx, `UPDATE social_reels SET script = $2, caption = $3, status = $4, reviewed_by = NULL,
		reviewed_at = NULL, updated_at = clock_timestamp() WHERE id = $1`, id, data, caption, status); err != nil {
		return reel, false, err
	}
	reel, err = store.Get(ctx, id)
	return reel, rerender, err
}

// Review applies an editor decision: approve, reject, posted (posted by hand on Facebook) or reopen.
func (store *Store) Review(ctx context.Context, id, action string, actor *string) (Reel, error) {
	allowed := map[string]struct {
		to   string
		from []string
	}{
		"approve": {StatusApproved, []string{StatusReady, StatusRejected}},
		"reject":  {StatusRejected, []string{StatusReady, StatusApproved, StatusFailed}},
		"posted":  {StatusPosted, []string{StatusApproved, StatusReady}},
		"reopen":  {StatusReady, []string{StatusApproved, StatusRejected, StatusPosted}},
	}
	rule, ok := allowed[action]
	if !ok {
		return Reel{}, ErrInvalid
	}
	tag, err := store.pool.Exec(ctx, `UPDATE social_reels SET status = $2, reviewed_by = $3::uuid, reviewed_at = clock_timestamp(),
		updated_at = clock_timestamp() WHERE id = $1 AND status = ANY($4) AND (video_key IS NOT NULL OR $2 = 'rejected')`,
		id, rule.to, actor, rule.from)
	if err != nil {
		return Reel{}, err
	}
	if tag.RowsAffected() == 0 {
		if _, err := store.Get(ctx, id); err != nil {
			return Reel{}, err
		}
		return Reel{}, ErrConflict
	}
	return store.Get(ctx, id)
}

// Queue sends a draft back for a re-render with its saved script and images.
func (store *Store) Queue(ctx context.Context, id string) error {
	tag, err := store.pool.Exec(ctx, `UPDATE social_reels SET status = 'queued', error = NULL, updated_at = clock_timestamp()
		WHERE id = $1 AND status IN ('ready', 'approved', 'rejected', 'failed')`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

// recentStories maps each storyline reeled in the window to the event its latest reel covered.
func (store *Store) recentStories(ctx context.Context, window time.Duration) (map[string]reeled, error) {
	rows, err := store.pool.Query(ctx, `SELECT DISTINCT ON (storyline_id) storyline_id::text, COALESCE(event_id::text, ''), created_at
		FROM social_reels WHERE storyline_id IS NOT NULL AND created_at >= now() - $1::interval AND status <> 'failed'
		ORDER BY storyline_id, created_at DESC`, fmt.Sprintf("%d seconds", int(window.Seconds())))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]reeled)
	for rows.Next() {
		var storylineID string
		var item reeled
		if err := rows.Scan(&storylineID, &item.eventID, &item.at); err != nil {
			return nil, err
		}
		result[storylineID] = item
	}
	return result, rows.Err()
}

type reeled struct {
	eventID string
	at      time.Time
}

// lastMusic returns the soundtrack files used by the most recent drafts, newest first.
func (store *Store) lastMusic(ctx context.Context, limit int) ([]string, error) {
	rows, err := store.pool.Query(ctx, `SELECT assets->'music'->>'file' FROM social_reels
		WHERE assets ? 'music' ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []string
	for rows.Next() {
		var file *string
		if err := rows.Scan(&file); err != nil {
			return nil, err
		}
		if file != nil {
			files = append(files, *file)
		}
	}
	return files, rows.Err()
}

// validScript keeps edited on-screen text within what the layout can show.
func validScript(script Script) error {
	runes := func(s string) int { return len([]rune(s)) }
	switch {
	case runes(script.Tag) > 30:
		return fmt.Errorf("%w: the label is longer than 30 characters", ErrInvalid)
	case len(script.Headline) < 1 || len(script.Headline) > 3:
		return fmt.Errorf("%w: the headline needs 1 to 3 lines", ErrInvalid)
	case len(script.Facts) < 1 || len(script.Facts) > 3:
		return fmt.Errorf("%w: the reel needs 1 to 3 facts", ErrInvalid)
	case runes(script.Question) < 4 || runes(script.Question) > 90:
		return fmt.Errorf("%w: the question must be 4 to 90 characters", ErrInvalid)
	case runes(script.YesLabel) < 1 || runes(script.YesLabel) > 16 || runes(script.NoLabel) < 1 || runes(script.NoLabel) > 16:
		return fmt.Errorf("%w: the answer labels must be 1 to 16 characters", ErrInvalid)
	}
	for _, line := range script.Headline {
		if runes(line) < 1 || runes(line) > 30 {
			return fmt.Errorf("%w: each headline line must be 1 to 30 characters", ErrInvalid)
		}
	}
	for _, fact := range script.Facts {
		if runes(fact) < 1 || runes(fact) > 90 {
			return fmt.Errorf("%w: each fact must be 1 to 90 characters", ErrInvalid)
		}
	}
	return nil
}
