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

type Repository interface {
	ListSubscribers(ctx context.Context) (SubscriberList, error)
	GetSettings(ctx context.Context) (Settings, error)
	UpdateSettings(ctx context.Context, settings Settings, actor uuid.UUID) (Settings, error)
	CreateSubscriber(ctx context.Context, input SubscriberInput, actor uuid.UUID) (Subscriber, error)
	UpdateSubscriber(ctx context.Context, id uuid.UUID, input SubscriberInput, actor uuid.UUID) (Subscriber, error)
	DeleteSubscriber(ctx context.Context, id uuid.UUID, actor uuid.UUID) error
	Unsubscribe(ctx context.Context, token uuid.UUID) error
}
