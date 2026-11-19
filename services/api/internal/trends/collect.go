package trends

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Collect refreshes statistics for the watched channels' uploads of the last 30 days and for
// YouTube's Sri Lanka charts. It does nothing without an API key.
func (radar *Radar) Collect(ctx context.Context) error {
	yt := radar.config.YouTube
	if yt.Key == "" {
		return nil
	}
	channels, err := radar.config.Store.channels(ctx)
	if err != nil {
		return err
	}
	since := time.Now().Add(-30 * 24 * time.Hour)
	var ids []string
	var failures []error
	for _, c := range channels {
		pages := 1
		if !c.Baselined {
			pages = 8
		}
		found, err := yt.recentUploads(ctx, c.ID, since, pages)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", c.Title, err))
			continue
		}
		ids = append(ids, found...)
	}
	videos, err := yt.videos(ctx, ids)
	if err != nil {
		return err
	}
	ranks := map[string]int{}
	for _, category := range []string{"", "25"} { // all videos, then News & Politics
		chart, err := yt.chart(ctx, category)
		if err != nil {
			failures = append(failures, fmt.Errorf("chart %q: %w", category, err))
			continue
		}
		for i, v := range chart {
			if _, seen := ranks[v.ID]; !seen {
				ranks[v.ID] = i + 1
				videos = append(videos, v)
			}
		}
	}
	videos = uniqueVideos(videos)
	if err := radar.config.Store.saveVideos(ctx, videos, ranks); err != nil {
		return err
	}
	slog.InfoContext(ctx, "trend videos collected", "channels", len(channels), "videos", len(videos), "charted", len(ranks))
	if len(failures) == len(channels)+2 {
		return errors.Join(failures...)
	}
	for _, failure := range failures {
		slog.WarnContext(ctx, "trend video source failed", "error", failure)
	}
	return nil
}

func uniqueVideos(videos []video) []video {
	seen := make(map[string]bool, len(videos))
	unique := videos[:0]
	for _, v := range videos {
		if !seen[v.ID] {
			seen[v.ID] = true
			unique = append(unique, v)
		}
	}
	return unique
}
