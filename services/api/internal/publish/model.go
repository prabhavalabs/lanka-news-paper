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

type Reader interface {
	ListPublic(ctx context.Context, limit int) (Page, error)
}
