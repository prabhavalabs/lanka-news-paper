package trends

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
)

type completer interface {
	Complete(context.Context, llm.Request) (llm.Response, error)
}

// WebTopic is one topic the web research found people in Sri Lanka discussing.
type WebTopic struct {
	Key      string `json:"key,omitempty"`
	TitleEN  string `json:"title_en"`
	TitleSI  string `json:"title_si"`
	Category string `json:"category"`
	Why      string `json:"why"`
	Momentum string `json:"momentum"`
	Evidence []Link `json:"evidence"`
}

type Link struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

var categories = []string{"politics", "economy", "courts", "crime", "sports", "entertainment", "society", "disaster", "international", "other"}

const researchPrompt = `You are a trend researcher for Thulawa News, a balanced Sinhala news outlet in Sri Lanka. Use web search to find what people in Sri Lanka are actually talking about right now, the way a "last 30 days" researcher would: look for real public attention, not just what one newsroom published.

Search widely and in both Sinhala and English:
- Sri Lankan news sites (Ada Derana, Hiru News, Newsfirst, Daily Mirror, Lankadeepa, Divaina, Daily FT, EconomyNext and others) for their most read, most commented and live stories.
- Social media attention reported by the media: viral Facebook or TikTok posts, X (Twitter) and Reddit r/srilanka discussions, YouTube clips going viral.
- Sport (especially cricket), entertainment and celebrity news popular in Sri Lanka, as well as politics, economy and courts.

Return up to 10 distinct topics from the last 24 hours that are generating the most public discussion in Sri Lanka, most discussed first. For each:
- title_en and title_si: a short neutral headline in English and in Sinhala.
- category: one of politics, economy, courts, crime, sports, entertainment, society, disaster, international, other.
- why: one sentence on why people are talking about it (what makes it viral or controversial).
- momentum: "rising" if attention is growing fast or new today, "steady" if it has been big for days, "fading" if it is past its peak, compared with the past 30 days.
- evidence: 2 to 4 links you actually opened, from different sites where possible, each with the page title.
Only include topics you found evidence for. Never invent links.`

func researchSchema() map[string]any {
	str := func(max int) map[string]any { return map[string]any{"type": "string", "maxLength": max} }
	return map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"topics"},
		"properties": map[string]any{"topics": map[string]any{"type": "array", "maxItems": 10, "items": map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []string{"title_en", "title_si", "category", "why", "momentum", "evidence"},
			"properties": map[string]any{
				"title_en": str(140), "title_si": str(140), "why": str(400),
				"category": map[string]any{"type": "string", "enum": categories},
				"momentum": map[string]any{"type": "string", "enum": []string{"rising", "steady", "fading"}},
				"evidence": map[string]any{"type": "array", "minItems": 1, "maxItems": 4, "items": map[string]any{
					"type": "object", "additionalProperties": false, "required": []string{"title", "url"},
					"properties": map[string]any{"title": str(200), "url": str(600)},
				}},
			},
		}}},
	}
}

// Research asks the model, with live web search, what Sri Lanka is discussing in the last 24 hours.
func Research(ctx context.Context, model completer, now time.Time) ([]WebTopic, error) {
	colombo := now.In(colomboTime)
	input := fmt.Sprintf("Now in Sri Lanka: %s (Asia/Colombo). Window: the last 24 hours, compared with the previous 30 days.", colombo.Format("Monday 2 January 2006, 15:04"))
	response, err := model.Complete(ctx, llm.Request{Task: "trend_research", System: researchPrompt, Input: input, JSONSchema: researchSchema(), WebSearch: true})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(response.Text) == "" {
		return nil, fmt.Errorf("the trend_research AI task returned nothing; check its route under AI routing")
	}
	var result struct {
		Topics []WebTopic `json:"topics"`
	}
	if err := json.Unmarshal([]byte(response.Text), &result); err != nil {
		return nil, fmt.Errorf("decode trend research: %w", err)
	}
	topics := result.Topics[:0]
	for _, topic := range result.Topics {
		links := topic.Evidence[:0]
		for _, link := range topic.Evidence {
			if strings.HasPrefix(link.URL, "https://") || strings.HasPrefix(link.URL, "http://") {
				links = append(links, link)
			}
		}
		if topic.Evidence = links; len(links) > 0 {
			topics = append(topics, topic)
		}
	}
	return topics, nil
}
