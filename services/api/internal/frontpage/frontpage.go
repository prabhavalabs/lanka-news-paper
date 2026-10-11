package frontpage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	SlotLead = "lead"
	SlotTop  = "top"
	SlotMore = "more"

	topStories  = 6
	moreStories = 12
	maxTop      = 12
	maxMore     = 24
	maxNote     = 280
	// The public front page is rebuilt at most this often; editor changes clear it at once.
	cacheTTL = 30 * time.Second
)

var (
	ErrInvalid  = errors.New("invalid front page request")
	ErrNotFound = errors.New("not found")
)

type FrontPage struct {
	Blocks      []EditionBlock    `json:"blocks"`
	EditionID   *string           `json:"edition_id"`
	EditionDate string            `json:"edition_date"`
	GeneratedAt time.Time         `json:"generated_at"`
	Lead        *Story            `json:"lead"`
	Top         []Story           `json:"top"`
	More        []Story           `json:"more"`
	Editorial   *EditorialSummary `json:"editorial"`
	Curated     bool              `json:"curated"`
}

type Placement struct {
	Slot      string     `json:"slot"`
	Rank      int        `json:"rank"`
	Story     Story      `json:"story"`
	Note      *string    `json:"note"`
	ExpiresAt *time.Time `json:"expires_at"`
	PlacedBy  string     `json:"placed_by"`
	PlacedAt  time.Time  `json:"placed_at"`
}

