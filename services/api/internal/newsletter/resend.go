package newsletter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type EmailMessage struct {
	From           string
	To             string
	Subject        string
	HTML           string
	Text           string
	Headers        map[string]string
	IdempotencyKey string
}

type Sender interface {
	Send(context.Context, EmailMessage) (string, error)
}

type ResendSender struct {
	apiKey   string
	client   *http.Client
	endpoint string
}

func NewResendSender(apiKey string, client *http.Client) *ResendSender {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &ResendSender{apiKey: apiKey, client: client, endpoint: "https://api.resend.com/emails"}
}

func (sender *ResendSender) Send(ctx context.Context, message EmailMessage) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"from": message.From, "to": []string{message.To}, "subject": message.Subject,
		"html": message.HTML, "text": message.Text, "headers": message.Headers,
	})
	if err != nil {
		return "", fmt.Errorf("encode Resend email: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, sender.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create Resend request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+sender.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "lanka-news-paper/1.0")
	if message.IdempotencyKey != "" {
		request.Header.Set("Idempotency-Key", message.IdempotencyKey)
	}
	response, err := sender.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("send newsletter through Resend: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return "", fmt.Errorf("read Resend response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var problem struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &problem); err != nil || strings.TrimSpace(problem.Message) == "" {
			problem.Message = http.StatusText(response.StatusCode)
		}
		return "", fmt.Errorf("Resend rejected newsletter (%d): %s", response.StatusCode, problem.Message)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode Resend response: %w", err)
	}
	if strings.TrimSpace(result.ID) == "" {
		return "", fmt.Errorf("decode Resend response: message id is empty")
	}
	return result.ID, nil
}
