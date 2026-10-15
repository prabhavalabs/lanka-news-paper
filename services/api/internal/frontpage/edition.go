package frontpage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/media"
)

// Block sizes, largest first, with the report length each one is written to.
const (
	SizeLead     = "lead"
	SizeFeature  = "feature"
	SizeStandard = "standard"
	SizeShort    = "short"
	SizeBrief    = "brief"
)

var sizeWords = map[string]int{SizeLead: 450, SizeFeature: 300, SizeStandard: 180, SizeShort: 110, SizeBrief: 60}

const (
	maxBlocks         = 24
	maxImages         = 3
	candidatePool     = 36
	editionMaxAge     = 12 * time.Hour
	editionRefreshAge = 3 * time.Hour
	// Builds pause overnight in Colombo; the last edition stays up.
	quietStartHour   = 0
	quietEndHour     = 5
	evidenceArticles = 6
	evidenceRunes    = 2500
	imageTimeout     = 4 * time.Minute
)

// Illustrator draws one illustration from a prompt.
type Illustrator interface {
	GenerateImage(ctx context.Context, model, prompt string, timeout time.Duration) ([]byte, error)
}

type StoryReport struct {
	Headline   string    `json:"headline"`
	Standfirst string    `json:"standfirst"`
	Paragraphs []string  `json:"paragraphs"`
	WordCount  int       `json:"word_count"`
	WrittenAt  time.Time `json:"written_at"`
}

