package reels

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Wikimedia asks automated clients for a descriptive User-Agent.
const userAgent = "ThulawaNewsReels/1.0 (+https://thulawanews.com)"

func fetch(ctx context.Context, client *http.Client, address string, limit int64) ([]byte, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("User-Agent", userAgent)
	response, err := client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s: %s", address, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit))
	return data, response.Header.Get("Content-Type"), err
}

// googleTrend is one Google Trends search rising in Sri Lanka, with the news Google links to it.
type googleTrend struct {
	Query   string   `json:"search"`
	Traffic string   `json:"approx_searches"`
	News    []string `json:"news_headlines"`
}

const googleTrendsURL = "https://trends.google.com/trending/rss?geo=LK"

func googleTrends(ctx context.Context, client *http.Client) ([]googleTrend, error) {
	data, _, err := fetch(ctx, client, googleTrendsURL, 2<<20)
	if err != nil {
		return nil, err
	}
	return parseGoogleTrends(data)
}

func parseGoogleTrends(data []byte) ([]googleTrend, error) {
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
	trends := make([]googleTrend, 0, len(feed.Items))
	for _, item := range feed.Items {
		trend := googleTrend{Query: strings.TrimSpace(item.Title), Traffic: strings.TrimSpace(item.Traffic)}
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

// commonsImage is a Wikimedia Commons file we may reuse: public domain, CC0 or CC BY.
type commonsImage struct {
	Title, URL, License, Artist string
	Width                       int
}

// Credit is the on-screen and caption attribution, e.g. "Jane Doe / Wikimedia Commons (CC BY 2.0)".
func (image commonsImage) Credit() string {
	artist := image.Artist
	if artist == "" {
		return "Wikimedia Commons (" + image.License + ")"
	}
	return artist + " / Wikimedia Commons (" + image.License + ")"
}

// Share-alike (CC BY-SA) would bind the whole reel to that licence, and non-commercial or
// no-derivatives licences forbid this use, so only these pass.
var reusableLicense = regexp.MustCompile(`(?i)^(cc0( 1\.0)?|public domain|pdm( 1\.0)?|pd|cc by \d(\.\d)?)$`)

var tags = regexp.MustCompile(`<[^>]+>`)

// A reel is a 9:16 crop, so very wide panoramas end up as sky; skip them.
func usableShape(width, height int) bool {
	return height > 0 && float64(width)/float64(height) <= 1.8
}

var yearPattern = regexp.MustCompile(`\b(1[89]\d\d|20\d\d)\b`)

// archival reports photos taken before 1990: old prints rarely show today's place or object.
func archival(taken string) bool {
	match := yearPattern.FindString(plainText(taken, 200))
	return match != "" && match < "1990"
}

func plainText(value string, limit int) string {
	text := strings.Join(strings.Fields(html.UnescapeString(tags.ReplaceAllString(value, " "))), " ")
	if runes := []rune(text); len(runes) > limit {
		text = strings.TrimSpace(string(runes[:limit])) + "…"
	}
	return text
}

type imageInfoResponse struct {
	Query struct {
		Pages map[string]struct {
			Title     string `json:"title"`
			ImageInfo []struct {
				ThumbURL    string `json:"thumburl"`
				URL         string `json:"url"`
				Width       int    `json:"width"`
				Height      int    `json:"height"`
				Mime        string `json:"mime"`
				ExtMetadata map[string]struct {
					Value metadataValue `json:"value"`
				} `json:"extmetadata"`
			} `json:"imageinfo"`
		} `json:"pages"`
	} `json:"query"`
}

// metadataValue accepts Commons extmetadata values, which are usually strings but sometimes
// numbers or booleans (for example "Copyrighted" or image dimensions).
type metadataValue string

func (value *metadataValue) UnmarshalJSON(data []byte) error {
	var text string
	if json.Unmarshal(data, &text) == nil {
		*value = metadataValue(text)
		return nil
	}
	*value = metadataValue(strings.Trim(string(data), `"`))
	return nil
}

func (response imageInfoResponse) images() []commonsImage {
	var images []commonsImage
	for _, page := range response.Query.Pages {
		if len(page.ImageInfo) == 0 {
			continue
		}
		info := page.ImageInfo[0]
		if info.Mime != "image/jpeg" && info.Mime != "image/png" {
			continue
		}
		license := plainText(string(info.ExtMetadata["LicenseShortName"].Value), 40)
		if !reusableLicense.MatchString(license) || info.Width < 600 || !usableShape(info.Width, info.Height) ||
			archival(string(info.ExtMetadata["DateTimeOriginal"].Value)) {
			continue
		}
		address := info.ThumbURL
		if address == "" {
			address = info.URL
		}
		images = append(images, commonsImage{
			Title: page.Title, URL: address, License: license, Width: info.Width,
			Artist: plainText(string(info.ExtMetadata["Artist"].Value), 60),
		})
	}
	return images
}

const commonsAPI = "https://commons.wikimedia.org/w/api.php"

func imageInfoParams(values url.Values) url.Values {
	values.Set("action", "query")
	values.Set("prop", "imageinfo")
	values.Set("iiprop", "url|size|mime|extmetadata")
	values.Set("iiurlwidth", "1280")
	values.Set("format", "json")
	return values
}

// personPhoto returns the lead image of a person's English Wikipedia article when its licence allows reuse.
func personPhoto(ctx context.Context, client *http.Client, name string) (*commonsImage, error) {
	query := url.Values{"action": {"query"}, "titles": {name}, "prop": {"pageimages"}, "piprop": {"name"}, "redirects": {"1"}, "format": {"json"}}
	data, _, err := fetch(ctx, client, "https://en.wikipedia.org/w/api.php?"+query.Encode(), 1<<20)
	if err != nil {
		return nil, err
	}
	var pages struct {
		Query struct {
			Pages map[string]struct {
				PageImage string `json:"pageimage"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(data, &pages); err != nil {
		return nil, err
	}
	for _, page := range pages.Query.Pages {
		if page.PageImage == "" {
			continue
		}
		info, err := commonsInfo(ctx, client, url.Values{"titles": {"File:" + page.PageImage}})
		if err != nil || len(info) == 0 {
			return nil, err
		}
		return &info[0], nil
	}
	return nil, nil
}

// searchPhoto returns the first reusable Commons photo for a search query.
func searchPhoto(ctx context.Context, client *http.Client, query string) (*commonsImage, error) {
	info, err := commonsInfo(ctx, client, url.Values{
		"generator": {"search"}, "gsrsearch": {query + " filetype:bitmap"}, "gsrnamespace": {"6"}, "gsrlimit": {"8"},
	})
	if err != nil || len(info) == 0 {
		return nil, err
	}
	best := info[0]
	for _, image := range info[1:] {
		if image.Width > best.Width && best.Width < 1200 {
			best = image
		}
	}
	return &best, nil
}

func commonsInfo(ctx context.Context, client *http.Client, values url.Values) ([]commonsImage, error) {
	data, _, err := fetch(ctx, client, commonsAPI+"?"+imageInfoParams(values).Encode(), 4<<20)
	if err != nil {
		return nil, err
	}
	var response imageInfoResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.images(), nil
}
