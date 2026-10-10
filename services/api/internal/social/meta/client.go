package meta

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

const requiredScopes = "pages_show_list,pages_read_engagement,pages_manage_posts"

type Config struct {
	AppID, AppSecret, RedirectURL, LoginConfigID, Version, BaseURL string
}

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(config Config, client *http.Client) *Client {
	if config.Version == "" {
		config.Version = "v26.0"
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://graph.facebook.com/" + config.Version
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	// Never forward credentials through a Graph redirect.
	copy := *client
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{config: config, http: &copy}
}

type Token struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type Page struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	AccessToken string   `json:"access_token"`
	Tasks       []string `json:"tasks"`
	Picture     struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	} `json:"picture"`
}

type TokenInfo struct {
	IsValid        bool     `json:"is_valid"`
	AppID          string   `json:"app_id"`
	Scopes         []string `json:"scopes"`
	ExpiresAt      int64    `json:"expires_at"`
	ProfileID      string   `json:"profile_id"`
	GranularScopes []struct {
		Scope     string   `json:"scope"`
		TargetIDs []string `json:"target_ids"`
	} `json:"granular_scopes"`
}

type Photo struct {
	ID     string `json:"id"`
	PostID string `json:"post_id"`
}

type Error struct {
	Code    int    `json:"code"`
	Subcode int    `json:"error_subcode"`
	Type    string `json:"type"`
	TraceID string `json:"fbtrace_id"`
}

func (err *Error) Error() string {
	return fmt.Sprintf("Meta Graph error (code=%d, subcode=%d, type=%s, trace=%s)", err.Code, err.Subcode, safeIdentifier(err.Type), safeIdentifier(err.TraceID))
}

func (err *Error) Classification() string {
	switch {
	case err.Code == 190 || err.Code == 102 || err.Code == 10 || (err.Code >= 200 && err.Code <= 299):
		return "needs_reconnect"
	case err.Code == 4 || err.Code == 17 || err.Code == 32 || err.Code == 613 || err.Code == 80001:
		return "rate_limited"
	default:
		return "failed"
	}
}

// Sent means a write may have reached Meta. Callers must not retry that upload automatically.
type TransportError struct{ Sent bool }

