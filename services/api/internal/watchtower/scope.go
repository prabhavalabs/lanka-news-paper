package watchtower

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var relativeWindowPattern = regexp.MustCompile(`(?i)\b(?:last|past|previous)\s+(\d+)\s*(hour|hours|hr|hrs|day|days)\b`)

func ParseSearchScope(question string, now time.Time) SearchScope {
	question = strings.TrimSpace(question)
	lower := strings.ToLower(question)
	scope := SearchScope{To: now, From: now.Add(-7 * 24 * time.Hour), Label: "Last 7 days"}

	if match := relativeWindowPattern.FindStringSubmatch(lower); len(match) == 3 {
		amount, _ := strconv.Atoi(match[1])
		unit := 24 * time.Hour
		labelUnit := "days"
		if strings.HasPrefix(match[2], "h") {
			unit = time.Hour
			labelUnit = "hours"
		}
		if amount == 1 {
			labelUnit = strings.TrimSuffix(labelUnit, "s")
		}
		scope.From = now.Add(-time.Duration(amount) * unit)
		scope.Label = fmt.Sprintf("Last %d %s", amount, labelUnit)
	} else if strings.Contains(lower, "today") {
		location, err := time.LoadLocation("Asia/Colombo")
		if err != nil {
			location = time.UTC
		}
		localNow := now.In(location)
		scope.From = time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location).UTC()
		scope.Label = "Today"
	} else if strings.Contains(lower, "yesterday") {
		location, err := time.LoadLocation("Asia/Colombo")
		if err != nil {
			location = time.UTC
		}
		localNow := now.In(location)
		startToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
		scope.From = startToday.Add(-24 * time.Hour).UTC()
		scope.To = startToday.UTC()
		scope.Label = "Yesterday"
	} else if strings.Contains(lower, "last month") || strings.Contains(lower, "past month") {
		scope.From = now.Add(-30 * 24 * time.Hour)
		scope.Label = "Last 30 days"
	} else if strings.Contains(lower, "all time") || strings.Contains(lower, "entire history") {
		scope.From = now.Add(-10 * 365 * 24 * time.Hour)
		scope.Label = "All available history"
	}

	scope.Category = categoryFromQuestion(lower)
	scope.CategoryOnly = scope.Category != ""
	scope.Terms = meaningfulTerms(question)
	scope.ExcludeWorld = strings.Contains(lower, "in sri lanka") || strings.Contains(lower, "across sri lanka")
	return scope
}

func categoryFromQuestion(question string) string {
	categories := []struct {
		slug  string
		terms []string
	}{
		{slug: "politics", terms: []string{"politic", "election", "government", "parliament"}},
		{slug: "economy", terms: []string{"econom", "business", "finance", "inflation", "market"}},
		{slug: "crime", terms: []string{"crime", "police", "court", "arrest"}},
		{slug: "health", terms: []string{"health", "hospital", "medical", "disease"}},
		{slug: "sport", terms: []string{"sport", "cricket", "football", "rugby"}},
		{slug: "education", terms: []string{"education", "school", "university", "exam"}},
		{slug: "technology", terms: []string{"technology", "tech", "digital", "cyber"}},
		{slug: "environment", terms: []string{"environment", "weather", "climate", "flood"}},
		{slug: "entertainment", terms: []string{"entertainment", "cinema", "film", "music"}},
	}
	for _, category := range categories {
		for _, term := range category.terms {
			if strings.Contains(question, term) {
				return category.slug
			}
		}
	}
	return ""
}

func meaningfulTerms(question string) []string {
	stopWords := map[string]bool{
		"a": true, "about": true, "all": true, "and": true, "anything": true, "are": true,
		"biggest": true, "can": true, "coverage": true, "current": true, "did": true, "do": true, "for": true, "from": true, "happened": true,
		"happening": true, "has": true, "have": true, "hours": true, "how": true, "in": true,
		"important": true, "is": true, "lanka": true, "last": true, "latest": true, "me": true,
		"news": true, "of": true, "on": true, "past": true, "please": true, "previous": true,
		"sri": true, "stories": true, "tell": true, "the": true, "today": true, "what": true,
		"this": true, "week": true, "widest": true, "which": true, "who": true, "why": true, "with": true, "yesterday": true,
	}
	seen := make(map[string]bool)
	terms := make([]string, 0, 8)
	for _, token := range strings.FieldsFunc(strings.ToLower(question), func(character rune) bool {
		return !unicode.IsLetter(character) && !unicode.IsNumber(character)
	}) {
		if len([]rune(token)) < 3 || stopWords[token] || seen[token] {
			continue
		}
		if _, err := strconv.Atoi(token); err == nil {
			continue
		}
		seen[token] = true
		terms = append(terms, token)
		if len(terms) == 8 {
			break
		}
	}
	return terms
}