type PlacementInput struct {
	Slot      string     `json:"slot"`
	Kind      string     `json:"kind"`
	ID        string     `json:"id"`
	Note      *string    `json:"note"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type Source struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	FrontPage bool   `json:"front_page"`
}

type AdminView struct {
	Placements []Placement `json:"placements"`
	Candidates []Story     `json:"candidates"`
	Sources    []Source    `json:"sources"`
	Preview    FrontPage   `json:"preview"`
}

type Service struct {
	store    *Store
	now      func() time.Time
	location *time.Location

	desk     *Desk
	onChange func(context.Context)

	mu       sync.Mutex
	cached   *FrontPage
	cachedAt time.Time
}

func NewService(store *Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	location, err := time.LoadLocation("Asia/Colombo")
	if err != nil {
		location = time.FixedZone("Asia/Colombo", 5*3600+1800)
	}
	return &Service{store: store, now: now, location: location}
}

// FrontPage returns the public front page, rebuilding it when the cached copy is stale.
func (service *Service) FrontPage(ctx context.Context) (FrontPage, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	now := service.now()
	if service.cached != nil && now.Sub(service.cachedAt) < cacheTTL {
		return *service.cached, nil
	}
	page, err := service.build(ctx, now)
	if err != nil {
		return FrontPage{}, err
	}
	service.cached, service.cachedAt = &page, now
	return page, nil
}

func (service *Service) invalidate() {
	service.mu.Lock()
	service.cached = nil
	service.mu.Unlock()
}

func (service *Service) build(ctx context.Context, now time.Time) (FrontPage, error) {
	placements, err := service.placements(ctx, now)
	if err != nil {
		return FrontPage{}, err
	}
	candidates, err := service.store.candidates(ctx, now, 80)
	if err != nil {
		return FrontPage{}, err
	}
	page := FrontPage{Blocks: []EditionBlock{}, EditionDate: now.In(service.location).Format("2006-01-02"), GeneratedAt: now, Top: []Story{}, More: []Story{}}
	used := map[string]bool{}
	chosen := make([]storyWords, 0, 1+topStories+moreStories)
	for _, placement := range placements {
		story := placement.Story
		story.Curated, story.Slot, story.EditorNote = true, placement.Slot, placement.Note
		used[story.Kind+":"+story.ID] = true
		chosen = append(chosen, wordsOf(story))
		page.Curated = true
		switch placement.Slot {
		case SlotLead:
			page.Lead = &story
		case SlotTop:
			page.Top = append(page.Top, story)
		default:
			page.More = append(page.More, story)
		}
	}
	next := func(slot string) (Story, bool) {
		for len(candidates) > 0 {
			story := candidates[0]
			candidates = candidates[1:]
			if used[story.Kind+":"+story.ID] {
				continue
			}
			used[story.Kind+":"+story.ID] = true
			words := wordsOf(story)
			if repeats(words, chosen) {
				continue
			}
			chosen = append(chosen, words)
			story.Slot = slot
			return story, true
		}
		return Story{}, false
	}
	if page.Lead == nil {
		if story, ok := next(SlotLead); ok {
			page.Lead = &story
		}
	}
	for len(page.Top) < topStories {
		story, ok := next(SlotTop)
		if !ok {
			break
		}
		page.Top = append(page.Top, story)
	}
	for len(page.More) < moreStories {
		story, ok := next(SlotMore)
		if !ok {
			break
		}
		page.More = append(page.More, story)
	}
	editorials, err := service.store.publishedEditorials(ctx, 1)
	if err != nil {
		return FrontPage{}, err
	}
	if len(editorials) > 0 {
		page.Editorial = &editorials[0]
	}
	if err := service.attachEdition(ctx, &page, placements, now); err != nil {
		return FrontPage{}, err
	}
	return page, nil
}

// placements returns the unexpired placements whose stories are still public.
func (service *Service) placements(ctx context.Context, now time.Time) ([]Placement, error) {
	rows, err := service.store.pool.Query(ctx, `
		SELECT p.slot, p.rank, p.event_id::text, p.article_id::text, p.note, p.expires_at,
		       COALESCE(u.display_name, ''), p.placed_at
		FROM front_page_placements p LEFT JOIN admin_users u ON u.id = p.placed_by
		WHERE p.expires_at IS NULL OR p.expires_at > $1
		ORDER BY CASE p.slot WHEN 'lead' THEN 0 WHEN 'top' THEN 1 ELSE 2 END, p.rank`, now)
	if err != nil {
		return nil, fmt.Errorf("load front page placements: %w", err)
	}
	type row struct {
		placement Placement
		eventID   *string
		articleID *string
	}
	loaded := make([]row, 0)
	var eventIDs, articleIDs []string
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.placement.Slot, &item.placement.Rank, &item.eventID, &item.articleID, &item.placement.Note,
			&item.placement.ExpiresAt, &item.placement.PlacedBy, &item.placement.PlacedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if item.eventID != nil {
			eventIDs = append(eventIDs, *item.eventID)
		} else if item.articleID != nil {
			articleIDs = append(articleIDs, *item.articleID)
		}
		loaded = append(loaded, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	events, err := service.store.eventStories(ctx, eventIDs, false)
	if err != nil {
		return nil, err
	}
	articles, err := service.store.articleStories(ctx, articleIDs)
	if err != nil {
		return nil, err
	}
	result := make([]Placement, 0, len(loaded))
	for _, item := range loaded {
		var story Story
		var ok bool
		if item.eventID != nil {
			story, ok = events[*item.eventID]
		} else if item.articleID != nil {
			story, ok = articles[*item.articleID]
		}
		if !ok {
			continue
		}
		story.Curated, story.Slot, story.EditorNote = true, item.placement.Slot, item.placement.Note
		item.placement.Story = story
		result = append(result, item.placement)
	}
	return result, nil
}

func (service *Service) Admin(ctx context.Context) (AdminView, error) {
	now := service.now()
	placements, err := service.placements(ctx, now)
	if err != nil {
		return AdminView{}, err
	}
	candidates, err := service.store.candidates(ctx, now, 60)
	if err != nil {
		return AdminView{}, err
	}
	placed := map[string]bool{}
	for _, placement := range placements {
		placed[placement.Story.Kind+":"+placement.Story.ID] = true
	}
	open := make([]Story, 0, len(candidates))
	for _, story := range candidates {
		if !placed[story.Kind+":"+story.ID] {
			open = append(open, story)
		}
	}
	sources, err := service.store.sources(ctx)
	if err != nil {
		return AdminView{}, err
	}
	preview, err := service.build(ctx, now)
	if err != nil {
		return AdminView{}, err
	}
	return AdminView{Placements: placements, Candidates: open, Sources: sources, Preview: preview}, nil
}

// Replace swaps the whole set of placements for the given list, in order within each slot.
func (service *Service) Replace(ctx context.Context, actor uuid.UUID, inputs []PlacementInput) (AdminView, error) {
	if err := validatePlacements(inputs, service.now()); err != nil {
		return AdminView{}, err
	}
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return AdminView{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM front_page_placements`); err != nil {
		return AdminView{}, err
	}
	ranks := map[string]int{}
	for _, input := range inputs {
		var eventID, articleID *string
		if input.Kind == KindEvent {
			eventID = &input.ID
		} else {
			articleID = &input.ID
		}
		note := cleanNote(input.Note)
		if _, err := tx.Exec(ctx, `
			INSERT INTO front_page_placements (slot, rank, event_id, article_id, note, expires_at, placed_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, input.Slot, ranks[input.Slot], eventID, articleID, note, input.ExpiresAt, actor); err != nil {
			if strings.Contains(err.Error(), "foreign key") {
				return AdminView{}, fmt.Errorf("%w: story %s no longer exists", ErrInvalid, input.ID)
			}
			return AdminView{}, err
		}
		ranks[input.Slot]++
	}
	if err := writeAudit(ctx, tx, actor, "front_page_update", "front_page", "current"); err != nil {
		return AdminView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AdminView{}, err
	}
	service.changed(ctx)
	return service.Admin(ctx)
}

func validatePlacements(inputs []PlacementInput, now time.Time) error {
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, input := range inputs {
		if input.Slot != SlotLead && input.Slot != SlotTop && input.Slot != SlotMore {
			return fmt.Errorf("%w: unknown slot %q", ErrInvalid, input.Slot)
		}
		if input.Kind != KindEvent && input.Kind != KindArticle {
			return fmt.Errorf("%w: unknown story kind %q", ErrInvalid, input.Kind)
		}
		if _, err := uuid.Parse(input.ID); err != nil {
			return fmt.Errorf("%w: invalid story id", ErrInvalid)
		}
		key := input.Kind + ":" + input.ID
		if seen[key] {
			return fmt.Errorf("%w: a story can appear only once", ErrInvalid)
		}
		seen[key] = true
		if note := cleanNote(input.Note); note != nil && utf8.RuneCountInString(*note) > maxNote {
			return fmt.Errorf("%w: notes are limited to %d characters", ErrInvalid, maxNote)
		}
		if input.ExpiresAt != nil && !input.ExpiresAt.After(now) {
			return fmt.Errorf("%w: expiry must be in the future", ErrInvalid)
		}
		counts[input.Slot]++
	}
	switch {
	case counts[SlotLead] > 1:
		return fmt.Errorf("%w: only one lead story", ErrInvalid)
	case counts[SlotTop] > maxTop:
		return fmt.Errorf("%w: at most %d top stories", ErrInvalid, maxTop)
	case counts[SlotMore] > maxMore:
		return fmt.Errorf("%w: at most %d more stories", ErrInvalid, maxMore)
	}
	return nil
}

func cleanNote(note *string) *string {
	if note == nil {
		return nil
	}
	value := strings.TrimSpace(*note)
	if value == "" {
		return nil
	}
	return &value
}

func (store *Store) sources(ctx context.Context) ([]Source, error) {
	rows, err := store.pool.Query(ctx, `SELECT id::text, name, source_type, front_page FROM sources WHERE active ORDER BY name`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Source, error) {
		var source Source
		err := row.Scan(&source.ID, &source.Name, &source.Type, &source.FrontPage)
		return source, err
	})
}

// SetSourceFrontPage includes or excludes an outlet from automatic front-page selection.
func (service *Service) SetSourceFrontPage(ctx context.Context, actor uuid.UUID, id string, enabled bool) (Source, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Source{}, ErrNotFound
	}
	tx, err := service.store.pool.Begin(ctx)
	if err != nil {
		return Source{}, err
	}
	defer tx.Rollback(ctx)
	var source Source
	err = tx.QueryRow(ctx, `UPDATE sources SET front_page = $2 WHERE id = $1 RETURNING id::text, name, source_type, front_page`, id, enabled).
		Scan(&source.ID, &source.Name, &source.Type, &source.FrontPage)
	if errors.Is(err, pgx.ErrNoRows) {
		return Source{}, ErrNotFound
	}
	if err != nil {
		return Source{}, err
	}
	action := "front_page_source_exclude"
	if enabled {
		action = "front_page_source_include"
	}
	if err := writeAudit(ctx, tx, actor, action, "source", id); err != nil {
		return Source{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Source{}, err
	}
	service.changed(ctx)
	return source, nil
}

func writeAudit(ctx context.Context, tx pgx.Tx, actor uuid.UUID, action, targetType, target string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_logs (actor_id, action, target_type, target_id, result) VALUES ($1, $2, $3, $4, 'ok')`, actor, action, targetType, target)
	return err
}

