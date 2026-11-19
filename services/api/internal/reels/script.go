package reels

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
)

const scriptPrompt = `You write the on-screen script for a 14 second vertical Facebook Reel for Thulawa News (තුලාව), a balanced Sinhala news outlet in Sri Lanka. The reel shows images with a bold headline, three key facts and a debate question that viewers answer with a Like (👍) or a Love (❤️) reaction.

Write all viewer-facing text in natural, everyday Sinhala as Sri Lankans speak and read it in news; keep names in the form the reports use.

Rules:
- Use only facts stated in the reports provided. Never add numbers, dates, names or claims that are not there. Where reports disagree, leave the point out.
- Each on-screen fact must be reported by at least two different outlets (the cross-source summary says which outlets report what). Leave out anything only one outlet reports, however newsworthy.
- Someone charged or in court is accused, not guilty. Say "චෝදනා", never imply guilt, and never insult anyone.
- tag: 2-3 words that label the story, for example "අධිකරණ පුවත" or "ආර්ථික පුවත".
- headline: 2 or 3 short punchy lines that hook the viewer in one glance; each line at most 22 characters.
- facts: exactly 3 key facts, each at most 55 characters, in short spoken Sinhala that is read in two seconds, most important first. No source names inside the facts.
- questions: exactly 3 different yes/no opinion questions, each at most 60 characters, for the reel to choose from. The aim is a real debate in the comments: a question Sri Lankans split on roughly half and half, not one that nearly everyone answers the same way.
  - Avoid obvious questions that everyone agrees with, such as whether a case should continue, whether the law should be followed, or whether the truth should come out.
  - Prefer questions about satisfaction, trust or expectations, where people's experience and politics lead them to different answers. Examples: "මේ නඩු කටයුතු සිදුවන ආකාරය ගැන ඔබ සෑහීමකට පත්වෙනවාද?", "මේ නඩුවෙන් සාධාරණ තීන්දුවක් ලැබේ යැයි ඔබ විශ්වාස කරනවාද?", "මෙය දේශපාලන පළිගැනීමක් ලෙස ඔබට පෙනෙනවාද?" only when the reports show that claim being made.
  - Never ask whether the accused is guilty or innocent, should be jailed or freed, or anything that prejudges a court.
  - For each question give yes_label and no_label (1-2 plain words, no emoji) and expected_yes_percent: your honest estimate (0-100) of how many Sri Lankan Facebook users would answer yes.
- caption: the Facebook post text, 2-4 sentences of plain facts that obey the same two-outlet rule; mention that it is based on reports from several outlets. Do not include the question; it is added after the caption.
- hashtags: 3-5 relevant hashtags (Sinhala or English), no "#" signs.
- image_prompts: exactly 2 English prompts for photorealistic, cinematic, symbolic news images that illustrate the story's setting or subject (for example a court building, a gavel, an aircraft, documents, a stadium). Never depict a real, identifiable person or any likeness of one; any people must be anonymous, seen from behind or in silhouette. No text, letters, logos or flags with text.
- people: up to 2 public figures central to the story, written exactly as the title of their English Wikipedia article (for example "Namal Rajapaksa"). Leave it empty when the story is not about named public figures.
- photo_queries: up to 2 short English search queries for real photos of places, buildings, vehicles or objects central to the story that are likely on Wikimedia Commons (for example "SriLankan Airlines Airbus A330", "Parliament of Sri Lanka"). Never people.`

