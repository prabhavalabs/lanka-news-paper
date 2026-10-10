package social

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

func truncate(value string, maximum int) string {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return strings.TrimSpace(string([]rune(value)[:maximum-1])) + "…"
}

func rankCandidates(candidates []Candidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Breaking != b.Breaking {
			return a.Breaking
		}
		if a.SourceCount != b.SourceCount {
			return a.SourceCount > b.SourceCount
		}
		if a.ArticleCount != b.ArticleCount {
			return a.ArticleCount > b.ArticleCount
		}
		if !a.LatestAt.Equal(b.LatestAt) {
			return a.LatestAt.After(b.LatestAt)
		}
		return a.ID < b.ID
	})
}

func reevaluateSkipped(candidate Candidate, previousSources int, previousBreaking bool) bool {
	return (candidate.Breaking && !previousBreaking) || candidate.SourceCount >= previousSources+2
}

func (store *Store) Candidates(ctx context.Context, s Settings, now time.Time, baseURL, eventID, articleID string) ([]Candidate, error) {
	manual := eventID != "" || articleID != ""
	if manual {
		if (eventID == "") == (articleID == "") {
			return nil, ErrInvalid
		}
		id := eventID
		if id == "" {
			id = articleID
		}
		if _, err := uuid.Parse(id); err != nil {
			return nil, ErrInvalid
		}
	}
	rows, err := store.pool.Query(ctx, `
 WITH qualifying AS (
 SELECT a.*,source.name AS source_name,NULLIF(trim(d.summary_text),'') AS summary_text
 FROM articles a JOIN sources source ON source.id=a.source_id
 JOIN rights_profiles rights ON rights.id=a.rights_profile_id
 LEFT JOIN categories category ON category.id=a.category_id
 LEFT JOIN event_clusters parent_event ON parent_event.id=a.event_id
 LEFT JOIN categories parent_category ON parent_category.id=parent_event.category_id
 LEFT JOIN article_analysis_documents d ON d.article_id=a.id
 WHERE a.public_status='published' AND source.active
 AND rights.mode NOT IN ('disabled','internal_verification')
 AND (rights.expires_at IS NULL OR rights.expires_at>$1)
 AND (category.id IS NULL OR NOT category.held)
 AND (parent_category.id IS NULL OR NOT parent_category.held)
 ), subjects AS (
 SELECT e.id,'event'::text AS subject_kind,e.display_title AS title,
 COALESCE(category.name_si,'වෙනත්') AS category,e.is_breaking AS breaking,e.last_update_at AS latest_at,
 count(DISTINCT a.id)::int AS article_count,count(DISTINCT a.source_id)::int AS source_count,
 array_agg(DISTINCT a.source_name ORDER BY a.source_name) AS source_names,
 array_agg(DISTINCT a.id ORDER BY a.id) AS evidence_article_ids,
 COALESCE(CASE WHEN NOT EXISTS (SELECT 1 FROM articles member WHERE member.event_id=e.id
 AND NOT EXISTS (SELECT 1 FROM qualifying permitted WHERE permitted.id=member.id))
 THEN NULLIF(trim(analysis.summary),'') END,min(a.summary_text)) AS summary,
 ARRAY(SELECT recent.summary_text FROM qualifying recent WHERE recent.event_id=e.id AND recent.summary_text IS NOT NULL ORDER BY recent.published_at DESC,recent.id LIMIT 3) AS member_summaries,
 CASE WHEN analysis.rated_source_count>0 AND NOT EXISTS (SELECT 1 FROM articles member WHERE member.event_id=e.id
 AND NOT EXISTS (SELECT 1 FROM qualifying permitted WHERE permitted.id=member.id)) THEN jsonb_build_object('left',round(analysis.left_percentage)::int,
 'center',round(analysis.center_percentage)::int,'right',round(analysis.right_percentage)::int) END AS spectrum
 FROM event_clusters e JOIN qualifying a ON a.event_id=e.id
 LEFT JOIN categories category ON category.id=e.category_id
 LEFT JOIN event_narrative_analyses analysis ON analysis.event_id=e.id
 WHERE (category.id IS NULL OR NOT category.held)
 AND (($2::boolean AND e.id::text=$3) OR (NOT $2 AND e.last_update_at >= $5))
 GROUP BY e.id,category.name_si,analysis.summary,analysis.rated_source_count,
 analysis.left_percentage,analysis.center_percentage,analysis.right_percentage
 HAVING ($2 OR e.is_breaking OR count(DISTINCT a.source_id)>=2)
 AND COALESCE(CASE WHEN NOT EXISTS (SELECT 1 FROM articles member WHERE member.event_id=e.id
 AND NOT EXISTS (SELECT 1 FROM qualifying permitted WHERE permitted.id=member.id))
 THEN NULLIF(trim(analysis.summary),'') END,min(a.summary_text)) IS NOT NULL
 UNION ALL
 SELECT a.id,'article',a.headline,COALESCE(category.name_si,'වෙනත්'),false,a.published_at,
 1,1,ARRAY[a.source_name],ARRAY[a.id],a.summary_text,ARRAY[a.summary_text],NULL::jsonb
 FROM qualifying a LEFT JOIN categories category ON category.id=a.category_id
 WHERE a.summary_text IS NOT NULL
 AND (($2 AND a.id::text=$4) OR (NOT $2 AND $6::boolean AND a.event_id IS NULL AND a.published_at >= $5))
 )
 SELECT to_jsonb(s) FROM subjects s
 WHERE NOT EXISTS (SELECT 1 FROM social_posts p WHERE
 ((s.subject_kind='event' AND p.event_id=s.id) OR (s.subject_kind='article' AND p.article_id=s.id))
 AND p.status NOT IN ('rejected','skipped','failed'))
 AND ($2 OR NOT EXISTS (
 SELECT 1 FROM (SELECT p.evaluated_source_count,p.evaluated_breaking FROM social_posts p
 WHERE p.status='skipped' AND ((s.subject_kind='event' AND p.event_id=s.id) OR (s.subject_kind='article' AND p.article_id=s.id))
 ORDER BY p.created_at DESC,p.id DESC LIMIT 1) previous
 WHERE NOT (s.breaking AND NOT previous.evaluated_breaking) AND s.source_count<previous.evaluated_source_count+2
 ))
 ORDER BY breaking DESC,source_count DESC,article_count DESC,latest_at DESC,id
 LIMIT $7`, now, manual, eventID, articleID, now.Add(-time.Duration(s.LookbackHours)*time.Hour), s.IncludeSingleArticles, s.CandidatesPerRun)
	if err != nil {
		return nil, fmt.Errorf("select social candidates: %w", err)
	}
	defer rows.Close()
	result := make([]Candidate, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		// Keep the evidence snapshot out of the model input.
		var record struct {
			Candidate
			Summary            string   `json:"summary"`
			EvidenceArticleIDs []string `json:"evidence_article_ids"`
		}
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, err
		}
		candidate := record.Candidate
		candidate.EvidenceArticleIDs = record.EvidenceArticleIDs
		candidate.Summary = truncate(record.Summary, 1400)
		for i := range candidate.MemberSummaries {
			candidate.MemberSummaries[i] = truncate(candidate.MemberSummaries[i], 650)
		}
		prefix := "/e/"
		if candidate.SubjectKind == "article" {
			prefix = "/a/"
		}
		candidate.LinkURL = strings.TrimRight(baseURL, "/") + prefix + candidate.ID
		result = append(result, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rankCandidates(result)
	return result, nil
}
