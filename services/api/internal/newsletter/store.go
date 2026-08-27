package newsletter

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
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
		    updated_at = CASE
		      WHEN enabled IS DISTINCT FROM $1
		        OR timezone IS DISTINCT FROM $2
		        OR send_hour IS DISTINCT FROM $3
		      THEN clock_timestamp()
		      ELSE updated_at
		    END
		WHERE singleton
	`, settings.Enabled, settings.Timezone, settings.SendHour)
	if err != nil {
		return fmt.Errorf("sync newsletter settings: %w", err)
	}
	return nil
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
	settings := Settings{}
	if err := store.pool.QueryRow(ctx, `
		SELECT enabled, timezone, send_hour
		FROM newsletter_settings
		WHERE singleton
	`).Scan(&settings.Enabled, &settings.Timezone, &settings.SendHour); err != nil {
		return SubscriberList{}, fmt.Errorf("load newsletter settings: %w", err)
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
