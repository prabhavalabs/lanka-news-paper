package classify

import "strings"

var keywords = map[string][]string{
	"politics":      {"රජය", "අමාත්‍ය", "පාර්ලිමේන්තු", "ඡන්ද", "president", "minister", "election"},
	"economy":       {"ආර්ථික", "බදු", "බැංකු", "මිල", "budget", "inflation", "tax"},
	"sport":         {"ක්‍රීඩා", "ක්‍රිකට්", "පාපන්දු", "cricket", "football", "match"},
	"world":         {"ලෝක", "ඇමරිකා", "ඉන්දියා", "china", "israel", "ukraine"},
	"crime":         {"අත්අඩංගුව", "ඝාතන", "උසාවි", "police", "court", "arrest"},
	"health":        {"සෞඛ්‍ය", "රෝහල", "වෛද්‍ය", "hospital", "covid", "dengue"},
	"education":     {"අධ්‍යාපන", "පාසල", "විශ්වවිද්‍යාල", "school", "exam"},
	"technology":    {"තාක්ෂණ", "ඩිජිටල්", "tech", "ai", "cyber"},
	"environment":   {"කාලගුණ", "ගංවතුර", "පරිසර", "weather", "flood", "cyclone"},
	"entertainment": {"සිනමා", "සංගීත", "film", "cinema", "music"},
	"official":      {"නිල නිවේදන", "press release", "gazette"},
}

func From(publisherCategories []string, headline string) (slug string, confidence float64) {
	blob := strings.ToLower(strings.Join(append(publisherCategories, headline), " "))
	best, score := "local", 0
	for slug, words := range keywords {
		hits := 0
		for _, word := range words {
			if strings.Contains(blob, strings.ToLower(word)) {
				hits++
			}
		}
		if hits > score {
			best, score = slug, hits
		}
	}
	if score == 0 {
		return "latest", 0.3
	}
	return best, 0.55 + 0.1*float64(score)
}
