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
)

func normalizeTestInput(input TestInput) (TestInput, error) {
	input.Mode = strings.ToLower(strings.TrimSpace(input.Mode))
	input.WindowMode = strings.ToLower(strings.TrimSpace(input.WindowMode))
	input.RecipientEmail = strings.ToLower(strings.TrimSpace(input.RecipientEmail))
	input.RecipientName = strings.TrimSpace(input.RecipientName)
	if input.Mode != "preview" && input.Mode != "send" {
		return TestInput{}, ErrInvalidTest
	}
	if input.WindowMode == "" {
		input.WindowMode = "latest_24h"
	}
	if input.WindowMode != "latest_24h" && input.WindowMode != "scheduled" {
		return TestInput{}, ErrInvalidTest
	}
	if utf8.RuneCountInString(input.RecipientName) > 160 {
		return TestInput{}, ErrInvalidTest
	}
	if input.Mode == "send" {
		address, err := mail.ParseAddress(input.RecipientEmail)
		if err != nil || address.Address != input.RecipientEmail || len(input.RecipientEmail) > 254 {
			return TestInput{}, ErrInvalidEmail
		}
	} else {
		input.RecipientEmail = ""
	}
	return input, nil
}

func testWindow(now time.Time, location *time.Location, sendHour int, mode string) (string, time.Time, time.Time) {
	localNow := now.In(location)
	end := localNow
	if mode == "scheduled" {
		end = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), sendHour, 0, 0, 0, location)
		if localNow.Before(end) {
			end = end.AddDate(0, 0, -1)
		}
	}
	return end.Format("2006-01-02"), end.Add(-24 * time.Hour), end
}

func (service *Service) RunTest(ctx context.Context, raw TestInput, actor uuid.UUID) (TestResult, error) {
	if service == nil || service.store == nil || service.model == nil {
		return TestResult{}, ErrTestSendDisabled
	}
	started := service.now()
	input, err := normalizeTestInput(raw)
	if err != nil {
		return TestResult{}, err
	}
	if input.Mode == "send" && (!service.config.TestSendReady || service.sender == nil || strings.TrimSpace(service.config.From) == "") {
		return TestResult{}, ErrTestSendDisabled
	}
	settings, err := service.store.GetSettings(ctx)
	if err != nil {
		return TestResult{}, err
	}
	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return TestResult{}, fmt.Errorf("load newsletter test timezone: %w", err)
	}
	editionDate, start, end := testWindow(service.now(), location, settings.SendHour, input.WindowMode)
	digest, err := service.store.BuildDigest(ctx, editionDate, start, end, service.config.BaseURL)
	if err != nil {
		return service.failedTest(ctx, input, actor, started, TestRun{}, err)
	}
	if len(digest.Stories) > settings.MaxStories {
		digest.Stories = digest.Stories[:settings.MaxStories]
	}
	modelContext, cancel := context.WithTimeout(ctx, 45*time.Second)
	digest, settings, outcome, err := applyEditorialPlanStrict(modelContext, service.model, digest, settings)
	cancel()
	run := TestRun{
		Mode: input.Mode, WindowMode: input.WindowMode, RecipientEmail: input.RecipientEmail,
		Provider: outcome.Provider, Model: outcome.Model, StoryCount: len(digest.Stories),
		ArticleCount: digest.ArticleCount, EventCount: digest.EventCount, SourceCount: digest.SourceCount,
	}
	if err != nil {
		return service.failedTest(ctx, input, actor, started, run, err)
	}
	rendered, err := RenderEditionWithSettings(digest, input.RecipientName, "", settings)
	if err != nil {
		return service.failedTest(ctx, input, actor, started, run, err)
	}
	run.Subject, run.Preheader = rendered.Subject, rendered.Preheader
	if input.Mode == "send" {
		run.Subject = "[TEST] " + run.Subject
		messageID, sendErr := service.sender.Send(ctx, EmailMessage{
			From: service.config.From, To: input.RecipientEmail, Subject: run.Subject,
			HTML: rendered.HTML, Text: rendered.Text,
			IdempotencyKey: "newsletter-test-" + uuid.NewString(),
		})
		if sendErr != nil {
			return service.failedTest(ctx, input, actor, started, run, sendErr)
		}
		run.ProviderMessageID = messageID
	}
	run.Status = "succeeded"
	run.DurationMS = elapsedMilliseconds(started, service.now())
	run, err = service.store.RecordTest(ctx, run, actor)
	if err != nil {
		return TestResult{}, err
	}
	return TestResult{TestRun: run, HTML: rendered.HTML, Text: rendered.Text}, nil
}