type StoryImage struct {
	URL     string `json:"url"`
	Alt     string `json:"alt"`
	Caption string `json:"caption"`
	Kind    string `json:"kind"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type EditionBlock struct {
	Size   string       `json:"size"`
	Story  Story        `json:"story"`
	Report *StoryReport `json:"report"`
	Image  *StoryImage  `json:"image"`
}

// storedBlock is one block as saved in editions.blocks.
type storedBlock struct {
	Size     string `json:"size"`
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	ReportID string `json:"report_id,omitempty"`
	Image    bool   `json:"image"`
}

// Desk holds what the worker needs to build editions; the API runs without it.
type Desk struct {
	Model       llm.Completer
	Illustrator Illustrator
	Media       *media.Store
	ImageModel  string
}

func (service *Service) SetDesk(desk Desk) { service.desk = &desk }

// SetOnChange registers a callback for editor changes, used to rebuild the edition promptly.
func (service *Service) SetOnChange(callback func(context.Context)) { service.onChange = callback }

func (service *Service) changed(ctx context.Context) {
	service.invalidate()
	if service.onChange != nil {
		service.onChange(ctx)
	}
}

// BuildEdition lays out, writes and illustrates a new edition and publishes it. It does nothing
// overnight or when the candidate stories have not changed since a recent edition.
func (service *Service) BuildEdition(ctx context.Context, force bool) error {
	if service.desk == nil || service.desk.Model == nil {
		return errors.New("the front page desk is not configured")
	}
	now := service.now()
	if hour := now.In(service.location).Hour(); !force && hour >= quietStartHour && hour < quietEndHour {
		return nil
	}
	placements, err := service.placements(ctx, now)
	if err != nil {
		return err
	}
	candidates, err := service.store.candidates(ctx, now, candidatePool)
	if err != nil {
		return err
	}
	stories := map[string]Story{}
	for _, story := range candidates {
		stories[story.Kind+":"+story.ID] = story
	}
	for _, placement := range placements {
		stories[placement.Story.Kind+":"+placement.Story.ID] = placement.Story
	}
	if len(stories) == 0 {
		return nil
	}
	fingerprint := editionFingerprint(candidates, placements)
	previous, err := service.store.latestEdition(ctx)
	if err != nil {
		return err
	}
	if !force && previous != nil && previous.fingerprint == fingerprint && now.Sub(previous.publishedAt) < editionRefreshAge {
		return nil
	}
	editionID, err := service.store.startEdition(ctx, fingerprint)
	if err != nil {
		return err
	}
	blocks, note, layoutErr := service.layout(ctx, candidates, placements, previous)
	if layoutErr != nil {
		slog.Warn("edition layout fell back to ranking", "error", layoutErr)
		blocks, note = fallbackLayout(candidates, placements), "Ranked automatically because the layout editor was unavailable."
	}
	blocks = service.writeReports(ctx, blocks, stories)
	service.illustrate(ctx, blocks, stories)
	if err := service.store.publishEdition(ctx, editionID, blocks, note); err != nil {
		_ = service.store.failEdition(context.WithoutCancel(ctx), editionID, err)
		return err
	}
	service.invalidate()
	return nil
}

func editionFingerprint(candidates []Story, placements []Placement) string {
	hash := sha256.New()
	limit := min(len(candidates), maxBlocks)
	for _, story := range candidates[:limit] {
		fmt.Fprintf(hash, "%s:%s:%d:%d|", story.Kind, story.ID, story.UpdatedAt.Unix(), story.SourceCount)
		if story.Storyline != nil {
			fmt.Fprintf(hash, "s%s:%t|", story.Storyline.ID, story.Storyline.Trending)
		}
	}
	for _, placement := range placements {
		fmt.Fprintf(hash, "p%s:%s:%s|", placement.Slot, placement.Story.Kind, placement.Story.ID)
	}
	return hex.EncodeToString(hash.Sum(nil))[:32]
}

type layoutCandidate struct {
	StorylineTitle    string `json:"storyline_title,omitempty"`
	StorylineTrending bool   `json:"storyline_trending"`
	Kind              string `json:"kind"`
	ID                string `json:"id"`
	Headline          string `json:"headline"`
	Summary           string `json:"summary"`
	Section           string `json:"section"`
	Breaking          bool   `json:"breaking"`
	Outlets           int    `json:"outlets"`
	HoursOld          int    `json:"hours_since_update"`
	FixedSlot         string `json:"fixed_slot,omitempty"`
	OnPrevious        bool   `json:"on_previous_edition"`
}

var layoutSchema = map[string]any{
	"type": "object", "additionalProperties": false, "required": []string{"blocks", "note"},
	"properties": map[string]any{
		"note": map[string]any{"type": "string", "maxLength": 600},
		"blocks": map[string]any{"type": "array", "minItems": 1, "maxItems": maxBlocks, "items": map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"kind", "id", "size", "image"},
			"properties": map[string]any{
				"kind":  map[string]any{"type": "string", "enum": []string{KindEvent, KindArticle}},
				"id":    map[string]any{"type": "string"},
				"size":  map[string]any{"type": "string", "enum": []string{SizeLead, SizeFeature, SizeStandard, SizeShort, SizeBrief}},
				"image": map[string]any{"type": "boolean"},
			},
		}},
	},
}

func (service *Service) layout(ctx context.Context, candidates []Story, placements []Placement, previous *edition) ([]storedBlock, string, error) {
	now := service.now()
	onPrevious := map[string]bool{}
	if previous != nil {
		for _, block := range previous.blocks {
			onPrevious[block.Kind+":"+block.ID] = true
		}
	}
	fixed := map[string]string{}
	input := make([]layoutCandidate, 0, len(candidates)+len(placements))
	add := func(story Story, slot string) {
		item := layoutCandidate{Kind: story.Kind, ID: story.ID, Headline: story.Headline, Breaking: story.Breaking, Outlets: story.SourceCount,
			HoursOld: int(now.Sub(story.UpdatedAt).Hours()), FixedSlot: slot, OnPrevious: onPrevious[story.Kind+":"+story.ID]}
		if story.Storyline != nil {
			item.StorylineTitle = story.Storyline.Title
			item.StorylineTrending = story.Storyline.Trending
		}
		if story.Summary != nil {
			item.Summary = truncateRunes(*story.Summary, 300)
		}
		if story.Category != nil {
			item.Section = story.Category.Slug
		}
		input = append(input, item)
	}
	for _, placement := range placements {
		fixed[placement.Story.Kind+":"+placement.Story.ID] = placement.Slot
		add(placement.Story, placement.Slot)
	}
	for _, story := range candidates {
		if _, ok := fixed[story.Kind+":"+story.ID]; !ok {
			add(story, "")
		}
	}
	data, err := json.Marshal(map[string]any{"candidates": input})
	if err != nil {
		return nil, "", err
	}
	response, err := service.desk.Model.Complete(ctx, llm.Request{Task: "edition_layout", System: layoutPrompt, Input: string(data), JSONSchema: layoutSchema})
	if err != nil {
		return nil, "", err
	}
	if response.Provider == "none" || strings.TrimSpace(response.Text) == "" {
		return nil, "", errors.New("layout model unavailable")
	}
	var result struct {
		Blocks []storedBlock `json:"blocks"`
		Note   string        `json:"note"`
	}
	if err := json.Unmarshal([]byte(response.Text), &result); err != nil {
		return nil, "", fmt.Errorf("decode layout: %w", err)
	}
	allowed := map[string]bool{}
	for _, item := range input {
		allowed[item.Kind+":"+item.ID] = true
	}
	return normalizeLayout(result.Blocks, allowed, fixed), truncateRunes(result.Note, 600), nil
}

// normalizeLayout keeps known, unique stories, enforces one lead and the image limit, and
// makes sure every fixed placement is on the page in a size that matches its slot.
func normalizeLayout(blocks []storedBlock, allowed map[string]bool, fixed map[string]string) []storedBlock {
	seen := map[string]bool{}
	result := make([]storedBlock, 0, len(blocks)+len(fixed))
	hasLead := false
	for _, block := range blocks {
		key := block.Kind + ":" + block.ID
		if !allowed[key] || seen[key] || sizeWords[block.Size] == 0 {
			continue
		}
		seen[key] = true
		if slot, ok := fixed[key]; ok {
			block.Size = sizeForSlot(slot, block.Size)
		}
		if block.Size == SizeLead {
			if hasLead {
				block.Size = SizeFeature
			}
			hasLead = true
		}
		result = append(result, block)
	}
	for key, slot := range fixed {
		if seen[key] {
			continue
		}
		kind, id, _ := strings.Cut(key, ":")
		result = append(result, storedBlock{Kind: kind, ID: id, Size: sizeForSlot(slot, SizeStandard)})
	}
	// A fixed lead always leads; otherwise promote the first story when the model chose none.
	for i, block := range result {
		if fixed[block.Kind+":"+block.ID] == SlotLead {
			for j := range result {
				if j != i && result[j].Size == SizeLead {
					result[j].Size = SizeFeature
				}
			}
			result[i].Size = SizeLead
			result[0], result[i] = result[i], result[0]
			hasLead = true
			break
		}
	}
	if !hasLead && len(result) > 0 {
		result[0].Size = SizeLead
	}
	if len(result) > maxBlocks {
		result = result[:maxBlocks]
	}
	images := 0
	for i := range result {
		if result[i].Image {
			if images >= maxImages || result[i].Size == SizeBrief || result[i].Size == SizeShort {
				result[i].Image = false
				continue
			}
			images++
		}
	}
	return result
}

func sizeForSlot(slot, chosen string) string {
	switch slot {
	case SlotLead:
		return SizeLead
	case SlotTop:
		if chosen == SizeFeature || chosen == SizeStandard {
			return chosen
		}
		return SizeFeature
	default:
		if chosen == SizeShort || chosen == SizeBrief {
			return chosen
		}
		return SizeShort
	}
}

// fallbackLayout arranges the ranked candidates when the layout editor is unavailable.
func fallbackLayout(candidates []Story, placements []Placement) []storedBlock {
	sizes := []string{SizeLead, SizeFeature, SizeFeature, SizeStandard, SizeStandard, SizeStandard, SizeStandard, SizeShort, SizeShort, SizeShort, SizeShort, SizeShort}
	allowed := map[string]bool{}
	fixed := map[string]string{}
	blocks := make([]storedBlock, 0, maxBlocks)
	for _, placement := range placements {
		key := placement.Story.Kind + ":" + placement.Story.ID
		allowed[key], fixed[key] = true, placement.Slot
	}
	chosen := make([]map[string]bool, 0, maxBlocks)
	for _, story := range candidates {
		if len(blocks) >= 20 {
			break
		}
		words := wordsOf(story)
		if repeats(storyWords{headline: words.headline}, toStoryWords(chosen)) {
			continue
		}
		chosen = append(chosen, words.headline)
		size := SizeBrief
		if len(blocks) < len(sizes) {
			size = sizes[len(blocks)]
		}
		key := story.Kind + ":" + story.ID
		allowed[key] = true
		blocks = append(blocks, storedBlock{Kind: story.Kind, ID: story.ID, Size: size, Image: len(blocks) == 0})
	}
	return normalizeLayout(blocks, allowed, fixed)
}

func toStoryWords(headlines []map[string]bool) []storyWords {
	result := make([]storyWords, len(headlines))
	for i, headline := range headlines {
		result[i] = storyWords{headline: headline}
	}
	return result
}

func truncateRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit-1]) + "…"
}

// writtenSizes are the block sizes the desk writes reports for. Every story on the front page
// is rewritten so readers only see text written to the house style (natural attribution, one
// script); reports are reused while a story is unchanged, so later editions write few new ones.
var writtenSizes = map[string]bool{SizeLead: true, SizeFeature: true, SizeStandard: true, SizeShort: true, SizeBrief: true}

// writeReports writes (or reuses) a report for every block, two at a time.
func (service *Service) writeReports(ctx context.Context, blocks []storedBlock, stories map[string]Story) []storedBlock {
	var wait sync.WaitGroup
	slots := make(chan struct{}, 2)
	for i := range blocks {
		if !writtenSizes[blocks[i].Size] {
			continue
		}
		story, ok := stories[blocks[i].Kind+":"+blocks[i].ID]
		if !ok {
			continue
		}
		wait.Add(1)
		go func(i int, story Story) {
			defer wait.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			reportID, err := service.report(ctx, story, blocks[i].Size)
			if err != nil {
				slog.Warn("front page report not written", "story", story.ID, "size", blocks[i].Size, "error", err)
				return
			}
			blocks[i].ReportID = reportID
		}(i, story)
	}
	wait.Wait()
	return blocks
}

type edition struct {
	id          string
	blocks      []storedBlock
	fingerprint string
	publishedAt time.Time
}

func (store *Store) latestEdition(ctx context.Context) (*edition, error) {
	var item edition
	var blocks []byte
	err := store.pool.QueryRow(ctx, `
		SELECT id::text, blocks, fingerprint, published_at FROM editions
		WHERE status = 'published' ORDER BY published_at DESC LIMIT 1`).Scan(&item.id, &blocks, &item.fingerprint, &item.publishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load latest edition: %w", err)
	}
	if err := json.Unmarshal(blocks, &item.blocks); err != nil {
		return nil, fmt.Errorf("decode edition blocks: %w", err)
	}
	return &item, nil
}

func (store *Store) startEdition(ctx context.Context, fingerprint string) (string, error) {
	var id string
	err := store.pool.QueryRow(ctx, `INSERT INTO editions (fingerprint) VALUES ($1) RETURNING id::text`, fingerprint).Scan(&id)
	return id, err
}

func (store *Store) publishEdition(ctx context.Context, id string, blocks []storedBlock, note string) error {
	data, err := json.Marshal(blocks)
	if err != nil {
		return err
	}
	_, err = store.pool.Exec(ctx, `
		UPDATE editions SET status = 'published', blocks = $2, layout_note = $3, published_at = clock_timestamp()
		WHERE id = $1`, id, data, note)
	return err
}

func (store *Store) failEdition(ctx context.Context, id string, cause error) error {
	_, err := store.pool.Exec(ctx, `UPDATE editions SET status = 'failed', error = $2 WHERE id = $1`, id, truncateRunes(cause.Error(), 2000))
	return err
}

type reportRow struct {
	report StoryReport
	image  *StoryImage
}

func (store *Store) reports(ctx context.Context, ids []string) (map[string]reportRow, error) {
	result := map[string]reportRow{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := store.pool.Query(ctx, `
		SELECT id::text, headline, standfirst, paragraphs, word_count, created_at, image_key, image_alt, image_width, image_height
		FROM story_reports WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, fmt.Errorf("load story reports: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var row reportRow
		var paragraphs []byte
		var key, alt *string
		var width, height *int
		if err := rows.Scan(&id, &row.report.Headline, &row.report.Standfirst, &paragraphs, &row.report.WordCount, &row.report.WrittenAt, &key, &alt, &width, &height); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(paragraphs, &row.report.Paragraphs); err != nil {
			return nil, err
		}
		row.image = imageFor(key, alt, width, height)
		result[id] = row
	}
	return result, rows.Err()
}

