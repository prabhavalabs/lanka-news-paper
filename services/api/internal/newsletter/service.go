package newsletter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RuntimeConfig struct {
	BaseURL  string
	Enabled  bool
	From     string
	Location *time.Location
	SendHour int
}

type Service struct {
	store  *Store
	sender Sender
	config RuntimeConfig
	now    func() time.Time
}

func NewService(store *Store, sender Sender, config RuntimeConfig, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, sender: sender, config: config, now: now}
}

func (service *Service) Enabled() bool {
	return service != nil && service.config.Enabled && service.sender != nil && service.store != nil
}

func (service *Service) Location() *time.Location {
	if service == nil || service.config.Location == nil {
		return time.UTC
	}
	return service.config.Location
}

func (service *Service) SendHour() int {
	if service == nil {
		return 8
	}
	return service.config.SendHour
}

func (service *Service) SendDaily(ctx context.Context) error {
	if !service.Enabled() {
		return nil
	}
	editionDate, start, end, due := deliveryWindow(service.now(), service.config.Location, service.config.SendHour)
	if !due {
		return nil
	}
	digest, err := service.store.BuildDigest(ctx, editionDate, start, end, service.config.BaseURL)
	if err != nil {
		return err
	}
	metadata, err := RenderEdition(digest, "", service.config.BaseURL)
	if err != nil {
		return err
	}
	edition, err := service.store.CreateOrLoadEdition(ctx, digest, metadata.Subject, metadata.Preheader)
	if err != nil {
		return err
	}
	subscribers, err := service.store.ActiveSubscribers(ctx)
	if err != nil {
		return err
	}
	deliveryErrors := make([]error, 0)
	for _, subscriber := range subscribers {
		claimed, claimErr := service.store.BeginDelivery(ctx, edition.ID, subscriber.ID)
		if claimErr != nil {
			deliveryErrors = append(deliveryErrors, claimErr)
			continue
		}
		if !claimed {
			continue
		}
		unsubscribeURL := fmt.Sprintf("%s/api/v1/newsletter/unsubscribe/%s", service.config.BaseURL, subscriber.UnsubscribeToken)
		rendered, renderErr := RenderEdition(edition.Digest, subscriber.Name, unsubscribeURL)
		if renderErr != nil {
			service.recordFailure(ctx, edition.ID, subscriber.ID, renderErr, &deliveryErrors)
			continue
		}
		messageID, sendErr := service.sender.Send(ctx, EmailMessage{
			From: service.config.From, To: subscriber.Email, Subject: rendered.Subject,
			HTML: rendered.HTML, Text: rendered.Text,
			Headers: map[string]string{
				"List-Unsubscribe":      "<" + unsubscribeURL + ">",
				"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			},
			IdempotencyKey: fmt.Sprintf("newsletter-%s-%s", editionDate, subscriber.ID),
		})
		if sendErr != nil {
			service.recordFailure(ctx, edition.ID, subscriber.ID, sendErr, &deliveryErrors)
			continue
		}
		if err := service.store.MarkDeliverySent(ctx, edition.ID, subscriber.ID, messageID); err != nil {
			deliveryErrors = append(deliveryErrors, err)
		}
	}
	return errors.Join(deliveryErrors...)
}

func (service *Service) recordFailure(ctx context.Context, editionID, subscriberID uuid.UUID, cause error, failures *[]error) {
	if err := service.store.MarkDeliveryFailed(ctx, editionID, subscriberID, cause); err != nil {
		*failures = append(*failures, errors.Join(cause, err))
		return
	}
	*failures = append(*failures, cause)
}
