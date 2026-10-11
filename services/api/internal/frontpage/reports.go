package frontpage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/llm"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/textcheck"
)

// minReviewScore is the reviewer score a report needs, with no blocker or major issue, to print.
const minReviewScore = 80

type evidence struct {
	Outlet      string    `json:"outlet"`
	Headline    string    `json:"headline"`
	PublishedAt time.Time `json:"published_at"`
	Summary     string    `json:"summary,omitempty"`
	Text        string    `json:"text,omitempty"`
}

type draft struct {
	Headline           string   `json:"headline"`
	Standfirst         string   `json:"standfirst"`
	Paragraphs         []string `json:"paragraphs"`
	IllustrationPrompt string   `json:"illustration_prompt"`
}

var draftSchema = map[string]any{
	"type": "object", "additionalProperties": false, "required": []string{"headline", "standfirst", "paragraphs", "illustration_prompt"},
	"properties": map[string]any{
		"headline":            map[string]any{"type": "string", "maxLength": 120},
		"standfirst":          map[string]any{"type": "string", "maxLength": 260},
		"paragraphs":          map[string]any{"type": "array", "minItems": 1, "maxItems": 14, "items": map[string]any{"type": "string", "maxLength": 1200}},
		"illustration_prompt": map[string]any{"type": "string", "maxLength": 400},
	},
}

type reviewIssue struct {
	Severity string `json:"severity"`
	Problem  string `json:"problem"`
}

type reviewResult struct {
	Pass    bool          `json:"pass"`
	Score   int           `json:"score"`
	Summary string        `json:"summary"`
	Issues  []reviewIssue `json:"issues"`
}

var reviewSchema = map[string]any{
	"type": "object", "additionalProperties": false, "required": []string{"pass", "score", "summary", "issues"},
	"properties": map[string]any{
		"pass":    map[string]any{"type": "boolean"},
		"score":   map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
		"summary": map[string]any{"type": "string", "maxLength": 600},
		"issues": map[string]any{"type": "array", "maxItems": 12, "items": map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"severity", "problem"},
			"properties": map[string]any{
				"severity": map[string]any{"type": "string", "enum": []string{"blocker", "major", "minor"}},
				"problem":  map[string]any{"type": "string", "maxLength": 400},
			},
		}},
	},
}

func (result reviewResult) printable() bool {
	if !result.Pass || result.Score < minReviewScore {
		return false
	}
	for _, issue := range result.Issues {
		if issue.Severity == "blocker" || issue.Severity == "major" {
			return false
		}
	}
	return true
}

// report returns a printable report for the story at the given size, reusing the latest one
// while the story is unchanged, otherwise writing, reviewing and (once) revising a new one.
func (service *Service) report(ctx context.Context, story Story, size string) (string, error) {
	var reportID string
	err := service.store.pool.QueryRow(ctx, `
		SELECT id::text FROM story_reports
		WHERE (event_id::text = $1 OR article_id::text = $1) AND size = $2
		  AND story_updated_at = $3 AND source_count = $4
		ORDER BY created_at DESC LIMIT 1`, story.ID, size, story.UpdatedAt, story.SourceCount).Scan(&reportID)
	if err == nil {
		return reportID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	items, err := service.store.evidence(ctx, story)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", errors.New("no public evidence for the story")
	}
	input, err := json.Marshal(map[string]any{
		"size": size, "target_words": sizeWords[size], "section": sectionName(story),
		"story_summary": valueOr(story.Summary), "outlets": len(story.Sources), "evidence": items,
	})
	if err != nil {
		return "", err
	}
	written, callID, err := service.draft(ctx, reportPrompt, string(input))
	if err != nil {
		return "", err
	}
	review, err := service.review(ctx, input, written)
	if err != nil {
		return "", err
	}
	revisions := 0
	if !review.printable() {
		feedback, _ := json.Marshal(map[string]any{"evidence_and_target": json.RawMessage(input), "draft": written, "issues": review.Issues})
		if revised, revisedCall, err := service.draft(ctx, reportRevisionPrompt, string(feedback)); err == nil {
			written, callID, revisions = revised, revisedCall, 1
			if review, err = service.review(ctx, input, written); err != nil {
				return "", err
			}
		}
	}
	if !review.printable() {
		return "", fmt.Errorf("report failed review (score %d): %s", review.Score, truncateRunes(review.Summary, 300))
	}
	paragraphs, err := json.Marshal(written.Paragraphs)
	if err != nil {
		return "", err
	}
	standfirst := written.Standfirst
	if size == SizeShort || size == SizeBrief {
		standfirst = ""
	}
	var eventID, articleID *string
	if story.Kind == KindEvent {
		eventID = &story.ID
	} else {
		articleID = &story.ID
	}
	err = service.store.pool.QueryRow(ctx, `
		INSERT INTO story_reports (event_id, article_id, size, headline, standfirst, paragraphs, word_count, illustration_prompt,
		                           story_updated_at, source_count, review_score, review_summary, revisions, llm_call_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) RETURNING id::text`,
		eventID, articleID, size, written.Headline, standfirst, paragraphs, countWords(written.Paragraphs), written.IllustrationPrompt,
		story.UpdatedAt, story.SourceCount, review.Score, truncateRunes(review.Summary, 600), revisions, callID).Scan(&reportID)
	return reportID, err
}