func (service *Service) failedTest(ctx context.Context, input TestInput, actor uuid.UUID, started time.Time, run TestRun, cause error) (TestResult, error) {
	run.Mode, run.WindowMode, run.RecipientEmail = input.Mode, input.WindowMode, input.RecipientEmail
	run.Status, run.DurationMS = "failed", elapsedMilliseconds(started, service.now())
	run.ErrorDetail = truncateText(cause.Error(), 2000)
	stored, recordErr := service.store.RecordTest(ctx, run, actor)
	if recordErr != nil {
		return TestResult{}, errors.Join(cause, recordErr)
	}
	return TestResult{TestRun: stored}, cause
}

func elapsedMilliseconds(start, end time.Time) int {
	value := end.Sub(start).Milliseconds()
	if value < 0 {
		return 0
	}
	return int(value)
}

func (store *Store) RecordTest(ctx context.Context, run TestRun, actor uuid.UUID) (TestRun, error) {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return TestRun{}, fmt.Errorf("begin newsletter test record: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	err = transaction.QueryRow(ctx, `
		INSERT INTO newsletter_test_runs (
		  mode, window_mode, status, recipient_email, provider_id, model, subject, preheader,
		  story_count, article_count, event_count, source_count, duration_ms,
		  provider_message_id, error_detail, created_by
		) VALUES ($1, $2, $3, NULLIF($4, '')::citext, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at, completed_at
	`, run.Mode, run.WindowMode, run.Status, run.RecipientEmail, run.Provider, run.Model,
		run.Subject, run.Preheader, run.StoryCount, run.ArticleCount, run.EventCount,
		run.SourceCount, run.DurationMS, run.ProviderMessageID, run.ErrorDetail, actor,
	).Scan(&run.ID, &run.CreatedAt, &run.CompletedAt)
	if err != nil {
		return TestRun{}, fmt.Errorf("record newsletter test: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, target_type, target_id, result)
		VALUES ($1, 'run_newsletter_test', 'newsletter_test_run', $2, $3)
	`, actor, run.ID.String(), run.Status); err != nil {
		return TestRun{}, fmt.Errorf("audit newsletter test: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return TestRun{}, fmt.Errorf("commit newsletter test record: %w", err)
	}
	return run, nil
}

func (store *Store) ListTests(ctx context.Context) ([]TestRun, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id, mode, window_mode, status, COALESCE(recipient_email::text, ''),
		       provider_id, model, subject, preheader, story_count, article_count,
		       event_count, source_count, duration_ms, provider_message_id, error_detail,
		       created_at, completed_at
		FROM newsletter_test_runs
		ORDER BY created_at DESC, id DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, fmt.Errorf("list newsletter tests: %w", err)
	}
	defer rows.Close()
	items := make([]TestRun, 0)
	for rows.Next() {
		var item TestRun
		if err := rows.Scan(&item.ID, &item.Mode, &item.WindowMode, &item.Status,
			&item.RecipientEmail, &item.Provider, &item.Model, &item.Subject, &item.Preheader,
			&item.StoryCount, &item.ArticleCount, &item.EventCount, &item.SourceCount,
			&item.DurationMS, &item.ProviderMessageID, &item.ErrorDetail,
			&item.CreatedAt, &item.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan newsletter test: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list newsletter tests: %w", err)
	}
	return items, nil
}
