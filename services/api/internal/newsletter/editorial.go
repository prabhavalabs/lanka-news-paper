package newsletter

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
)

const editorialSystemPrompt = `Prepare the editorial shape of a Sinhala morning news email from a rights-approved 24-hour digest.
Return a short, natural Sinhala introduction and the story IDs in the order readers should see them.
Prioritize public interest, urgency, breadth of independent coverage, and a useful mix of subjects.
Do not rewrite, add, or infer story facts. Story titles and summaries are rendered from the verified digest after your response.`

type editorialPlan struct {
	Intro    string   `json:"intro"`
	StoryIDs []string `json:"story_ids"`
}

var editorialSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"intro", "story_ids"},
	"properties": map[string]any{
		"intro": map[string]any{"type": "string", "maxLength": 1200},
		"story_ids": map[string]any{
			"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 50,
		},
	},
}

func applyEditorialPlan(ctx context.Context, model llm.Completer, digest Digest, settings Settings) (Digest, Settings) {
	if model == nil || len(digest.Stories) == 0 {
		return digest, settings
	}
	payload, err := json.Marshal(digest)
	if err != nil {
		return digest, settings
	}
	response, err := model.Complete(ctx, llm.Request{
		Task: "newsletter_editorial", System: editorialSystemPrompt, Input: string(payload),
		JSONSchema: editorialSchema, DisableReasoning: true, MaxTokens: 1800,
	})
	if err != nil || strings.TrimSpace(response.Text) == "" {
		return digest, settings
	}
	var plan editorialPlan
	if err := json.Unmarshal([]byte(response.Text), &plan); err != nil {
		return digest, settings
	}
	plan.Intro = strings.TrimSpace(plan.Intro)
	if plan.Intro != "" && utf8.RuneCountInString(plan.Intro) <= 1200 {
		settings.IntroText = plan.Intro
		digest.Intro = plan.Intro
	}
	byID := make(map[string]Story, len(digest.Stories))
	for _, story := range digest.Stories {
		byID[story.ID] = story
	}
	ordered := make([]Story, 0, len(digest.Stories))
	seen := make(map[string]bool, len(digest.Stories))
	for _, id := range plan.StoryIDs {
		if story, ok := byID[id]; ok && !seen[id] {
			ordered = append(ordered, story)
			seen[id] = true
		}
	}
	for _, story := range digest.Stories {
		if !seen[story.ID] {
			ordered = append(ordered, story)
		}
	}
	digest.Stories = ordered
	return digest, settings
}