// attachEdition fills page.Blocks from the latest published edition (if recent) and derives
// the lead/top/more lists from it so older readers of the API keep working.
func (service *Service) attachEdition(ctx context.Context, page *FrontPage, placements []Placement, now time.Time) error {
	latest, err := service.store.latestEdition(ctx)
	if err != nil || latest == nil || now.Sub(latest.publishedAt) > editionMaxAge {
		return err
	}
	var eventIDs, articleIDs, reportIDs []string
	for _, block := range latest.blocks {
		if block.Kind == KindEvent {
			eventIDs = append(eventIDs, block.ID)
		} else {
			articleIDs = append(articleIDs, block.ID)
		}
		if block.ReportID != "" {
			reportIDs = append(reportIDs, block.ReportID)
		}
	}
	events, err := service.store.eventStories(ctx, eventIDs, false)
	if err != nil {
		return err
	}
	articles, err := service.store.articleStories(ctx, articleIDs)
	if err != nil {
		return err
	}
	reports, err := service.store.reports(ctx, reportIDs)
	if err != nil {
		return err
	}
	curated := map[string]Placement{}
	for _, placement := range placements {
		curated[placement.Story.Kind+":"+placement.Story.ID] = placement
	}
	blocks := make([]EditionBlock, 0, len(latest.blocks))
	for _, stored := range latest.blocks {
		story, ok := events[stored.ID]
		if stored.Kind == KindArticle {
			story, ok = articles[stored.ID]
		}
		if !ok {
			continue
		}
		story.Slot = slotForSize(stored.Size)
		if placement, ok := curated[stored.Kind+":"+stored.ID]; ok {
			story.Curated, story.EditorNote = true, placement.Note
		}
		block := EditionBlock{Size: stored.Size, Story: story}
		if row, ok := reports[stored.ReportID]; ok {
			report := row.report
			block.Report = &report
			if stored.Image {
				block.Image = row.image
			}
		}
		blocks = append(blocks, block)
	}
	if len(blocks) == 0 {
		return nil
	}
	page.Blocks, page.EditionID = blocks, &latest.id
	page.Lead, page.Top, page.More = nil, []Story{}, []Story{}
	for _, block := range blocks {
		switch story := block.Story; story.Slot {
		case SlotLead:
			if page.Lead == nil {
				page.Lead = &story
			}
		case SlotTop:
			page.Top = append(page.Top, story)
		default:
			page.More = append(page.More, story)
		}
	}
	return nil
}