// repeatOverlap is the share of the shorter headline's word stems that marks two stories as the
// same news reported in separate clusters. Stems are the first stemRunes characters, which
// absorbs Sinhala case endings (කමිටුවක් / කමිටුවේ).
const (
	repeatOverlap   = 0.4
	stemRunes       = 5
	minSummaryStems = 8
)

func headlineWords(text string) map[string]bool {
	result := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && r != '\u200d'
	}) {
		if runes := []rune(word); len(runes) >= 3 {
			result[string(runes[:min(len(runes), stemRunes)])] = true
		}
	}
	return result
}

// storyWords holds the stems of a story's headline and summary.
type storyWords struct{ headline, summary map[string]bool }

func wordsOf(story Story) storyWords {
	words := storyWords{headline: headlineWords(story.Headline)}
	if story.Summary != nil {
		words.summary = headlineWords(*story.Summary)
	}
	return words
}

// repeats reports whether a story's headline or summary largely matches one already chosen.
// Summaries need at least minSummaryStems stems so short texts do not match by chance.
func repeats(words storyWords, chosen []storyWords) bool {
	for _, other := range chosen {
		if overlap(words.headline, other.headline) >= repeatOverlap {
			return true
		}
		if len(words.summary) >= minSummaryStems && len(other.summary) >= minSummaryStems && overlap(words.summary, other.summary) >= repeatOverlap {
			return true
		}
	}
	return false
}

func overlap(a, b map[string]bool) float64 {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(a) == 0 {
		return 0
	}
	shared := 0
	for word := range a {
		if b[word] {
			shared++
		}
	}
	return float64(shared) / float64(len(a))
}
