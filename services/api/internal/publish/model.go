package publish

import (
	"context"
	"time"
)

type Article struct {
	ID            string    `json:"id"`
	Headline      string    `json:"headline"`
	Source        Source    `json:"source"`
	Category      *Category `json:"category"`
	PublishedAt   time.Time `json:"published_at"`
	ReceivedAt    time.Time `json:"received_at"`
	OriginalURL   string    `json:"original_url"`
	Excerpt       *string   `json:"excerpt"`
	Media         *string   `json:"media"`
	EventID       *string   `json:"event_id"`
	EditorialNote *string   `json:"editorial_note"`
}

type Source struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Website     string `json:"website,omitempty"`
}

type Category struct {
	Slug   string `json:"slug"`
	NameSI string `json:"name_si"`
}

type Page struct {
	Items      []Article `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}

// KnowledgeGraph is the public, explicitly allowlisted analytical view.
// Keep it separate from the richer admin graph so operational fields cannot leak.
type KnowledgeGraph struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Days        int                 `json:"days"`
	Summary     KnowledgeSummary    `json:"summary"`
	Categories  []KnowledgeCategory `json:"categories"`
	Events      []KnowledgeEvent    `json:"events"`
}

type KnowledgeSummary struct {
	Articles          int `json:"articles"`
	Events            int `json:"events"`
	MultiSourceEvents int `json:"multi_source_events"`
	Sources           int `json:"sources"`
}

type KnowledgeCategory struct {
	Slug     string `json:"slug"`
	NameSI   string `json:"name_si"`
	NameEN   string `json:"name_en"`
	Articles int    `json:"articles"`
	Events   int    `json:"events"`
}

type KnowledgeEvent struct {
	ID             string             `json:"id"`
	Title          string             `json:"title"`
	Category       string             `json:"category"`
	CategoryNameSI string             `json:"category_name_si"`
	IsBreaking     bool               `json:"is_breaking"`
	LastUpdateAt   time.Time          `json:"last_update_at"`
	Articles       []KnowledgeArticle `json:"articles"`
}

type KnowledgeArticle struct {
	ID          string              `json:"id"`
	Headline    string              `json:"headline"`
	SourceID    string              `json:"source_id"`
	Source      string              `json:"source"`
	PublishedAt time.Time           `json:"published_at"`
	Narrative   *KnowledgeNarrative `json:"narrative,omitempty"`
}

type KnowledgeNarrative struct {
	Label         string  `json:"label"`
	EconomicFrame float64 `json:"economic_frame"`
	Confidence    float64 `json:"confidence"`
}

type Reader interface {
	ListPublic(ctx context.Context, limit int) (Page, error)
}
