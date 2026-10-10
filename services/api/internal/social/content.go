package social

import (
	"context"
	"time"
)

// Channel describes a publishing destination. Only Facebook publishes today; the others are
// listed so editors can see what is planned and the calendar can grow without schema changes.
type Channel struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Status    string `json:"status"` // connected, needs_reconnect, disconnected or planned
}

func (service *Service) Channels(ctx context.Context) ([]Channel, error) {
	status, err := service.Status(ctx)
	if err != nil {
		return nil, err
	}
	facebook := "disconnected"
	if status.Account != nil {
		facebook = status.Account.Status
	}
	return []Channel{
		{Key: "facebook", Name: "Facebook Page", Available: true, Status: facebook},
		{Key: "instagram", Name: "Instagram", Status: "planned"},
		{Key: "x", Name: "X (Twitter)", Status: "planned"},
		{Key: "whatsapp", Name: "WhatsApp Channel", Status: "planned"},
		{Key: "telegram", Name: "Telegram", Status: "planned"},
	}, nil
}

type ContentList struct {
	Items []Post    `json:"items"`
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
}

const maxContentItems = 1000

// Content returns the posts whose calendar time (published, else scheduled, else created)
// falls in [from, to), newest first, for the content calendar and log.
func (store *Store) Content(ctx context.Context, from, to time.Time, channel, status string) (ContentList, error) {
	result := ContentList{Items: make([]Post, 0), From: from, To: to}
	if from.IsZero() || to.IsZero() || !to.After(from) || to.Sub(from) > 400*24*time.Hour {
		return result, ErrInvalid
	}
	if status != "" && !validStatus(status) {
		return result, ErrInvalid
	}
	if channel != "" && channel != "facebook" {
		return result, nil
	}
	rows, err := store.pool.Query(ctx, postSelect+`WHERE COALESCE(p.published_at,p.scheduled_for,p.created_at) >= $1
 AND COALESCE(p.published_at,p.scheduled_for,p.created_at) < $2 AND ($3='' OR p.channel=$3) AND ($4='' OR p.status=$4)
 ORDER BY COALESCE(p.published_at,p.scheduled_for,p.created_at) DESC, p.id DESC LIMIT $5`, from, to, channel, status, maxContentItems)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, p)
	}
	return result, rows.Err()
}

// Content returns the calendar items with a publish plan on each.
func (service *Service) Content(ctx context.Context, from, to time.Time, channel, status string) (ContentList, error) {
	result, err := service.store.Content(ctx, from, to, channel, status)
	if err != nil {
		return result, err
	}
	return result, service.annotatePlans(ctx, result.Items)
}

// ListPosts returns a page of the review queue with a publish plan on each post.
func (service *Service) ListPosts(ctx context.Context, status string, limit int, cursor string) (PostList, error) {
	result, err := service.store.ListPosts(ctx, status, limit, cursor)
	if err != nil {
		return result, err
	}
	return result, service.annotatePlans(ctx, result.Items)
}

// annotatePlans explains for each post whether and when it will be published. Times for
// approved posts are estimated by replaying the publisher's queue order against the caps,
// minimum gap and quiet hours.
func (service *Service) annotatePlans(ctx context.Context, posts []Post) error {
	if len(posts) == 0 {
		return nil
	}
	now := service.now()
	settings, err := service.store.GetSettings(ctx)
	if err != nil {
		return err
	}
	account, err := service.store.Account(ctx)
	if err != nil {
		return err
	}
	connected := service.config.Configured() && account != nil && account.Status == "connected"
	estimates := map[string]time.Time{}
	if settings.Enabled && connected {
		history, err := service.store.publishHistory(ctx, now)
		if err != nil {
			return err
		}
		var retryAt *time.Time
		if err = service.store.pool.QueryRow(ctx, `SELECT (SELECT publish_retry_at FROM social_accounts WHERE status='connected')`).Scan(&retryAt); err != nil {
			return err
		}
		rows, err := service.store.pool.Query(ctx, `SELECT id::text,COALESCE(scheduled_for,$1) FROM social_posts WHERE status='approved'
 ORDER BY kind='breaking' DESC,created_at,id LIMIT 200`, now)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var start time.Time
			if err := rows.Scan(&id, &start); err != nil {
				return err
			}
			if start.Before(now) {
				start = now
			}
			if retryAt != nil && retryAt.After(start) {
				start = *retryAt
			}
			at, err := NextEligible(start, settings, history)
			if err != nil {
				return err
			}
			estimates[id] = at
			history = append(history, at)
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}
	for i := range posts {
		plan := planFor(posts[i], settings, connected, estimates)
		posts[i].Plan = &plan
	}
	return nil
}

func planFor(p Post, settings Settings, connected bool, estimates map[string]time.Time) PublishPlan {
	lastError := ""
	if p.LastError != nil {
		lastError = *p.LastError
	}
	switch p.Status {
	case "published":
		return PublishPlan{State: "published", At: p.PublishedAt, Note: "Published on the Page."}
	case "publishing":
		return PublishPlan{State: "publishing", Note: "Uploading to Facebook now."}
	case "approved":
		if !settings.Enabled {
			return PublishPlan{State: "paused", Note: "Approved, but publishing is turned off in the settings. It goes out after publishing is enabled."}
		}
		if !connected {
			return PublishPlan{State: "paused", Note: "Approved, but no Facebook Page is connected. It goes out after the Page is connected."}
		}
		if at, ok := estimates[p.ID]; ok {
			return PublishPlan{State: "scheduled", At: &at, Estimated: true, Note: "Expected time: the next free slot after the hourly and daily limits, the minimum gap and quiet hours. Breaking news goes first."}
		}
		return PublishPlan{State: "scheduled", At: p.ScheduledFor, Estimated: true, Note: "Queued for the next free publishing slot."}
	case "pending_review":
		note := "Waiting for an editor to approve. It is not published until then."
		switch {
		case p.ReviewStatus == "failed":
			note = "Will not publish automatically: the AI review found problems that must be fixed first. Edit the post and run the review again, or approve it yourself if the issues are acceptable."
		case p.ReviewStatus == "unavailable":
			note = "Will not publish automatically: the AI review could not run. Run the review again, or approve it yourself."
		case p.ReviewStatus == "passed" && p.HoldReason != nil && *p.HoldReason != "":
			note = *p.HoldReason
		case p.ReviewStatus == "passed" && !settings.AutoPublish:
			note = "Passed the AI review. Waiting for an editor because auto-publish is off."
		case p.ReviewStatus == "passed":
			note = "Passed the AI review. Waiting for an editor to approve."
		}
		return PublishPlan{State: "needs_editor", Note: note}
	case "skipped":
		if lastError != "" {
			return PublishPlan{State: "not_publishing", Note: lastError}
		}
		return PublishPlan{State: "not_publishing", Note: "Not selected for the Page: " + p.SelectionReason}
	case "rejected":
		return PublishPlan{State: "not_publishing", Note: "Rejected by an editor; it will not be published."}
	case "failed":
		return PublishPlan{State: "not_publishing", Note: "Publishing failed: " + lastError + " Retry it from the review queue."}
	case "needs_attention":
		return PublishPlan{State: "not_publishing", Note: "The outcome of the upload is unknown. Check the Page before retrying: " + lastError}
	}
	return PublishPlan{State: "not_publishing"}
}