func scriptSchema() map[string]any {
	str := func(max int) map[string]any { return map[string]any{"type": "string", "maxLength": max} }
	list := func(item map[string]any, min, max int) map[string]any {
		return map[string]any{"type": "array", "items": item, "minItems": min, "maxItems": max}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"tag", "headline", "facts", "questions", "caption", "hashtags", "image_prompts", "people", "photo_queries"},
		"properties": map[string]any{
			"tag": str(30), "headline": list(str(30), 2, 3), "facts": list(str(90), 3, 3),
			"questions": list(map[string]any{
				"type": "object", "additionalProperties": false,
				"required": []string{"question", "yes_label", "no_label", "expected_yes_percent"},
				"properties": map[string]any{
					"question": str(80), "yes_label": str(16), "no_label": str(16),
					"expected_yes_percent": map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
				},
			}, 3, 3),
			"caption": str(800), "hashtags": list(str(40), 3, 5), "image_prompts": list(str(500), 2, 2),
			"people": list(str(80), 0, 2), "photo_queries": list(str(80), 0, 2),
		},
	}
}

// storyInput is what the script writer reads: the storyline, its latest development and reports.
type storyInput struct {
	Title, Summary, LatestTitle, LatestSummary, Analysis string
	Reports                                              []report
	Earlier                                              []string
}

type report struct{ Source, Headline, Excerpt string }

func (input storyInput) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Storyline: %s\nStoryline summary: %s\n\nLatest development: %s\n%s\n\n", input.Title, input.Summary, input.LatestTitle, input.LatestSummary)
	if input.Analysis != "" {
		fmt.Fprintf(&b, "Cross-source summary of the latest development:\n%s\n\n", input.Analysis)
	}
	b.WriteString("Reports:\n")
	for _, r := range input.Reports {
		fmt.Fprintf(&b, "- %s: %s. %s\n", r.Source, r.Headline, r.Excerpt)
	}
	if len(input.Earlier) > 0 {
		b.WriteString("\nEarlier developments (newest first):\n")
		for _, line := range input.Earlier {
			fmt.Fprintf(&b, "- %s\n", line)
		}
	}
	return b.String()
}

func writeScript(ctx context.Context, model completer, input storyInput) (Script, error) {
	response, err := model.Complete(ctx, llm.Request{Task: "reel_script", System: scriptPrompt, Input: input.text(), JSONSchema: scriptSchema()})
	if err != nil {
		return Script{}, err
	}
	if strings.TrimSpace(response.Text) == "" {
		return Script{}, fmt.Errorf("the reel_script AI task returned nothing; check its route under AI routing")
	}
	var script Script
	if err := json.Unmarshal([]byte(response.Text), &script); err != nil {
		return Script{}, fmt.Errorf("decode reel script: %w", err)
	}
	if err := script.chooseQuestion(); err != nil {
		return Script{}, err
	}
	return script, validScript(script)
}

// chooseQuestion keeps the candidate expected to split viewers most evenly.
func (script *Script) chooseQuestion() error {
	if len(script.Questions) == 0 {
		return fmt.Errorf("the reel script has no questions")
	}
	best := script.Questions[0]
	for _, candidate := range script.Questions[1:] {
		if distance(candidate.ExpectedYes) < distance(best.ExpectedYes) {
			best = candidate
		}
	}
	script.Question, script.YesLabel, script.NoLabel = strings.TrimSpace(best.Question), plainLabel(best.YesLabel), plainLabel(best.NoLabel)
	return nil
}

func distance(percent int) int {
	if percent < 50 {
		return 50 - percent
	}
	return percent - 50
}

// plainLabel drops emoji and symbols from a button label; the buttons draw their own icons.
func plainLabel(label string) string {
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Variation_Selector, r) {
			return -1
		}
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsSpace(r) || unicode.IsDigit(r) || r == 0x200D || r == 0x200C {
			return r
		}
		return -1
	}, label)), " ")
}