func (service *Service) draft(ctx context.Context, system, input string) (draft, *int64, error) {
	response, err := service.desk.Model.Complete(ctx, llm.Request{Task: "story_report", System: system, Input: input, JSONSchema: draftSchema})
	if err != nil {
		return draft{}, nil, err
	}
	if response.Provider == "none" || strings.TrimSpace(response.Text) == "" {
		return draft{}, nil, errors.New("report model unavailable")
	}
	var result draft
	if err := json.Unmarshal([]byte(response.Text), &result); err != nil {
		return draft{}, nil, fmt.Errorf("decode report: %w", err)
	}
	result.Headline = truncateRunes(result.Headline, 120)
	result.Standfirst = truncateRunes(result.Standfirst, 260)
	paragraphs := make([]string, 0, len(result.Paragraphs))
	for _, paragraph := range result.Paragraphs {
		if paragraph = strings.TrimSpace(paragraph); paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	}
	result.Paragraphs = paragraphs
	if result.Headline == "" || len(paragraphs) == 0 || !containsSinhala(result.Headline+strings.Join(paragraphs, "")) {
		return draft{}, nil, errors.New("report is empty or not in Sinhala")
	}
	if err := textcheck.Check("report", result.Headline+"\n"+result.Standfirst+"\n"+strings.Join(paragraphs, "\n")); err != nil {
		return draft{}, nil, err
	}
	var callID *int64
	if response.CallID > 0 {
		callID = &response.CallID
	}
	return result, callID, nil
}

func (service *Service) review(ctx context.Context, evidenceInput []byte, written draft) (reviewResult, error) {
	input, err := json.Marshal(map[string]any{"evidence_and_target": json.RawMessage(evidenceInput), "report": written})
	if err != nil {
		return reviewResult{}, err
	}
	response, err := service.desk.Model.Complete(ctx, llm.Request{Task: "story_report_review", System: reportReviewPrompt, Input: string(input), JSONSchema: reviewSchema})
	if err != nil {
		return reviewResult{}, err
	}
	if response.Provider == "none" || strings.TrimSpace(response.Text) == "" {
		return reviewResult{}, errors.New("review model unavailable")
	}
	var result reviewResult
	if err := json.Unmarshal([]byte(response.Text), &result); err != nil {
		return reviewResult{}, fmt.Errorf("decode review: %w", err)
	}
	return result, nil
}

// evidence returns up to six public reports on the story (one per outlet first), with the
// stored article text where the outlet's rights allow storing it.
func (store *Store) evidence(ctx context.Context, story Story) ([]evidence, error) {
	filter := `a.event_id = $2::uuid`
	if story.Kind == KindArticle {
		filter = `a.id = $2::uuid`
	}
	rows, err := store.pool.Query(ctx, `
		WITH q AS (`+publicArticles+` AND `+filter+`)
		SELECT DISTINCT ON (q.source_id) q.source_name, q.headline, q.published_at, COALESCE(q.summary, ''),
		       COALESCE(left(body.body_text, $3), '')
		FROM q LEFT JOIN article_contents body ON body.article_id = q.id AND body.current
		ORDER BY q.source_id, length(COALESCE(body.body_text, '')) DESC, q.published_at
		LIMIT $4`, false, story.ID, evidenceRunes, evidenceArticles)
	if err != nil {
		return nil, fmt.Errorf("load report evidence: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (evidence, error) {
		var item evidence
		err := row.Scan(&item.Outlet, &item.Headline, &item.PublishedAt, &item.Summary, &item.Text)
		return item, err
	})
}

func sectionName(story Story) string {
	if story.Category == nil {
		return ""
	}
	return story.Category.NameSI
}

func valueOr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func countWords(paragraphs []string) int {
	total := 0
	for _, paragraph := range paragraphs {
		total += len(strings.Fields(paragraph))
	}
	return total
}

func containsSinhala(value string) bool {
	for _, r := range value {
		if r >= 0x0D80 && r <= 0x0DFF {
			return true
		}
	}
	return false
}

var _ = utf8.RuneCountInString
