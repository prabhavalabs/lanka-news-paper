// Package trends collects outside signals of what Sri Lanka is watching, searching and
// discussing, and merges them with the newsroom's storylines into a ranked trend radar.
package trends

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// YouTube reads public video statistics with a YouTube Data API v3 key.
type YouTube struct {
	Key  string
	HTTP *http.Client
}

type video struct {
	ID, ChannelID, ChannelTitle, Title string
	PublishedAt                        time.Time
	Views, Likes, Comments             int64
}

const youtubeAPI = "https://www.googleapis.com/youtube/v3/"

func (yt YouTube) get(ctx context.Context, path string, query url.Values, into any) error {
	query.Set("key", yt.Key)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeAPI+path+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	response, err := yt.HTTP.Do(request)
	if err != nil {
		// The URL carries the key; never let it reach logs.
		return fmt.Errorf("YouTube %s request failed", path)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		var problem struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &problem)
		return fmt.Errorf("YouTube %s: %s %s", path, response.Status, problem.Error.Message)
	}
	return json.Unmarshal(body, into)
}

// recentUploads lists a channel's uploads published since the cutoff, newest first, reading up to
// pages pages of 50 (one page an hour keeps up; more the first time builds the baseline).
func (yt YouTube) recentUploads(ctx context.Context, channelID string, since time.Time, pages int) ([]string, error) {
	uploads := "UU" + strings.TrimPrefix(channelID, "UC")
	var ids []string
	token := ""
	for page := 0; page < pages; page++ {
		var list struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ContentDetails struct {
					VideoID          string    `json:"videoId"`
					VideoPublishedAt time.Time `json:"videoPublishedAt"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		query := url.Values{"part": {"contentDetails"}, "playlistId": {uploads}, "maxResults": {"50"}}
		if token != "" {
			query.Set("pageToken", token)
		}
		if err := yt.get(ctx, "playlistItems", query, &list); err != nil {
			return nil, err
		}
		reachedCutoff := false
		for _, item := range list.Items {
			if item.ContentDetails.VideoPublishedAt.Before(since) {
				reachedCutoff = true
				continue
			}
			if item.ContentDetails.VideoID != "" {
				ids = append(ids, item.ContentDetails.VideoID)
			}
		}
		if token = list.NextPageToken; token == "" || reachedCutoff {
			break
		}
	}
	return ids, nil
}

type videoList struct {
	Items []struct {
		ID      string `json:"id"`
		Snippet struct {
			ChannelID    string    `json:"channelId"`
			ChannelTitle string    `json:"channelTitle"`
			Title        string    `json:"title"`
			PublishedAt  time.Time `json:"publishedAt"`
		} `json:"snippet"`
		Statistics struct {
			Views    string `json:"viewCount"`
			Likes    string `json:"likeCount"`
			Comments string `json:"commentCount"`
		} `json:"statistics"`
	} `json:"items"`
}

func (list videoList) videos() []video {
	count := func(value string) int64 { n, _ := strconv.ParseInt(value, 10, 64); return n }
	videos := make([]video, 0, len(list.Items))
	for _, item := range list.Items {
		videos = append(videos, video{
			ID: item.ID, ChannelID: item.Snippet.ChannelID, ChannelTitle: item.Snippet.ChannelTitle, Title: item.Snippet.Title,
			PublishedAt: item.Snippet.PublishedAt, Views: count(item.Statistics.Views), Likes: count(item.Statistics.Likes),
			Comments: count(item.Statistics.Comments),
		})
	}
	return videos
}

// videos fetches statistics for up to 50 ids per request.
func (yt YouTube) videos(ctx context.Context, ids []string) ([]video, error) {
	var all []video
	for start := 0; start < len(ids); start += 50 {
		var list videoList
		if err := yt.get(ctx, "videos", url.Values{"part": {"snippet,statistics"}, "id": {strings.Join(ids[start:min(start+50, len(ids))], ",")}}, &list); err != nil {
			return nil, err
		}
		all = append(all, list.videos()...)
	}
	return all, nil
}

// chart returns YouTube's most popular videos in Sri Lanka, optionally for one category.
func (yt YouTube) chart(ctx context.Context, category string) ([]video, error) {
	query := url.Values{"part": {"snippet,statistics"}, "chart": {"mostPopular"}, "regionCode": {"LK"}, "maxResults": {"50"}}
	if category != "" {
		query.Set("videoCategoryId", category)
	}
	var list videoList
	if err := yt.get(ctx, "videos", query, &list); err != nil {
		return nil, err
	}
	return list.videos(), nil
}

// VideoSignal is a recent video with how far it is running ahead of its channel's normal.
type VideoSignal struct {
	Key       string  `json:"key"`
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Channel   string  `json:"channel"`
	Views     int64   `json:"views"`
	AgeHours  float64 `json:"age_hours"`
	Baseline  float64 `json:"channel_median_views"`
	Outlier   float64 `json:"outlier"`
	ChartRank *int    `json:"chart_rank,omitempty"`
	URL       string  `json:"url"`
}

// outlierRatio compares a video's views with what its channel usually reaches by that age. Most
// news videos get the bulk of their views in the first day, so the channel's 30-day median is
// scaled down for younger videos (a gentle curve that never drops below a twentieth).
func outlierRatio(views int64, ageHours, channelMedian float64) float64 {
	if channelMedian <= 0 {
		return 0
	}
	age := math.Max(ageHours, 1)
	expected := channelMedian * math.Max(math.Pow(math.Min(1, age/24), 0.6), 0.05)
	return float64(views) / expected
}
