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

type hiruAPIPost struct {
	ArticleID apiID  `json:"sinhala_art_id"`
	Title     string `json:"sinhala_title"`
	Story     string `json:"sinhala_story"`
	AddedDate string `json:"sinhala_added_date"`
	PostURL   string `json:"seourltitle"`
}

func ParseEndpoint(endpointType, website string, body []byte) (*gofeed.Feed, error) {
	if endpointType != "rest_api" {
		return gofeed.NewParser().ParseString(string(body))
	}

	var hiruPosts []hiruAPIPost
	if err := json.Unmarshal(body, &hiruPosts); err == nil && containsHiruPost(hiruPosts) {
		return parseHiruPosts(website, hiruPosts)
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

func containsHiruPost(posts []hiruAPIPost) bool {
	for _, post := range posts {
		if post.ArticleID != "" || strings.TrimSpace(post.Title) != "" || strings.TrimSpace(post.Story) != "" {
			return true
		}
	}
	return false
}

func parseHiruPosts(website string, posts []hiruAPIPost) (*gofeed.Feed, error) {
	base, err := url.Parse(strings.TrimRight(website, "/") + "/")
	if err != nil {
		return nil, fmt.Errorf("parse Hiru website URL: %w", err)
	}

	feed := &gofeed.Feed{}
	seen := make(map[apiID]bool, len(posts))
	for _, post := range posts {
		if post.ArticleID == "" || seen[post.ArticleID] {
			continue
		}
		seen[post.ArticleID] = true
		reference, err := url.Parse(strings.TrimSpace(post.PostURL))
		if err != nil || reference.String() == "" {
			continue
		}
		feed.Items = append(feed.Items, &gofeed.Item{
			GUID:            string(post.ArticleID),
			Title:           html.UnescapeString(strings.TrimSpace(post.Title)),
			Link:            base.ResolveReference(reference).String(),
			Description:     post.Story,
			Content:         post.Story,
			PublishedParsed: parseHiruTime(post.AddedDate),
		})
	}
	if len(feed.Items) == 0 {
		return nil, fmt.Errorf("Hiru REST feed contains no identifiable posts")
	}
	return feed, nil
}

func parseHiruTime(value string) *time.Time {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	colombo := time.FixedZone("Asia/Colombo", 5*60*60+30*60)
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, colombo)
	if err != nil {
		return nil
	}
	return &parsed
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
