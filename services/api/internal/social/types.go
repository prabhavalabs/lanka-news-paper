package social

import (
	"errors"
	"github.com/nipuntheekshana/lanka-news-paper/services/api/internal/storyline"
	"time"
)

var (
	ErrInvalid          = errors.New("invalid social publishing request")
	ErrNotFound         = errors.New("social publishing item not found")
	ErrConflict         = errors.New("social publishing item has changed or is already queued")
	ErrNotConfigured    = errors.New("Facebook publishing through Loomcast is not configured")
	ErrModelUnavailable = errors.New("social editorial model is unavailable")
)

type Settings struct {
	Enabled               bool      `json:"enabled"`
	AutoPublish           bool      `json:"auto_publish"`
	MaxPostsPerHour       int       `json:"max_posts_per_hour"`
	MaxPostsPerDay        int       `json:"max_posts_per_day"`
	MinGapMinutes         int       `json:"min_gap_minutes"`
	QuietHoursStart       int       `json:"quiet_hours_start"`
	QuietHoursEnd         int       `json:"quiet_hours_end"`
	MinScore              int       `json:"min_score"`
	LookbackHours         int       `json:"lookback_hours"`
	CandidatesPerRun      int       `json:"candidates_per_run"`
	IncludeSingleArticles bool      `json:"include_single_articles"`
	Timezone              string    `json:"timezone"`
	RealtimeEnabled       bool      `json:"realtime_enabled"`
	MinReviewScore        int       `json:"min_review_score"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type Spectrum struct {
	Left   int `json:"left"`
	Center int `json:"center"`
	Right  int `json:"right"`
}

// PublishChannel is the Loomcast channel posts go out on: the Thulawa News Facebook Page.
type PublishChannel struct {
	ID               string `json:"id"`
	Platform         string `json:"platform"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	Access           string `json:"access"`
	DryRun           bool   `json:"dry_run"`
	RequiresApproval bool   `json:"requires_approval"`
}

type Post struct {
	Storyline            *StorylineInfo `json:"storyline,omitempty"`
	ID                   string         `json:"id"`
	SubjectKind          string         `json:"subject_kind"`
	EventID              *string        `json:"event_id"`
	ArticleID            *string        `json:"article_id"`
	SubjectTitle         string         `json:"subject_title"`
	Category             string         `json:"category"`
	Status               string         `json:"status"`
	Score                int            `json:"score"`
	SelectionReason      string         `json:"selection_reason"`
	Kind                 string         `json:"kind"`
	HeadlineSI           string         `json:"headline_si"`
	SummarySI            string         `json:"summary_si"`
	Caption              string         `json:"caption"`
	Hashtags             []string       `json:"hashtags"`
	LinkURL              string         `json:"link_url"`
	SourceNames          []string       `json:"source_names"`
	SourceCount          int            `json:"source_count"`
	Spectrum             *Spectrum      `json:"spectrum"`
	ImageKey             *string        `json:"image_key"`
	ImageURL             string         `json:"image_url"`
	ImageRenderedAt      *time.Time     `json:"image_rendered_at"`
	ScheduledFor         *time.Time     `json:"scheduled_for"`
	ApprovedBy           *string        `json:"approved_by"`
	ApprovedAt           *time.Time     `json:"approved_at"`
	RejectedBy           *string        `json:"rejected_by"`
	RejectedAt           *time.Time     `json:"rejected_at"`
	PublishedAt          *time.Time     `json:"published_at"`
	FBPostID             *string        `json:"fb_post_id"`
	FBPermalink          *string        `json:"fb_permalink"`
	LoomcastPostID       *string        `json:"loomcast_post_id"`
	HandedOffAt          *time.Time     `json:"handed_off_at"`
	Attempts             int            `json:"attempts"`
	LastError            *string        `json:"last_error"`
	LLMCallID            *int64         `json:"llm_call_id"`
	EvaluatedSourceCount int            `json:"evaluated_source_count"`
	CreatedBy            string         `json:"created_by"`
	Channel              string         `json:"channel"`
	Trigger              string         `json:"trigger"`
	ReviewStatus         string         `json:"review_status"`
	ReviewScore          *int           `json:"review_score"`
	ReviewSummary        string         `json:"review_summary"`
	ReviewIssues         []ReviewIssue  `json:"review_issues"`
	ReviewLLMCallID      *int64         `json:"review_llm_call_id"`
	ReviewedAt           *time.Time     `json:"reviewed_at"`
	HoldReason           *string        `json:"hold_reason"`
	ReviewRevisions      int            `json:"review_revisions"`
	Plan                 *PublishPlan   `json:"plan,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

// PublishPlan tells editors whether and when a post will go out.
// State is one of published, publishing, scheduled, paused, in_review, needs_editor or not_publishing.
type PublishPlan struct {
	State     string     `json:"state"`
	At        *time.Time `json:"at"`
	Estimated bool       `json:"estimated"`
	Note      string     `json:"note"`
}

type EditInput struct {
	HeadlineSI *string   `json:"headline_si"`
	SummarySI  *string   `json:"summary_si"`
	Caption    *string   `json:"caption"`
	Hashtags   *[]string `json:"hashtags"`
}

type PostList struct {
	Items      []Post `json:"items"`
	NextCursor string `json:"next_cursor"`
}

type Status struct {
	// Configured means a Loomcast API key is set; Ready means posts can go out now.
	Configured bool            `json:"configured"`
	Ready      bool            `json:"ready"`
	Channel    *PublishChannel `json:"channel"`
	// Problem says why posts cannot go out; Warning flags a setup that still publishes.
	Problem               string         `json:"problem,omitempty"`
	Warning               string         `json:"warning,omitempty"`
	PublishRetryAt        *time.Time     `json:"publish_retry_at"`
	Settings              Settings       `json:"settings"`
	Counts                map[string]int `json:"counts"`
	NextPublishEligibleAt *time.Time     `json:"next_publish_eligible_at"`
}

type Candidate struct {
	Storyline          *StorylineInfo `json:"storyline,omitempty"`
	EvidenceArticleIDs []string       `json:"-"`
	ID                 string         `json:"id"`
	SubjectKind        string         `json:"subject_kind"`
	Title              string         `json:"title"`
	Category           string         `json:"category"`
	CategorySlug       string         `json:"category_slug"`
	SourceNames        []string       `json:"source_names"`
	SourceCount        int            `json:"source_count"`
	ArticleCount       int            `json:"article_count"`
	Breaking           bool           `json:"breaking"`
	LatestAt           time.Time      `json:"latest_at"`
	Summary            string         `json:"synthesis_summary"`
	MemberSummaries    []string       `json:"member_summaries"`
	Spectrum           *Spectrum      `json:"spectrum"`
	LinkURL            string         `json:"link_url"`
}

type StorylineInfo = storyline.Reference
