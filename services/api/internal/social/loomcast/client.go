// Package loomcast talks to Loomcast's machine API, which publishes posts on the owner's
// Facebook Pages through Loomcast's own Meta app. See https://loomcast.prabhavalabs.com/llms.txt.
package loomcast

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://loomcast.prabhavalabs.com"

type Config struct {
	BaseURL string
	APIKey  string
}

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(config Config, httpClient *http.Client) *Client {
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	config.BaseURL = strings.TrimRight(config.BaseURL, "/")
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{config: config, http: httpClient}
}

// Channel is a connected account the API client was granted, such as a Facebook Page.
type Channel struct {
	ID               string   `json:"id"`
	Platform         string   `json:"platform"`
	Name             string   `json:"name"`
	Username         *string  `json:"username"`
	Status           string   `json:"status"`
	DryRun           bool     `json:"dryRun"`
	RequiresApproval bool     `json:"requiresApproval"`
	Access           string   `json:"access"`
	Formats          []string `json:"formats"`
}

type Media struct {
	Filename string `json:"filename"`
	MIME     string `json:"mime"`
	Bytes    int    `json:"bytes"`
	SHA256   string `json:"sha256"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	AltText  string `json:"altText,omitempty"`
}

type ChannelRef struct {
	ChannelID string `json:"channelId"`
}

type NewPost struct {
	Format      string       `json:"format"`
	Title       string       `json:"title,omitempty"`
	Caption     string       `json:"caption,omitempty"`
	Media       []Media      `json:"media"`
	Channels    []ChannelRef `json:"channels"`
	ScheduledAt time.Time    `json:"scheduledAt"`
	ExternalRef string       `json:"externalRef,omitempty"`
}

type Upload struct {
	MediaID   string            `json:"mediaId"`
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type TargetError struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	Retryable    bool   `json:"retryable"`
	RetryAfterMs int64  `json:"retryAfterMs"`
}

type Target struct {
	ChannelID   string       `json:"channelId"`
	Platform    string       `json:"platform"`
	PostID      *string      `json:"postId"`
	Status      *string      `json:"status"`
	ScheduledAt time.Time    `json:"scheduledAt"`
	Permalink   *string      `json:"permalink"`
	ExternalID  *string      `json:"externalId"`
	LastError   *TargetError `json:"lastError"`
	PublishedAt *time.Time   `json:"publishedAt"`
	DryRun      bool         `json:"dryRun"`
}

// Post statuses: awaiting_media, pending_approval, scheduled, publishing, published,
// partially_published, failed and cancelled.
type Post struct {
	ID          string    `json:"id"`
	ExternalRef *string   `json:"externalRef"`
	Status      string    `json:"status"`
	ScheduledAt time.Time `json:"scheduledAt"`
	Targets     []Target  `json:"targets"`
	Uploads     []Upload  `json:"uploads"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Target returns the post's latest target on a channel.
func (post Post) Target(channelID string) *Target {
	var found *Target
	for i := range post.Targets {
		if post.Targets[i].ChannelID == channelID {
			found = &post.Targets[i]
		}
	}
	return found
}

// APIError is a response Loomcast answered with an error status.
type APIError struct {
	Status     int
	Code       string
	Message    string
	RetryAfter time.Duration
}

func (err *APIError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("Loomcast answered %d %s", err.Status, err.Code)
	}
	return fmt.Sprintf("Loomcast answered %d %s: %s", err.Status, err.Code, err.Message)
}

// Temporary reports whether the same request may succeed later.
func (err *APIError) Temporary() bool {
	return err.Status == http.StatusTooManyRequests || err.Status >= 500
}

// Unauthorized reports whether the API key or its grants need the owner's attention.
func (err *APIError) Unauthorized() bool {
	return err.Status == http.StatusUnauthorized || err.Status == http.StatusForbidden
}

// TransportError means no reliable answer arrived; the request may or may not have taken effect.
type TransportError struct{ Op string }

func (err *TransportError) Error() string {
	return "Loomcast did not answer " + err.Op + " reliably"
}

func (client *Client) Channels(ctx context.Context) ([]Channel, error) {
	var result struct {
		Items []Channel `json:"items"`
	}
	err := client.call(ctx, http.MethodGet, "/api/v1/channels", nil, "", &result)
	return result.Items, err
}

// CreatePost creates a post waiting for its files. The key makes a repeated request return the
// same post instead of a second one.
func (client *Client) CreatePost(ctx context.Context, idempotencyKey string, post NewPost) (Post, error) {
	var result Post
	err := client.call(ctx, http.MethodPost, "/api/v1/posts", post, idempotencyKey, &result)
	return result, err
}

func (client *Client) Finalize(ctx context.Context, id string) (Post, error) {
	var result Post
	err := client.call(ctx, http.MethodPost, "/api/v1/posts/"+url.PathEscape(id)+"/finalize", struct{}{}, "", &result)
	return result, err
}

func (client *Client) Post(ctx context.Context, id string) (Post, error) {
	var result Post
	err := client.call(ctx, http.MethodGet, "/api/v1/posts/"+url.PathEscape(id), nil, "", &result)
	return result, err
}

func (client *Client) Cancel(ctx context.Context, id string) (Post, error) {
	var result Post
	err := client.call(ctx, http.MethodPost, "/api/v1/posts/"+url.PathEscape(id)+"/cancel", struct{}{}, "", &result)
	return result, err
}

// Upload sends a file's bytes to its presigned URL with exactly the signed headers.
func (client *Client) Upload(ctx context.Context, upload Upload, data []byte) error {
	method := upload.Method
	if method == "" {
		method = http.MethodPut
	}
	target, err := url.Parse(upload.URL)
	if err != nil || (target.Scheme != "https" && target.Scheme != "http") || target.Host == "" {
		return errors.New("Loomcast returned an invalid upload address")
	}
	request, err := http.NewRequestWithContext(ctx, method, upload.URL, bytes.NewReader(data))
	if err != nil {
		return errors.New("Loomcast returned an invalid upload address")
	}
	for name, value := range upload.Headers {
		if strings.EqualFold(name, "Content-Length") {
			continue
		}
		request.Header.Set(name, value)
	}
	request.ContentLength = int64(len(data))
	response, err := client.http.Do(request)
	if err != nil {
		return &TransportError{Op: "the file upload"}
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return &APIError{Status: response.StatusCode, Code: "upload_failed", Message: "the file store refused the upload"}
	}
	return nil
}

func (client *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string, result any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.config.BaseURL+path, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+client.config.APIKey)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := client.http.Do(request)
	if err != nil {
		return &TransportError{Op: method + " " + path}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return &TransportError{Op: method + " " + path}
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		apiError := &APIError{Status: response.StatusCode}
		var problem struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &problem) == nil {
			apiError.Code, apiError.Message = problem.Error.Code, truncate(problem.Error.Message, 300)
		}
		if seconds, err := strconv.Atoi(response.Header.Get("Retry-After")); err == nil && seconds > 0 {
			apiError.RetryAfter = time.Duration(seconds) * time.Second
		}
		return apiError
	}
	if result == nil {
		return nil
	}
	if err := json.Unmarshal(data, result); err != nil {
		return &TransportError{Op: method + " " + path}
	}
	return nil
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
