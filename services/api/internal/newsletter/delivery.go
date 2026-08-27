package newsletter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Edition struct {
	ID        uuid.UUID
	Digest    Digest
	Subject   string
	Preheader string
}

func (store *Store) CreateOrLoadEdition(ctx context.Context, digest Digest, subject, preheader string) (Edition, error) {
	digestJSON, err := json.Marshal(digest)
	if err != nil {
		return Edition{}, fmt.Errorf("encode newsletter edition: %w", err)
	}
	var edition Edition
	var storedDigest []byte
	err = store.pool.QueryRow(ctx, `
		INSERT INTO newsletter_editions (
		  edition_date, window_start, window_end, subject, preheader, digest
		) VALUES ($1::date, $2, $3, $4, $5, $6)
		ON CONFLICT (edition_date) DO UPDATE SET edition_date = EXCLUDED.edition_date
		RETURNING id, subject, preheader, digest
	`, digest.EditionDate, digest.WindowStart, digest.WindowEnd, subject, preheader, digestJSON).Scan(
		&edition.ID, &edition.Subject, &edition.Preheader, &storedDigest,
	)
	if err != nil {
		return Edition{}, fmt.Errorf("save newsletter edition: %w", err)
	}
	if err := json.Unmarshal(storedDigest, &edition.Digest); err != nil {
		return Edition{}, fmt.Errorf("decode stored newsletter edition: %w", err)
	}
	return edition, nil
}

func (store *Store) BeginDelivery(ctx context.Context, editionID, subscriberID uuid.UUID) (bool, error) {
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO newsletter_deliveries (edition_id, subscriber_id)
		VALUES ($1, $2)
		ON CONFLICT (edition_id, subscriber_id) DO NOTHING
	`, editionID, subscriberID); err != nil {
		return false, fmt.Errorf("prepare newsletter delivery: %w", err)
	}
	var deliveryID uuid.UUID
	err := store.pool.QueryRow(ctx, `
		UPDATE newsletter_deliveries delivery
		SET status = 'sending', attempt_count = attempt_count + 1, last_error = NULL
		FROM newsletter_subscribers subscriber
		WHERE delivery.edition_id = $1
		  AND delivery.subscriber_id = $2
		  AND subscriber.id = delivery.subscriber_id
		  AND subscriber.status = 'active'
		  AND (
		    delivery.status IN ('pending', 'failed')
		    OR (delivery.status = 'sending' AND delivery.updated_at < clock_timestamp() - interval '15 minutes')
		  )
		RETURNING delivery.id
	`, editionID, subscriberID).Scan(&deliveryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("claim newsletter delivery: %w", err)
	}
	return true, nil
}

func (store *Store) MarkDeliverySent(ctx context.Context, editionID, subscriberID uuid.UUID, providerMessageID string) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE newsletter_deliveries
		SET status = 'sent', provider_message_id = $3, last_error = NULL,
		    sent_at = clock_timestamp()
		WHERE edition_id = $1 AND subscriber_id = $2
	`, editionID, subscriberID, providerMessageID)
	if err != nil {
		return fmt.Errorf("mark newsletter delivery sent: %w", err)
	}
	return nil
}

func (store *Store) MarkDeliveryFailed(ctx context.Context, editionID, subscriberID uuid.UUID, deliveryError error) error {
	detail := truncateText(deliveryError.Error(), 2000)
	_, err := store.pool.Exec(ctx, `
		UPDATE newsletter_deliveries
		SET status = 'failed', last_error = $3
		WHERE edition_id = $1 AND subscriber_id = $2
	`, editionID, subscriberID, detail)
	if err != nil {
		return fmt.Errorf("mark newsletter delivery failed: %w", err)
	}
	return nil
}

func deliveryWindow(now time.Time, location *time.Location, sendHour int) (string, time.Time, time.Time, bool) {
	localNow := now.In(location)
	windowEnd := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), sendHour, 0, 0, 0, location)
	if localNow.Before(windowEnd) {
		return "", time.Time{}, time.Time{}, false
	}
	return windowEnd.Format("2006-01-02"), windowEnd.AddDate(0, 0, -1), windowEnd, true
}
