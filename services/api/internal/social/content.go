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
