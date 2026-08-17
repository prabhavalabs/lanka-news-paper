package classify

import "strings"

type Result struct {
	Slug       string
	Confidence float64
	Model      string
}

type rule struct {
	slug     string
	aliases  []string
	keywords []string
}

// Ordered rules make ties deterministic. RSS aliases are deliberately separate
// from content keywords so publisher metadata can be cross-checked with the story.
var rules = []rule{
	{slug: "politics", aliases: []string{"දේශපාලන", "politics", "political"}, keywords: []string{"පාර්ලිමේන්තු", "ව්‍යවස්ථා", "ජනපති", "ජනාධිපති", "අගමැති", "අමාත්‍ය", "ඇමති", "මන්ත්‍රී", "මැතිවරණ", "ඡන්ද", "ආණ්ඩුව", "විපක්ෂ", "parliament", "president", "minister", "election"}},
	{slug: "economy", aliases: []string{"ආර්ථික", "ව්‍යාපාර", "කොටස් වෙළෙඳ", "economy", "business", "finance"}, keywords: []string{"ආර්ථික", "බදු", "බැංකු", "අයවැය", "උද්ධමන", "ආයෝජන", "කොටස් වෙළෙඳ", "රුපියල්", "ඩොලර්", "මිල", "economy", "budget", "inflation", "investment", "bank", "tax"}},
	{slug: "local", aliases: []string{"දේශීය", "ප්‍රාදේශීය", "local", "colombo"}, keywords: []string{"ශ්‍රී ලංකා", "කොළඹ", "මහනුවර", "යාපනය", "ගාල්ල", "මාතර", "අනුරාධපුර", "මඩකලපුව", "sri lanka", "colombo", "kandy", "jaffna"}},
	{slug: "world", aliases: []string{"විදෙස්", "ජාත්‍යන්තර", "ලෝක", "world", "international", "foreign"}, keywords: []string{"විදෙස්", "ජාත්‍යන්තර", "ඇමරිකා", "ඉන්දියා", "චීන", "රුසියා", "යුක්රේන", "ඊශ්‍රායල", "පලස්තීන", "ට‍්‍රම්ප්", "trump", "india", "china", "russia", "israel", "ukraine"}},
	{slug: "sport", aliases: []string{"ක්‍රීඩා", "sport", "sports"}, keywords: []string{"ක්‍රීඩා", "ක්‍රිකට්", "පාපන්දු", "ටෙස්ට්", "ශතක", "කඩුලු", "තරගාවලි", "cricket", "football", "match", "tournament"}},
	{slug: "crime", aliases: []string{"අධිකරණ", "නීතිය", "crime", "law", "court"}, keywords: []string{"අත්අඩංගුව", "රක්ෂිත බන්ධනාගාර", "ඝාතන", "මත්ද්‍රව්‍ය", "පොලිසි", "අධිකරණ", "උසාවි", "නඩුව", "අල්ලස්", "වංචා", "police", "court", "arrest", "murder", "fraud"}},
	{slug: "health", aliases: []string{"සෞඛ්‍ය", "health"}, keywords: []string{"සෞඛ්‍ය", "රෝහල", "වෛද්‍ය", "රෝග", "ඖෂධ", "ඩෙංගු", "hospital", "doctor", "covid", "dengue"}},
	{slug: "education", aliases: []string{"අධ්‍යාපන", "education"}, keywords: []string{"අධ්‍යාපන", "පාසල්", "විශ්වවිද්‍යාල", "විභාග", "ශිෂ්‍ය", "ගුරු", "school", "university", "exam", "student"}},
	{slug: "technology", aliases: []string{"තාක්ෂණ", "technology", "science"}, keywords: []string{"තාක්ෂණ", "ඩිජිටල්", "කෘත්‍රිම බුද්ධි", "සයිබර්", "ෆේස්බුක්", "ඉන්ස්ටග්‍රෑම්", "facebook", "instagram", "technology", "artificial intelligence", "cyber"}},
	{slug: "environment", aliases: []string{"පරිසර", "කාලගුණ", "environment", "weather"}, keywords: []string{"කාලගුණ", "ගංවතුර", "පරිසර", "නායයෑම්", "උණුසුම්", "සුළි කුණාටු", "weather", "flood", "cyclone", "climate"}},
	{slug: "entertainment", aliases: []string{"විනෝදාස්වාද", "කලා", "මීවිත", "සිනමා", "entertainment", "cinema"}, keywords: []string{"සිනමා", "නළු", "නිළි", "චිත්‍රපට", "සංගීත", "ගායක", "හොලිවුඩ්", "film", "cinema", "actor", "actress", "music", "hollywood"}},
	{slug: "official", aliases: []string{"නිල නිවේදන", "government", "official", "press release"}, keywords: []string{"නිල නිවේදන", "ගැසට්", "මාධ්‍ය නිවේදන", "press release", "gazette"}},
}

func From(publisherCategories []string, headline, description string) Result {
	publisher := publisherCategory(publisherCategories)
	content, score := contentCategory(headline, description)

	switch {
	case publisher != "" && content == publisher:
		return Result{Slug: content, Confidence: 0.96, Model: "rules-v2:feed+content"}
	case publisher != "" && content != "" && score >= 4:
		return Result{Slug: content, Confidence: 0.72, Model: "rules-v2:content-over-feed"}
	case publisher != "":
		return Result{Slug: publisher, Confidence: 0.84, Model: "rules-v2:feed"}
	case content != "":
		confidence := 0.58 + float64(score)*0.04
		if confidence > 0.90 {
			confidence = 0.90
		}
		return Result{Slug: content, Confidence: confidence, Model: "rules-v2:content"}
	default:
		return Result{Slug: "latest", Confidence: 0.30, Model: "rules-v2:fallback"}
	}
}

func ValidSlug(slug string) bool {
	if slug == "latest" {
		return true
	}
	for _, candidate := range rules {
		if candidate.slug == slug {
			return true
		}
	}
	return false
}

func publisherCategory(categories []string) string {
	for _, category := range categories {
		value := strings.ToLower(strings.TrimSpace(category))
		for _, candidate := range rules {
			for _, alias := range candidate.aliases {
				if strings.Contains(value, strings.ToLower(alias)) {
					return candidate.slug
				}
			}
		}
	}
	return ""
}

func contentCategory(headline, description string) (string, int) {
	headline = strings.ToLower(headline)
	description = strings.ToLower(description)
	best, bestScore := "", 0
	for _, candidate := range rules {
		score := 0
		for _, keyword := range candidate.keywords {
			keyword = strings.ToLower(keyword)
			if strings.Contains(headline, keyword) {
				score += 2
			}
			if strings.Contains(description, keyword) {
				score++
			}
		}
		if score > bestScore {
			best, bestScore = candidate.slug, score
		}
	}
	return best, bestScore
}