// buildCaption assembles the Facebook post text from the script and the image credits.
func buildCaption(script Script, photos []Photo) string {
	var caption strings.Builder
	caption.WriteString(strings.TrimSpace(script.Caption) + "\n\n")
	fmt.Fprintf(&caption, "🗳️ %s\n❤️ %s   👍 %s\nඇයි කියලත් comment එකකින් කියන්න 👇\n\n", script.Question, script.YesLabel, script.NoLabel)
	tags := make([]string, 0, len(script.Hashtags)+2)
	for _, tag := range script.Hashtags {
		if tag = strings.ReplaceAll(strings.TrimPrefix(strings.TrimSpace(tag), "#"), " ", ""); tag != "" {
			tags = append(tags, "#"+tag)
		}
	}
	caption.WriteString(strings.Join(append(tags, "#තුලාව", "#ThulawaNews"), " ") + "\n\n")
	var credits []string
	ai := false
	for _, photo := range photos {
		if photo.Kind == "ai" {
			ai = true
		} else if photo.Credit != "" {
			credits = append(credits, photo.Credit)
		}
	}
	if ai {
		credits = append(credits, "සමහර රූප AI මගින් නිර්මිත නිදර්ශන වේ")
	}
	if len(credits) > 0 {
		caption.WriteString("ඡායාරූප: " + strings.Join(credits, " · ") + "\n")
	}
	return strings.TrimSpace(caption.String())
}

const pickPrompt = `You choose which trending Sri Lankan news storyline Thulawa News (a balanced Sinhala outlet) should turn into its next short Facebook Reel. Each reel ends with a yes/no debate question that viewers answer with a Like or Love reaction.

You get the candidate storylines (with their coverage trend score) and what people in Sri Lanka are searching for on Google right now (Google Trends, with the news headlines Google links to each search).

Rank the candidates, best first, and leave out any that are unsuitable:
- Prefer stories Sri Lankans are actively searching for: match Google searches to storylines by meaning, in any language or spelling (an English search can match a Sinhala storyline).
- Prefer public-interest stories that invite a genuine debate: politics, courts involving public figures, economy and prices, public services, policy, sport.
- Leave out stories centred on deaths, disasters, accidents, violent crime against private individuals, children, or anything where a Like/Love reaction poll would be insensitive.
- Leave out stories whose only development is trivial.

For each ranked storyline give the matched Google searches (exact search terms, empty if none) and a one-sentence reason in English.`

func pickSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"ranking"},
		"properties": map[string]any{"ranking": map[string]any{"type": "array", "maxItems": 10, "items": map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"storyline_id", "google_trends", "reason"},
			"properties": map[string]any{
				"storyline_id":  map[string]any{"type": "string"},
				"google_trends": map[string]any{"type": "array", "maxItems": 5, "items": map[string]any{"type": "string", "maxLength": 80}},
				"reason":        map[string]any{"type": "string", "maxLength": 300},
			},
		}}},
	}
}

type candidate struct {
	ID          string `json:"storyline_id"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Latest      string `json:"latest_development"`
	Score       int    `json:"trend_score"`
	SourceCount int    `json:"source_count"`
}

type ranked struct {
	StorylineID  string   `json:"storyline_id"`
	GoogleTrends []string `json:"google_trends"`
	Reason       string   `json:"reason"`
}

// rankStories asks the model to order the candidates by expected interest; an empty result means
// none is suitable. Unknown ids in the answer are ignored.
func rankStories(ctx context.Context, model completer, candidates []candidate, trends []googleTrend) ([]ranked, error) {
	data, _ := json.Marshal(map[string]any{"candidates": candidates, "google_trends_sri_lanka": trends})
	response, err := model.Complete(ctx, llm.Request{Task: "reel_story_pick", System: pickPrompt, Input: string(data), JSONSchema: pickSchema()})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(response.Text) == "" {
		return nil, fmt.Errorf("the reel_story_pick AI task returned nothing; check its route under AI routing")
	}
	var result struct {
		Ranking []ranked `json:"ranking"`
	}
	if err := json.Unmarshal([]byte(response.Text), &result); err != nil {
		return nil, fmt.Errorf("decode story ranking: %w", err)
	}
	known := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		known[c.ID] = true
	}
	ordered := make([]ranked, 0, len(result.Ranking))
	for _, item := range result.Ranking {
		if known[item.StorylineID] {
			ordered = append(ordered, item)
			known[item.StorylineID] = false
		}
	}
	return ordered, nil
}