func (err *TransportError) Error() string { return "Meta request did not return a reliable response" }

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]*$`)

func safeIdentifier(value string) string {
	if len(value) > 80 || !identifierPattern.MatchString(value) {
		return "redacted"
	}
	return value
}

var sensitiveValue = regexp.MustCompile(`(?i)(access_token|appsecret_proof|client_secret|fb_exchange_token|input_token|code)["\s:=]+[^\s&,"}]+`)

func Redact(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[redacted]")
			value = strings.ReplaceAll(value, url.QueryEscape(secret), "[redacted]")
		}
	}
	return sensitiveValue.ReplaceAllString(value, "$1=[redacted]")
}

func (client *Client) proof(token string) string {
	hash := hmac.New(sha256.New, []byte(client.config.AppSecret))
	_, _ = hash.Write([]byte(token))
	return hex.EncodeToString(hash.Sum(nil))
}

func (client *Client) AuthorizeURL(state string) string {
	query := url.Values{"client_id": {client.config.AppID}, "redirect_uri": {client.config.RedirectURL}, "state": {state}, "response_type": {"code"}}
	if client.config.LoginConfigID != "" {
		query.Set("config_id", client.config.LoginConfigID)
	} else {
		query.Set("scope", requiredScopes)
	}
	return "https://www.facebook.com/" + client.config.Version + "/dialog/oauth?" + query.Encode()
}

func (client *Client) ExchangeCode(ctx context.Context, code string) (Token, error) {
	var result Token
	err := client.form(ctx, "oauth/access_token", url.Values{"client_id": {client.config.AppID}, "client_secret": {client.config.AppSecret}, "redirect_uri": {client.config.RedirectURL}, "code": {code}}, "", &result)
	if err == nil && result.AccessToken == "" {
		err = errors.New("Meta returned no user token")
	}
	return result, err
}

func (client *Client) ExchangeLongLived(ctx context.Context, token string) (Token, error) {
	var result Token
	err := client.form(ctx, "oauth/access_token", url.Values{"grant_type": {"fb_exchange_token"}, "client_id": {client.config.AppID}, "client_secret": {client.config.AppSecret}, "fb_exchange_token": {token}}, token, &result)
	if err == nil && result.AccessToken == "" {
		err = errors.New("Meta returned no long-lived user token")
	}
	return result, err
}

func (client *Client) ListPages(ctx context.Context, token string) ([]Page, error) {
	pages := make([]Page, 0)
	after := ""
	for range 100 {
		var result struct {
			Data   []Page `json:"data"`
			Paging struct {
				Next    string `json:"next"`
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
			} `json:"paging"`
		}
		query := url.Values{"fields": {"id,name,access_token,tasks,picture{url}"}, "limit": {"100"}}
		if after != "" {
			query.Set("after", after)
		}
		if err := client.get(ctx, "me/accounts", query, token, &result); err != nil {
			return nil, err
		}
		for _, page := range result.Data {
			if page.ID == "" || page.AccessToken == "" {
				continue
			}
			for _, task := range page.Tasks {
				if task == "CREATE_CONTENT" || task == "MANAGE" {
					pages = append(pages, page)
					break
				}
			}
		}
		if result.Paging.Next == "" {
			return pages, nil
		}
		if result.Paging.Cursors.After == "" || result.Paging.Cursors.After == after {
			return nil, errors.New("Meta Page pagination was incomplete")
		}
		after = result.Paging.Cursors.After
	}
	return nil, errors.New("Meta returned too many Page result pages")
}

func (client *Client) DebugToken(ctx context.Context, token string) (TokenInfo, error) {
	var result struct {
		Data TokenInfo `json:"data"`
	}
	err := client.get(ctx, "debug_token", url.Values{"input_token": {token}}, client.config.AppID+"|"+client.config.AppSecret, &result)
	return result.Data, err
}

func (client *Client) PublishPhoto(ctx context.Context, pageID, token string, png []byte, caption string) (Photo, error) {
	var result Photo
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range map[string]string{"message": caption, "published": "true", "access_token": token, "appsecret_proof": client.proof(token)} {
		if err := writer.WriteField(key, value); err != nil {
			return result, err
		}
	}
	part, err := writer.CreateFormFile("source", "card.png")
	if err != nil {
		return result, err
	}
	if _, err = part.Write(png); err != nil {
		return result, err
	}
	if err = writer.Close(); err != nil {
		return result, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint(pageID+"/photos"), &body)
	if err != nil {
		return result, errors.New("invalid Meta request")
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	err = client.do(request, &result, true)
	if err == nil && result.PostID == "" {
		err = &TransportError{Sent: true}
	}
	return result, err
}

func (client *Client) Permalink(ctx context.Context, postID, token string) (string, error) {
	var result struct {
		URL string `json:"permalink_url"`
	}
	err := client.get(ctx, postID, url.Values{"fields": {"permalink_url"}}, token, &result)
	return result.URL, err
}

func (client *Client) endpoint(path string) string {
	return strings.TrimRight(client.config.BaseURL, "/") + "/" + path
}

func (client *Client) get(ctx context.Context, path string, query url.Values, token string, result any) error {
	if token != "" {
		query.Set("appsecret_proof", client.proof(token))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.endpoint(path)+"?"+query.Encode(), nil)
	if err != nil {
		return errors.New("invalid Meta request")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return client.do(request, result, false)
}

func (client *Client) form(ctx context.Context, path string, values url.Values, token string, result any) error {
	if token != "" {
		values.Set("appsecret_proof", client.proof(token))
		values.Set("access_token", token)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint(path), strings.NewReader(values.Encode()))
	if err != nil {
		return errors.New("invalid Meta request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client.do(request, result, false)
}

func (client *Client) do(request *http.Request, result any, write bool) error {
	var sent atomic.Bool
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) { sent.Store(true) }}))
	response, err := client.http.Do(request)
	if err != nil {
		return &TransportError{Sent: write && sent.Load()}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return &TransportError{Sent: write}
	}
	var envelope struct {
		Error *Error `json:"error"`
	}
	if json.Unmarshal(data, &envelope) == nil && envelope.Error != nil {
		return envelope.Error
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if response.StatusCode == 429 {
			return &Error{Code: 4}
		}
		if write && response.StatusCode >= 500 {
			return &TransportError{Sent: true}
		}
		return fmt.Errorf("Meta returned HTTP %d", response.StatusCode)
	}
	if err := json.Unmarshal(data, result); err != nil {
		return &TransportError{Sent: write}
	}
	return nil
}