func slotForSize(size string) string {
	switch size {
	case SizeLead:
		return SlotLead
	case SizeFeature, SizeStandard:
		return SlotTop
	default:
		return SlotMore
	}
}

// Report returns the latest report and illustration written for a story.
func (service *Service) Report(ctx context.Context, kind, id string) (StoryReport, *StoryImage, error) {
	if kind != KindEvent && kind != KindArticle {
		return StoryReport{}, nil, ErrNotFound
	}
	column := "event_id"
	if kind == KindArticle {
		column = "article_id"
	}
	var reportID string
	err := service.store.pool.QueryRow(ctx, `SELECT id::text FROM story_reports WHERE `+column+`::text = $1
		ORDER BY CASE size WHEN 'lead' THEN 0 WHEN 'feature' THEN 1 WHEN 'standard' THEN 2 WHEN 'short' THEN 3 ELSE 4 END, created_at DESC LIMIT 1`, id).Scan(&reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return StoryReport{}, nil, ErrNotFound
	}
	if err != nil {
		return StoryReport{}, nil, err
	}
	rows, err := service.store.reports(ctx, []string{reportID})
	if err != nil {
		return StoryReport{}, nil, err
	}
	row := rows[reportID]
	if row.image == nil {
		// The longest report may not carry the story's illustration; use any one the story has.
		var key, alt *string
		var width, height *int
		_ = service.store.pool.QueryRow(ctx, `SELECT image_key, image_alt, image_width, image_height FROM story_reports
			WHERE `+column+`::text = $1 AND image_key IS NOT NULL ORDER BY created_at DESC LIMIT 1`, id).Scan(&key, &alt, &width, &height)
		row.image = imageFor(key, alt, width, height)
	}
	return row.report, row.image, nil
}
