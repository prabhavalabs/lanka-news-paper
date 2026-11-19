package trends

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
)

// GoogleTrend is one search rising in Sri Lanka on Google, with the news Google links to it.
type GoogleTrend struct {
	Key     string   `json:"key,omitempty"`
	Query   string   `json:"search"`
	Traffic string   `json:"approx_searches"`
	News    []string `json:"news_headlines"`
}

const googleTrendsURL = "https://trends.google.com/trending/rss?geo=LK"

// FetchGoogleTrends reads Google's trending searches for Sri Lanka.
func FetchGoogleTrends(ctx context.Context, client *http.Client) ([]GoogleTrend, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, googleTrendsURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "ThulawaNewsTrends/1.0 (+https://thulawanews.com)")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google Trends: %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return ParseGoogleTrends(data)
}

func ParseGoogleTrends(data []byte) ([]GoogleTrend, error) {
	var feed struct {
		Items []struct {
			Title   string `xml:"title"`
			Traffic string `xml:"approx_traffic"`
			News    []struct {
				Title string `xml:"news_item_title"`
			} `xml:"news_item"`
		} `xml:"channel>item"`
	}
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("parse Google Trends feed: %w", err)
	}
	trends := make([]GoogleTrend, 0, len(feed.Items))
	for _, item := range feed.Items {
		trend := GoogleTrend{Query: strings.TrimSpace(item.Title), Traffic: strings.TrimSpace(item.Traffic)}
		for _, news := range item.News {
			if title := strings.TrimSpace(html.UnescapeString(news.Title)); title != "" && len(trend.News) < 3 {
				trend.News = append(trend.News, title)
			}
		}
		if trend.Query != "" {
			trends = append(trends, trend)
		}
	}
	return trends, nil
}
