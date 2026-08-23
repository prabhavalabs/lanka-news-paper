package ingest

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

type apiID string

func (id *apiID) UnmarshalJSON(value []byte) error {
	*id = apiID(strings.Trim(string(value), `"`))
	return nil
}

type renderedText struct {
	Rendered string `json:"rendered"`
}

type apiPost struct {
	ID          apiID        `json:"id"`
	Date        string       `json:"date"`
	DateGMT     string       `json:"date_gmt"`
	Modified    string       `json:"modified"`
	ModifiedGMT string       `json:"modified_gmt"`
	Link        string       `json:"link"`
	PostURL     string       `json:"post_url"`
	GUID        renderedText `json:"guid"`
	Title       renderedText `json:"title"`
	Excerpt     renderedText `json:"excerpt"`
	Content     renderedText `json:"content"`
}

type apiResponse struct {
	Latest   []apiPost `json:"latestPost"`
	Local    []apiPost `json:"localPost"`
	Featured []apiPost `json:"featuredPost"`
	Sport    []apiPost `json:"sportPost"`
	World    []apiPost `json:"worldPost"`
	Business []apiPost `json:"businessPost"`
	Sticky   struct {
		Groups []struct {
			Posts []apiPost `json:"postResponseDto"`
		} `json:"postResponseDto"`
	} `json:"stickyPost"`
}

func ParseEndpoint(endpointType, website string, body []byte) (*gofeed.Feed, error) {
	if endpointType != "rest_api" {
		return gofeed.NewParser().ParseString(string(body))
	}

	var posts []apiPost
	if err := json.Unmarshal(body, &posts); err != nil {
		var response apiResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("parse REST feed: %w", err)
		}
		posts = append(posts, response.Latest...)
		posts = append(posts, response.Local...)
		posts = append(posts, response.Featured...)
		posts = append(posts, response.Sport...)
		posts = append(posts, response.World...)
		posts = append(posts, response.Business...)
		for _, group := range response.Sticky.Groups {
			posts = append(posts, group.Posts...)
		}
	}
	if len(posts) == 0 {
		return nil, fmt.Errorf("REST feed contains no posts")
	}

	feed := &gofeed.Feed{}
	seen := make(map[apiID]bool, len(posts))
	for _, post := range posts {
		if post.ID == "" || seen[post.ID] {
			continue
		}
		seen[post.ID] = true
		link := post.Link
		if link == "" {
			base, baseErr := url.Parse(strings.TrimRight(website, "/") + "/")
			reference, referenceErr := url.Parse(post.PostURL)
			if baseErr == nil && referenceErr == nil {
				link = base.ResolveReference(reference).String()
			}
		}
		item := &gofeed.Item{
			GUID:        string(post.ID),
			Title:       html.UnescapeString(strings.TrimSpace(post.Title.Rendered)),
			Link:        link,
			Description: post.Excerpt.Rendered,
			Content:     post.Content.Rendered,
		}
		if item.GUID == "" {
			item.GUID = post.GUID.Rendered
		}
		item.PublishedParsed = parseAPITime(post.DateGMT, post.Date)
		item.UpdatedParsed = parseAPITime(post.ModifiedGMT, post.Modified)
		feed.Items = append(feed.Items, item)
	}
	if len(feed.Items) == 0 {
		return nil, fmt.Errorf("REST feed contains no identifiable posts")
	}
	return feed, nil
}

func parseAPITime(values ...string) *time.Time {
	for _, value := range values {
		if value == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return &parsed
		}
		if parsed, err := time.ParseInLocation("2006-01-02T15:04:05", value, time.UTC); err == nil {
			return &parsed
		}
	}
	return nil
}
