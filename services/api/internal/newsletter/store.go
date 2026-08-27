package newsletter

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (store *Store) SyncSettings(ctx context.Context, settings Settings) error {
	if strings.TrimSpace(settings.Timezone) == "" {
		return errors.New("newsletter timezone is required")
	}
	if settings.SendHour < 0 || settings.SendHour > 23 {
		return errors.New("newsletter send hour must be between 0 and 23")
	}
	_, err := store.pool.Exec(ctx, `
		UPDATE newsletter_settings
		SET enabled = $1, timezone = $2, send_hour = $3,
		    runtime_bootstrapped = true, updated_at = clock_timestamp()
		WHERE singleton AND NOT runtime_bootstrapped
	`, settings.Enabled, settings.Timezone, settings.SendHour)
	if err != nil {
		return fmt.Errorf("sync newsletter settings: %w", err)
	}
	return nil
}

func defaultSettings() Settings {
	return Settings{
		Timezone: "Asia/Colombo", SendHour: 8, MaxStories: 30, LeadStoryCount: 5,
		SubjectTemplate:   "උදෑසන පුවත් සංග්‍රහය — {{date}}",
		PreheaderTemplate: "පසුගිය පැය 24: පුවත් {{articles}} · සිදුවීම් {{events}} · මූලාශ්‍ර {{sources}}",
		IntroText:         "ඔබ දැනගත යුතු පසුගිය පැය 24 හි වැදගත් පුවත් මෙන්න.",
		FooterText:        "මෙම සංග්‍රහය ප්‍රකාශිත පුවත් සහ මූලාශ්‍ර-අතර සාරාංශ මත ස්වයංක්‍රීයව සකස් කර ඇත. සම්පූර්ණ විස්තර සඳහා සබැඳි විවෘත කරන්න.",
	}
}

func validateSettings(settings Settings) (Settings, error) {
	settings.Timezone = strings.TrimSpace(settings.Timezone)
	settings.SubjectTemplate = strings.TrimSpace(settings.SubjectTemplate)
	settings.PreheaderTemplate = strings.TrimSpace(settings.PreheaderTemplate)
	settings.IntroText = strings.TrimSpace(settings.IntroText)
	settings.FooterText = strings.TrimSpace(settings.FooterText)
	if _, err := time.LoadLocation(settings.Timezone); err != nil {
		return Settings{}, ErrInvalidSettings
	}
	if settings.SendHour < 0 || settings.SendHour > 23 ||
		settings.MaxStories < 1 || settings.MaxStories > 50 ||
		settings.LeadStoryCount < 1 || settings.LeadStoryCount > 10 ||
		settings.LeadStoryCount > settings.MaxStories ||
		utf8.RuneCountInString(settings.SubjectTemplate) < 1 || utf8.RuneCountInString(settings.SubjectTemplate) > 240 ||
		utf8.RuneCountInString(settings.PreheaderTemplate) < 1 || utf8.RuneCountInString(settings.PreheaderTemplate) > 500 ||
		utf8.RuneCountInString(settings.IntroText) > 1200 || utf8.RuneCountInString(settings.FooterText) > 2000 {
		return Settings{}, ErrInvalidSettings
	}
	return settings, nil
}

func scanSettings(scanner subscriberScanner) (Settings, error) {
	var settings Settings
	err := scanner.Scan(
		&settings.Enabled, &settings.Timezone, &settings.SendHour,
		&settings.MaxStories, &settings.LeadStoryCount, &settings.SubjectTemplate,
		&settings.PreheaderTemplate, &settings.IntroText, &settings.FooterText, &settings.UpdatedAt,
	)
	return settings, err
}

func (store *Store) GetSettings(ctx context.Context) (Settings, error) {
	settings, err := scanSettings(store.pool.QueryRow(ctx, `
		SELECT enabled, timezone, send_hour, max_stories, lead_story_count,
		       subject_template, preheader_template, intro_text, footer_text, updated_at
		FROM newsletter_settings WHERE singleton
	`))
	if err != nil {
		return Settings{}, fmt.Errorf("load newsletter settings: %w", err)
	}
	return settings, nil
}

