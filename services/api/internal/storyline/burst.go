package storyline

import (
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Headline struct {
	ID, Text string
	At       time.Time
}
type BurstTerm struct {
	Term          string  `json:"term"`
	Articles24h   int     `json:"articles_24h"`
	BaselineDaily float64 `json:"baseline_daily"`
	Acceleration  float64 `json:"acceleration"`
}

var stopwords = map[string]bool{"සඳහා": true, "සමග": true, "සමඟ": true, "විසින්": true, "බව": true, "කරන": true, "කළ": true, "වෙයි": true, "වෙත": true, "ගැන": true, "සිට": true, "පසු": true, "අද": true, "ඇති": true, "the": true, "and": true, "for": true, "with": true, "from": true}

func BurstTerms(headlines []Headline, now time.Time) []BurstTerm {
	type counts struct{ current, previous map[string]bool }
	terms := map[string]*counts{}
	for _, h := range headlines {
		if h.At.After(now) || h.At.Before(now.Add(-8*24*time.Hour)) {
			continue
		}
		for _, term := range strings.FieldsFunc(strings.ToLower(h.Text), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && r != '\u200d'
		}) {
			if utf8.RuneCountInString(term) < 3 || stopwords[term] {
				continue
			}
			if terms[term] == nil {
				terms[term] = &counts{map[string]bool{}, map[string]bool{}}
			}
			if !h.At.Before(now.Add(-24 * time.Hour)) {
				terms[term].current[h.ID] = true
			} else {
				terms[term].previous[h.ID] = true
			}
		}
	}
	result := make([]BurstTerm, 0)
	for term, c := range terms {
		if len(c.current) < 3 {
			continue
		}
		baseline := float64(len(c.previous)) / 7
		result = append(result, BurstTerm{term, len(c.current), baseline, float64(len(c.current)) / max(1, baseline)})
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Acceleration != b.Acceleration {
			return a.Acceleration > b.Acceleration
		}
		if a.Articles24h != b.Articles24h {
			return a.Articles24h > b.Articles24h
		}
		return a.Term < b.Term
	})
	if len(result) > 15 {
		result = result[:15]
	}
	return result
}
