package watchtower

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidQuestion = errors.New("question is required")
	ErrNotFound        = errors.New("watch tower conversation not found")
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

type Thread struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"-"`
	Title        string    `json:"title"`
	MessageCount int       `json:"message_count"`
	LastMessage  string    `json:"last_message"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Citation struct {
	Number      int       `json:"number"`
	ArticleID   uuid.UUID `json:"article_id"`
	Headline    string    `json:"headline"`
	Source      string    `json:"source"`
	Category    string    `json:"category"`
	PublishedAt time.Time `json:"published_at"`
	OriginalURL string    `json:"original_url"`
}

type SearchSummary struct {
	Label        string    `json:"label"`
	From         time.Time `json:"from"`
	To           time.Time `json:"to"`
	ArticleCount int       `json:"article_count"`
}

type Message struct {
	ID          uuid.UUID      `json:"id"`
	Role        string         `json:"role"`
	Content     string         `json:"content"`
	Citations   []Citation     `json:"citations"`
	Suggestions []string       `json:"suggestions"`
	Provider    string         `json:"provider,omitempty"`
	Model       string         `json:"model,omitempty"`
	Search      *SearchSummary `json:"search,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Conversation struct {
	Thread   Thread    `json:"thread"`
	Messages []Message `json:"messages"`
}

type Exchange struct {
	Thread    Thread  `json:"thread"`
	User      Message `json:"user"`
	Assistant Message `json:"assistant"`
}

type ArticleEvidence struct {
	ID           uuid.UUID
	Headline     string
	Source       string
	Category     string
	PublishedAt  time.Time
	OriginalURL  string
	EventTitle   string
	Summary      string
	PublicStatus string
}

type SearchScope struct {
	From         time.Time
	To           time.Time
	Label        string
	Category     string
	Terms        []string
	ExcludeWorld bool
	CategoryOnly bool
}

type MessageDraft struct {
	Content     string
	Citations   []Citation
	Suggestions []string
	Provider    string
	Model       string
	Search      SearchSummary
}

type Repository interface {
	CreateThread(context.Context, uuid.UUID, string) (Thread, error)
	ListThreads(context.Context, uuid.UUID) ([]Thread, error)
	Conversation(context.Context, uuid.UUID, uuid.UUID) (Conversation, error)
	SearchArticles(context.Context, SearchScope) ([]ArticleEvidence, error)
	SaveExchange(context.Context, uuid.UUID, uuid.UUID, string, MessageDraft) (Conversation, error)
	DeleteThread(context.Context, uuid.UUID, uuid.UUID) error
}