func (store *Store) UpdateSettings(ctx context.Context, settings Settings, actor uuid.UUID) (Settings, error) {
	settings, err := validateSettings(settings)
	if err != nil {
		return Settings{}, err
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return Settings{}, fmt.Errorf("begin newsletter settings update: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	settings, err = scanSettings(transaction.QueryRow(ctx, `
		UPDATE newsletter_settings
		SET enabled = $1, timezone = $2, send_hour = $3, max_stories = $4,
		    lead_story_count = $5, subject_template = $6, preheader_template = $7,
		    intro_text = $8, footer_text = $9, runtime_bootstrapped = true,
		    updated_by = $10, updated_at = clock_timestamp()
		WHERE singleton
		RETURNING enabled, timezone, send_hour, max_stories, lead_story_count,
		          subject_template, preheader_template, intro_text, footer_text, updated_at
	`, settings.Enabled, settings.Timezone, settings.SendHour, settings.MaxStories,
		settings.LeadStoryCount, settings.SubjectTemplate, settings.PreheaderTemplate,
		settings.IntroText, settings.FooterText, actor))
	if err != nil {
		return Settings{}, fmt.Errorf("update newsletter settings: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target_type, target_id, result)
		VALUES ($1, 'update_newsletter_settings', 'newsletter_settings', 'daily', 'ok')
	`, actor); err != nil {
		return Settings{}, fmt.Errorf("audit newsletter settings: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return Settings{}, fmt.Errorf("commit newsletter settings update: %w", err)
	}
	return settings, nil
}

func normalizeSubscriberInput(input SubscriberInput, requireConsent bool) (SubscriberInput, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = StatusActive
	}
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 254 {
		return SubscriberInput{}, ErrInvalidEmail
	}
	if utf8.RuneCountInString(input.Name) > 160 {
		return SubscriberInput{}, ErrInvalidName
	}
	if input.Status != StatusActive && input.Status != StatusPaused && input.Status != StatusUnsubscribed {
		return SubscriberInput{}, ErrInvalidStatus
	}
	if requireConsent && !input.ConsentConfirmed {
		return SubscriberInput{}, ErrConsentRequired
	}
	return input, nil
}

func validateStatusTransition(current string, input SubscriberInput) error {
	if current == StatusUnsubscribed && input.Status != StatusUnsubscribed && !input.ConsentConfirmed {
		return ErrConsentRequired
	}
	return nil
}

func (store *Store) ListSubscribers(ctx context.Context) (SubscriberList, error) {
	settings, err := store.GetSettings(ctx)
	if err != nil {
		return SubscriberList{}, err
	}
	rows, err := store.pool.Query(ctx, `
		SELECT id, email::text, name, status, consent_source, consented_at,
		       unsubscribe_token, created_at, updated_at
		FROM newsletter_subscribers
		ORDER BY CASE status WHEN 'active' THEN 0 WHEN 'paused' THEN 1 ELSE 2 END,
		         created_at DESC, id DESC
	`)
	if err != nil {
		return SubscriberList{}, fmt.Errorf("list newsletter recipients: %w", err)
	}
	defer rows.Close()
	result := SubscriberList{Items: make([]Subscriber, 0), Settings: settings}
	for rows.Next() {
		subscriber, scanErr := scanSubscriber(rows)
		if scanErr != nil {
			return SubscriberList{}, scanErr
		}
		result.Items = append(result.Items, subscriber)
		result.Summary.Total++
		switch subscriber.Status {
		case StatusActive:
			result.Summary.Active++
		case StatusPaused:
			result.Summary.Paused++
		case StatusUnsubscribed:
			result.Summary.Unsubscribed++
		}
	}
	if err := rows.Err(); err != nil {
		return SubscriberList{}, fmt.Errorf("list newsletter recipients: %w", err)
	}
	return result, nil
}

func (store *Store) CreateSubscriber(ctx context.Context, input SubscriberInput, actor uuid.UUID) (Subscriber, error) {
	input, err := normalizeSubscriberInput(input, true)
	if err != nil {
		return Subscriber{}, err
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return Subscriber{}, fmt.Errorf("begin create newsletter recipient: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	row := transaction.QueryRow(ctx, `
		INSERT INTO newsletter_subscribers (
		  email, name, status, consent_source, created_by, updated_by
		) VALUES ($1, $2, $3, 'admin_confirmed', $4, $4)
		RETURNING id, email::text, name, status, consent_source, consented_at,
		          unsubscribe_token, created_at, updated_at
	`, input.Email, input.Name, input.Status, actor)
	subscriber, err := scanSubscriber(row)
	if err != nil {
		return Subscriber{}, subscriberWriteError("create newsletter recipient", err)
	}
	if err := auditSubscriber(ctx, transaction, actor, "create_newsletter_recipient", subscriber.ID); err != nil {
		return Subscriber{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Subscriber{}, fmt.Errorf("commit create newsletter recipient: %w", err)
	}
	return subscriber, nil
}

func (store *Store) UpdateSubscriber(ctx context.Context, id uuid.UUID, input SubscriberInput, actor uuid.UUID) (Subscriber, error) {
	input, err := normalizeSubscriberInput(input, false)
	if err != nil {
		return Subscriber{}, err
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return Subscriber{}, fmt.Errorf("begin update newsletter recipient: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var currentStatus string
	if err := transaction.QueryRow(ctx, `
		SELECT status
		FROM newsletter_subscribers
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&currentStatus); errors.Is(err, pgx.ErrNoRows) {
		return Subscriber{}, ErrSubscriberMissing
	} else if err != nil {
		return Subscriber{}, fmt.Errorf("lock newsletter recipient: %w", err)
	}
	if err := validateStatusTransition(currentStatus, input); err != nil {
		return Subscriber{}, err
	}
	row := transaction.QueryRow(ctx, `
		UPDATE newsletter_subscribers
		SET email = $2, name = $3, status = $4, updated_by = $5,
		    consent_source = CASE
		      WHEN status = 'unsubscribed' AND $4 <> 'unsubscribed' THEN 'admin_reconfirmed'
		      ELSE consent_source
		    END,
		    consented_at = CASE
		      WHEN status = 'unsubscribed' AND $4 <> 'unsubscribed' THEN clock_timestamp()
		      ELSE consented_at
		    END,
		    unsubscribe_token = CASE
		      WHEN status = 'unsubscribed' AND $4 <> 'unsubscribed' THEN gen_random_uuid()
		      ELSE unsubscribe_token
		    END
		WHERE id = $1
		RETURNING id, email::text, name, status, consent_source, consented_at,
		          unsubscribe_token, created_at, updated_at
	`, id, input.Email, input.Name, input.Status, actor)
	subscriber, err := scanSubscriber(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Subscriber{}, ErrSubscriberMissing
	}
	if err != nil {
		return Subscriber{}, subscriberWriteError("update newsletter recipient", err)
	}
	if err := auditSubscriber(ctx, transaction, actor, "update_newsletter_recipient", subscriber.ID); err != nil {
		return Subscriber{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Subscriber{}, fmt.Errorf("commit update newsletter recipient: %w", err)
	}
	return subscriber, nil
}

func (store *Store) DeleteSubscriber(ctx context.Context, id uuid.UUID, actor uuid.UUID) error {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete newsletter recipient: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	command, err := transaction.Exec(ctx, `DELETE FROM newsletter_subscribers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete newsletter recipient: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrSubscriberMissing
	}
	if err := auditSubscriber(ctx, transaction, actor, "delete_newsletter_recipient", id); err != nil {
		return err
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete newsletter recipient: %w", err)
	}
	return nil
}

func (store *Store) ImportConfiguredRecipient(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	input, err := normalizeSubscriberInput(SubscriberInput{
		Email: email, Status: StatusActive, ConsentConfirmed: true,
	}, true)
	if err != nil {
		return fmt.Errorf("import configured newsletter recipient: %w", err)
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin configured newsletter recipient import: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var imported bool
	if err := transaction.QueryRow(ctx, `
		SELECT configured_recipient_imported
		FROM newsletter_settings
		WHERE singleton
		FOR UPDATE
	`).Scan(&imported); err != nil {
		return fmt.Errorf("lock newsletter settings: %w", err)
	}
	if imported {
		return transaction.Commit(ctx)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO newsletter_subscribers (email, status, consent_source)
		VALUES ($1, 'active', 'configuration')
		ON CONFLICT (email) DO NOTHING
	`, input.Email); err != nil {
		return fmt.Errorf("import configured newsletter recipient: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		UPDATE newsletter_settings
		SET configured_recipient_imported = true, updated_at = clock_timestamp()
		WHERE singleton
	`); err != nil {
		return fmt.Errorf("mark configured newsletter recipient imported: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit configured newsletter recipient import: %w", err)
	}
	return nil
}

func (store *Store) ActiveSubscribers(ctx context.Context) ([]Subscriber, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id, email::text, name, status, consent_source, consented_at,
		       unsubscribe_token, created_at, updated_at
		FROM newsletter_subscribers
		WHERE status = 'active'
		ORDER BY created_at, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list active newsletter recipients: %w", err)
	}
	defer rows.Close()
	items := make([]Subscriber, 0)
	for rows.Next() {
		item, scanErr := scanSubscriber(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active newsletter recipients: %w", err)
	}
	return items, nil
}

func (store *Store) Unsubscribe(ctx context.Context, token uuid.UUID) error {
	command, err := store.pool.Exec(ctx, `
		UPDATE newsletter_subscribers
		SET status = 'unsubscribed'
		WHERE unsubscribe_token = $1 AND status <> 'unsubscribed'
	`, token)
	if err != nil {
		return fmt.Errorf("unsubscribe newsletter recipient: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrSubscriberMissing
	}
	return nil
}

type subscriberScanner interface {
	Scan(dest ...any) error
}

func scanSubscriber(scanner subscriberScanner) (Subscriber, error) {
	var subscriber Subscriber
	if err := scanner.Scan(
		&subscriber.ID, &subscriber.Email, &subscriber.Name, &subscriber.Status,
		&subscriber.ConsentSource, &subscriber.ConsentedAt, &subscriber.UnsubscribeToken,
		&subscriber.CreatedAt, &subscriber.UpdatedAt,
	); err != nil {
		return Subscriber{}, err
	}
	return subscriber, nil
}

func subscriberWriteError(action string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return ErrEmailExists
	}
	return fmt.Errorf("%s: %w", action, err)
}

func auditSubscriber(ctx context.Context, transaction pgx.Tx, actor uuid.UUID, action string, subscriberID uuid.UUID) error {
	if _, err := transaction.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target_type, target_id, result)
		VALUES ($1, $2, 'newsletter_subscriber', $3, 'ok')
	`, actor, action, subscriberID.String()); err != nil {
		return fmt.Errorf("audit newsletter recipient change: %w", err)
	}
	return nil
}
