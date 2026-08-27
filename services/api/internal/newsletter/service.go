package newsletter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
)

type RuntimeConfig struct {
	BaseURL string
	From    string
}

type Service struct {
	store  *Store
	sender Sender
	model  llm.Completer
	config RuntimeConfig
	now    func() time.Time
}

func NewService(store *Store, sender Sender, model llm.Completer, config RuntimeConfig, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: store, sender: sender, model: model, config: config, now: now}
}

func (service *Service) Enabled() bool {
	return service != nil && service.sender != nil && service.store != nil
}

func (service *Service) SendDaily(ctx context.Context) error {
	if !service.Enabled() {
		return nil
	}
	settings, err := service.store.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}
	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return fmt.Errorf("load newsletter timezone: %w", err)
	}
	editionDate, start, end, due := deliveryWindow(service.now(), location, settings.SendHour)
	if !due {
		return nil
	}
	edition, exists, err := service.store.LoadEdition(ctx, editionDate)
	if err != nil {
		return err
	}
	if !exists {
		digest, buildErr := service.store.BuildDigest(ctx, editionDate, start, end, service.config.BaseURL)
		if buildErr != nil {
			return buildErr
		}
		if len(digest.Stories) > settings.MaxStories {
			digest.Stories = digest.Stories[:settings.MaxStories]
		}
		digest, settings = applyEditorialPlan(ctx, service.model, digest, settings)
		metadata, renderErr := RenderEditionWithSettings(digest, "", service.config.BaseURL, settings)
		if renderErr != nil {
			return renderErr
		}
		edition, err = service.store.CreateOrLoadEdition(ctx, digest, metadata.Subject, metadata.Preheader)
		if err != nil {
			return err
		}
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
		deliverySettings := settings
		deliverySettings.SubjectTemplate = edition.Subject
		deliverySettings.PreheaderTemplate = edition.Preheader
		rendered, renderErr := RenderEditionWithSettings(edition.Digest, subscriber.Name, unsubscribeURL, deliverySettings)
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
