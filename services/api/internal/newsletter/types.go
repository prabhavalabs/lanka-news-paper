package newsletter

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrConsentRequired   = errors.New("recipient consent must be confirmed")
	ErrEmailExists       = errors.New("a newsletter recipient with this email already exists")
	ErrInvalidEmail      = errors.New("a valid recipient email is required")
	ErrInvalidName       = errors.New("recipient name must be 160 characters or fewer")
	ErrInvalidStatus     = errors.New("newsletter recipient status is invalid")
	ErrInvalidSettings   = errors.New("newsletter settings are invalid")
	ErrInvalidTest       = errors.New("newsletter test settings are invalid")
	ErrInactiveTestEmail = errors.New("paused or unsubscribed recipients cannot receive test emails")
	ErrTestSendDisabled  = errors.New("newsletter test sending is not configured")
	ErrSubscriberMissing = errors.New("newsletter recipient was not found")
)

const (
	StatusActive       = "active"
	StatusPaused       = "paused"
	StatusUnsubscribed = "unsubscribed"
)

type Subscriber struct {
	ID               uuid.UUID `json:"id"`
	Email            string    `json:"email"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	ConsentSource    string    `json:"consent_source"`
	ConsentedAt      time.Time `json:"consented_at"`
	UnsubscribeToken uuid.UUID `json:"-"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type SubscriberInput struct {
	Email            string `json:"email"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	ConsentConfirmed bool   `json:"consent_confirmed"`
}

type SubscriberSummary struct {
	Total        int `json:"total"`
	Active       int `json:"active"`
	Paused       int `json:"paused"`
	Unsubscribed int `json:"unsubscribed"`
}

type Settings struct {
	Enabled           bool      `json:"enabled"`
	Timezone          string    `json:"timezone"`
	SendHour          int       `json:"send_hour"`
	MaxStories        int       `json:"max_stories"`
	LeadStoryCount    int       `json:"lead_story_count"`
	SubjectTemplate   string    `json:"subject_template"`
	PreheaderTemplate string    `json:"preheader_template"`
	IntroText         string    `json:"intro_text"`
	FooterText        string    `json:"footer_text"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type SubscriberList struct {
	Items    []Subscriber      `json:"items"`
	Summary  SubscriberSummary `json:"summary"`
	Settings Settings          `json:"settings"`
}

type TestInput struct {
	Mode           string `json:"mode"`
	WindowMode     string `json:"window_mode"`
	RecipientEmail string `json:"recipient_email"`
	RecipientName  string `json:"recipient_name"`
}

type TestRun struct {
	ID                uuid.UUID `json:"id"`
	Mode              string    `json:"mode"`
	WindowMode        string    `json:"window_mode"`
	Status            string    `json:"status"`
	RecipientEmail    string    `json:"recipient_email,omitempty"`
	Provider          string    `json:"provider_id"`
	Model             string    `json:"model"`
	Subject           string    `json:"subject"`
	Preheader         string    `json:"preheader"`
	StoryCount        int       `json:"story_count"`
	ArticleCount      int       `json:"article_count"`
	EventCount        int       `json:"event_count"`
	SourceCount       int       `json:"source_count"`
	DurationMS        int       `json:"duration_ms"`
	ProviderMessageID string    `json:"provider_message_id,omitempty"`
	ErrorDetail       string    `json:"error_detail,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	CompletedAt       time.Time `json:"completed_at"`
}

type TestResult struct {
	TestRun
	HTML string `json:"html"`
	Text string `json:"text"`
}

type Tester interface {
	RunTest(context.Context, TestInput, uuid.UUID) (TestResult, error)
	ListTests(context.Context) ([]TestRun, error)
}

type Repository interface {
	ListSubscribers(ctx context.Context) (SubscriberList, error)
	GetSettings(ctx context.Context) (Settings, error)
	UpdateSettings(ctx context.Context, settings Settings, actor uuid.UUID) (Settings, error)
	CreateSubscriber(ctx context.Context, input SubscriberInput, actor uuid.UUID) (Subscriber, error)
	UpdateSubscriber(ctx context.Context, id uuid.UUID, input SubscriberInput, actor uuid.UUID) (Subscriber, error)
	DeleteSubscriber(ctx context.Context, id uuid.UUID, actor uuid.UUID) error
	Unsubscribe(ctx context.Context, token uuid.UUID) error
}
