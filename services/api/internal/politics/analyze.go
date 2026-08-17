package politics

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
)

const Model = "political-framing-rules-v1"

type Party struct {
	Slug       string
	Position   float64
	Confidence float64
	Aliases    []string
}

type Mention struct {
	PartySlug  string   `json:"party_slug"`
	Stance     float64  `json:"stance"`
	Confidence float64  `json:"confidence"`
	Terms      []string `json:"terms"`
}

type Analysis struct {
	EconomicFrame float64
	Confidence    float64
	Mentions      []Mention
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

var favorableTerms = map[string]struct{}{
	"achievement": {}, "commended": {}, "confidence": {}, "progress": {}, "praised": {},
	"reform": {}, "strong": {}, "success": {}, "successful": {}, "support": {}, "victory": {}, "wins": {},
	"ජය": {}, "ප්‍රගතිය": {}, "ප්‍රශංසා": {}, "සාර්ථක": {}, "සහාය": {}, "විශ්වාසය": {}, "ශක්තිමත්": {},
}

var criticalTerms = map[string]struct{}{
	"accused": {}, "arrested": {}, "blamed": {}, "corruption": {}, "crisis": {}, "criticised": {},
	"criticized": {}, "failed": {}, "failure": {}, "fraud": {}, "protest": {}, "rejected": {}, "scandal": {},
	"අර්බුද": {}, "අසාර්ථක": {}, "අත්අඩංගුව": {}, "චෝදනා": {}, "දූෂණ": {}, "ප්‍රතික්ෂේප": {}, "වංචා": {}, "විරෝධය": {},
}

func Analyze(parties []Party, headline, description string) Analysis {
	tokens := tokenize(headline + " " + description)
	result := Analysis{Mentions: make([]Mention, 0)}
	weightedFrame, totalWeight := 0.0, 0.0
	for _, party := range parties {
		positions := mentionPositions(tokens, party.Aliases)
		if len(positions) == 0 {
			continue
		}
		positive, negative, evidence := 0, 0, make(map[string]struct{})
		for position := range positions {
			start, end := max(0, position-4), min(len(tokens), position+5)
			for _, token := range tokens[start:end] {
				if _, ok := favorableTerms[token]; ok {
					positive++
					evidence[token] = struct{}{}
				}
				if _, ok := criticalTerms[token]; ok {
					negative++
					evidence[token] = struct{}{}
				}
			}
		}
		stance, confidence := 0.0, 0.3*party.Confidence
		if total := positive + negative; total > 0 {
			stance = float64(positive-negative) / float64(total)
			confidence = math.Min(0.92, 0.45+0.08*float64(min(len(positions), 3))+0.05*float64(min(total, 4))) * party.Confidence
		}
		terms := make([]string, 0, len(evidence))
		for term := range evidence {
			terms = append(terms, term)
		}
		result.Mentions = append(result.Mentions, Mention{
			PartySlug: party.Slug, Stance: stance, Confidence: confidence, Terms: terms,
		})
		weight := confidence * float64(len(positions))
		weightedFrame += party.Position * stance * weight
		totalWeight += weight
		if confidence > result.Confidence {
			result.Confidence = confidence
		}
	}
	if totalWeight > 0 {
		result.EconomicFrame = math.Max(-1, math.Min(1, weightedFrame/totalWeight))
	}
	return result
}

func (store *Store) Backfill(ctx context.Context, limit int) error {
	parties, err := store.parties(ctx)
	if err != nil {
		return err
	}
	rows, err := store.pool.Query(ctx, `
		SELECT a.id::text, a.headline, COALESCE(a.description, '')
		FROM articles a
		LEFT JOIN article_political_analysis analysis ON analysis.article_id = a.id
		WHERE a.public_status = 'published'
		  AND (analysis.article_id IS NULL OR analysis.model <> $1)
		ORDER BY a.published_at, a.id
		LIMIT $2
	`, Model, limit)
	if err != nil {
		return fmt.Errorf("list articles for political analysis: %w", err)
	}
	type article struct{ id, headline, description string }
	articles := make([]article, 0, limit)
	for rows.Next() {
		var item article
		if err := rows.Scan(&item.id, &item.headline, &item.description); err != nil {
			rows.Close()
			return err
		}
		articles = append(articles, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, article := range articles {
		analysis := Analyze(parties, article.headline, article.description)
		mentions, err := json.Marshal(analysis.Mentions)
		if err != nil {
			return fmt.Errorf("encode political analysis for %s: %w", article.id, err)
		}
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO article_political_analysis (article_id, model, economic_frame, confidence, mentions)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (article_id) DO UPDATE SET
			  model = EXCLUDED.model,
			  economic_frame = EXCLUDED.economic_frame,
			  confidence = EXCLUDED.confidence,
			  mentions = EXCLUDED.mentions,
			  analyzed_at = clock_timestamp()
		`, article.id, Model, analysis.EconomicFrame, analysis.Confidence, mentions); err != nil {
			return fmt.Errorf("save political analysis for %s: %w", article.id, err)
		}
	}
	return nil
}

func (store *Store) parties(ctx context.Context) ([]Party, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT slug, economic_position, confidence, aliases
		FROM political_parties WHERE active ORDER BY economic_position
	`)
	if err != nil {
		return nil, fmt.Errorf("list political parties: %w", err)
	}
	defer rows.Close()
	items := make([]Party, 0)
	for rows.Next() {
		var item Party
		if err := rows.Scan(&item.Slug, &item.Position, &item.Confidence, &item.Aliases); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func mentionPositions(tokens []string, aliases []string) map[int]struct{} {
	positions := make(map[int]struct{})
	for _, alias := range aliases {
		phrase := tokenize(alias)
		if len(phrase) == 0 || len(phrase) > len(tokens) {
			continue
		}
		for index := 0; index <= len(tokens)-len(phrase); index++ {
			if equalTokens(tokens[index:index+len(phrase)], phrase) {
				positions[index] = struct{}{}
			}
		}
	}
	return positions
}

func equalTokens(left, right []string) bool {
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func tokenize(value string) []string {
	fields := strings.Fields(strings.ToLower(value))
	tokens := fields[:0]
	for _, field := range fields {
		field = strings.TrimFunc(field, func(value rune) bool {
			return !unicode.IsLetter(value) && !unicode.IsNumber(value) && !unicode.IsMark(value)
		})
		if field != "" {
			tokens = append(tokens, field)
		}
	}
	return tokens
}
