export type SourceType =
  | "private_media"
  | "state_owned"
  | "government"
  | "independent"
  | "international"
  | "other";

export type PublicSource = {
  id: string;
  name: string;
  type: SourceType;
  /** Outlet logo URL readers can load, when the outlet has one. */
  icon?: string;
  description?: string;
  website?: string;
};

export type PublicCategory = {
  slug: string;
  name_si: string;
};

export type PublicArticle = {
  id: string;
  headline: string;
  source: PublicSource;
  category: PublicCategory | null;
  published_at: string;
  received_at: string;
  original_url: string;
  excerpt: string | null;
  media: string | null;
  event_id: string | null;
  editorial_note: string | null;
  analysis?: ArticleNarrativeAnalysis;
};

export type ArticleNarrativeAnalysis = {
  summary: string;
  relevant: boolean;
  label: string;
  left_probability: number;
  center_probability: number;
  right_probability: number;
  confidence: number;
};

export type EventSourceSpectrum = {
  article_id: string;
  source_id: string;
  source: string;
  source_icon: string;
  headline: string;
  original_url: string;
  label: "left" | "center" | "right" | "unrated";
  left_probability: number;
  center_probability: number;
  right_probability: number;
  confidence: number;
};

export type EventNarrativeAnalysis = {
  summary: string;
  article_count: number;
  source_count: number;
  rated_source_count: number;
  left_percentage: number;
  center_percentage: number;
  right_percentage: number;
  source_spectrum: EventSourceSpectrum[];
  analyzed_at: string;
};

/** One outlet's report inside a front-page story, with the link to the original article. */
export type StorySource = {
  id: string;
  name: string;
  type: SourceType;
  /** Outlet logo URL readers can load; empty when the outlet has none. Show it in greyscale. */
  icon: string;
  article_id: string;
  headline: string;
  original_url: string;
  published_at: string;
};

export type FrontPageSlot = "lead" | "top" | "more";

/** A story on the front page: an event covered by one or more outlets, or a single article. */
export type FrontPageStory = {
  kind: "event" | "article";
  id: string;
  headline: string;
  summary: string | null;
  category: PublicCategory | null;
  breaking: boolean;
  published_at: string;
  updated_at: string;
  source_count: number;
  sources: StorySource[];
  /** True when the chief editor placed it; false when it filled an open slot automatically. */
  curated: boolean;
  slot: FrontPageSlot;
  editor_note: string | null;
  media: string | null;
};

export type EditorialSummary = {
  id: string;
  title: string;
  standfirst: string;
  author_name: string;
  published_at: string;
};

export type Editorial = EditorialSummary & {
  body: string;
  event_id: string | null;
  updated_at: string;
};

/** Block sizes the AI layout editor chooses from, largest first. */
export type EditionBlockSize = "lead" | "feature" | "standard" | "short" | "brief";

/** A complete report written by the AI desk from every outlet's coverage, sized to its block. */
export type StoryReport = {
  headline: string;
  /** One-sentence summary under the headline; empty for short sizes. */
  standfirst: string;
  /** Plain-text paragraphs, in order. */
  paragraphs: string[];
  word_count: number;
  written_at: string;
};

/** A generated black-and-white illustration. Never a photograph; always captioned as such. */
export type StoryImage = {
  url: string;
  alt: string;
  caption: string;
  kind: "illustration";
  width: number;
  height: number;
};

export type EditionBlock = {
  size: EditionBlockSize;
  story: FrontPageStory;
  /** Null until the desk has written this story; show the story summary instead. */
  report: StoryReport | null;
  image: StoryImage | null;
};

export type FrontPage = {
  /** Ordered blocks of the AI-built edition. Empty when no edition has been built yet. */
  blocks: EditionBlock[];
  edition_id: string | null;
  edition_date: string;
  generated_at: string;
  lead: FrontPageStory | null;
  top: FrontPageStory[];
  more: FrontPageStory[];
  editorial: EditorialSummary | null;
  /** True when at least one slot was placed by the chief editor. */
  curated: boolean;
};

export type EditorialStatus = "draft" | "published" | "archived";

export type AdminEditorial = Omit<Editorial, "published_at"> & {
  status: EditorialStatus;
  /** Null until first published; kept when an editorial is unpublished or archived. */
  published_at: string | null;
  created_at: string;
};

export type EditorialInput = {
  title: string;
  standfirst: string;
  body: string;
  author_name: string;
  event_id?: string | null;
};

export type FrontPagePlacement = {
  slot: FrontPageSlot;
  rank: number;
  story: FrontPageStory;
  note: string | null;
  expires_at: string | null;
  placed_by: string;
  placed_at: string;
};

export type FrontPagePlacementInput = {
  slot: FrontPageSlot;
  kind: "event" | "article";
  id: string;
  note?: string | null;
  expires_at?: string | null;
};

export type FrontPageSource = {
  id: string;
  name: string;
  type: SourceType;
  front_page: boolean;
};

export type AdminFrontPage = {
  placements: FrontPagePlacement[];
  candidates: FrontPageStory[];
  sources: FrontPageSource[];
  preview: FrontPage;
};

export type CursorPage<T> = {
  items: T[];
  next_cursor: string | null;
};

export type PublicEvent = {
  id: string;
  title: string;
  is_breaking: boolean;
  articles: PublicArticle[];
  analysis?: EventNarrativeAnalysis;
};

export type PublicKnowledgeArticle = {
  id: string;
  headline: string;
  source_id: string;
  source: string;
  published_at: string;
  narrative?: {
    label: string;
    economic_frame: number;
    left_probability: number;
    center_probability: number;
    right_probability: number;
    axis_version: string;
    confidence: number;
  };
};

export type PublicKnowledgeEvent = {
  id: string;
  title: string;
  category: string;
  category_name_si: string;
  is_breaking: boolean;
  last_update_at: string;
  articles: PublicKnowledgeArticle[];
  analysis?: EventNarrativeAnalysis;
};

export type PublicKnowledgeGraph = {
  generated_at: string;
  days: number;
  summary: {
    articles: number;
    events: number;
    multi_source_events: number;
    sources: number;
  };
  categories: {
    slug: string;
    name_si: string;
    name_en: string;
    articles: number;
    events: number;
  }[];
  events: PublicKnowledgeEvent[];
};

export type AdminSource = {
  id: string;
  name: string;
  legal_name: string;
  source_type: SourceType;
  website: string;
  icon_url: string;
  description: string;
  active: boolean;
  published_article_count: number;
  latest_published_at: string | null;
};

export type AdminSourceInput = Omit<
  AdminSource,
  "id" | "published_article_count" | "latest_published_at"
>;

export type AdminEndpoint = {
  id: string;
  source_id: string;
  endpoint_type: string;
  url: string;
  paused: boolean;
  health_state: string;
  last_error: string | null;
  last_success_at: string | null;
  polling_interval_seconds: number;
  verified_official: boolean;
  last_latency_ms: number | null;
  last_item_count: number;
  last_new_item_count: number;
  total_captured: number;
};

export type AdminRights = {
  id: string;
  source_id: string;
  endpoint_id: string;
  mode: string;
  attribution: string;
};

export type AdminCollectionConfig = {
  discovery_urls: string[];
  allowed_hosts: string[];
  article_url_patterns: string[];
  link_selector: string;
  title_selector: string;
  published_selector: string;
  author_selector: string;
  content_selector: string;
  exclude_selectors: string[];
  pagination_mode: "none" | "next_link" | "page_parameter";
  next_page_selector: string;
  page_parameter: string;
  user_agent: string;
  min_content_characters: number;
  minimum_sinhala_ratio: number;
};

export type AdminCollectionProfile = {
  id: string;
  source_id: string;
  endpoint_id: string;
  version: number;
  discovery_method: "rss" | "atom" | "json_feed" | "rest_api" | "sitemap" | "listing_page" | "webhook" | "youtube";
  article_method: "metadata_only" | "feed_content" | "api_content" | "html_static";
  config: AdminCollectionConfig;
  min_delay_seconds: number;
  max_requests_per_run: number;
  max_pages: number;
  request_timeout_seconds: number;
  created_by: string;
  activated_at: string | null;
  created_at: string;
};

export type AdminComplianceReview = {
  id: string;
  source_id: string;
  version: number;
  status: "pending" | "approved" | "restricted" | "denied";
  robots_url: string;
  robots_checked_at: string | null;
  robots_allowed: boolean | null;
  terms_urls: string[];
  allow_discovery: boolean;
  allow_full_text_storage: boolean;
  allow_ai_processing: boolean;
  allow_embeddings: boolean;
  allow_training: boolean;
  allow_public_full_text: boolean;
  notes: string;
  reviewed_by: string;
  reviewed_at: string | null;
  review_on: string | null;
  created_at: string;
};

export type SourcePerformance = {
  total_captured: number;
  captured_today: number;
  published: number;
  last_success_at: string | null;
  daily: {
    date: string;
    captured: number;
    published: number;
  }[];
};

export type ReasoningEffort = "low" | "medium" | "high";

export type LlmProvider = {
  id: string;
  name: string;
  enabled: boolean;
  available: boolean;
  status: string;
  status_detail: string;
  checked_at: string;
  installed: boolean;
  authenticated: boolean;
  ready: boolean;
  version: string;
  auth_method: string;
};

export type LlmModel = {
  id: string;
  name: string;
  compatible_tasks: string[];
  reasoning_efforts: ReasoningEffort[];
};

export type LlmProfile = {
  task: string;
  name: string;
  purpose: string;
  provider_id: string;
  model: string;
  timeout_seconds: number;
  enabled: boolean;
  reasoning_effort: ReasoningEffort | "";
};

export type CodexStatus = {
  installed: boolean;
  authenticated: boolean;
  ready: boolean;
  path: string;
  version: string;
  auth_method: string;
  detail: string;
  checked_at: string;
  models: string[];
};

export type AnalysisBackfillScope = "date_range" | "catalog" | "article";
export type AnalysisBackfillWorkflow = "single_pass" | "full_pipeline";
export type AnalysisBackfillProvider = "codex_cli";

export type AnalysisBackfillRequest = {
  scope: AnalysisBackfillScope;
  workflow: AnalysisBackfillWorkflow;
  provider: AnalysisBackfillProvider;
  model: string;
  from?: string;
  to?: string;
  article_id?: string;
  confirmation?: string;
};

export type AnalysisBackfillRun = {
  id: string;
  scope: AnalysisBackfillScope;
  workflow: AnalysisBackfillWorkflow;
  provider: string; // Historical runs can retain legacy providers.
  model: string;
  from: string | null;
  to: string | null;
  article_id: string | null;
  status: "queued" | "running" | "paused" | "completed" | "partially_completed" | "failed" | "cancelled";
  total_articles: number;
  pending_articles: number;
  queued_articles: number;
  running_articles: number;
  succeeded_articles: number;
  failed_articles: number;
  cancelled_articles: number;
  created_by: string;
  error_detail: string | null;
  created_at: string;
  started_at: string | null;
  finished_at: string | null;
  latest_item_updated_at: string | null;
};

export type Overview = {
  published: number;
  held: number;
  quarantined: number;
  complaints: number;
  sick_feeds: number;
  stale_feeds: number;
  sources: number;
};

export type OverviewTrendPoint = {
  date: string;
  published: number;
  received: number;
};

export type KnowledgeArticle = {
  id: string;
  headline: string;
  source_id: string;
  source: string;
  source_icon: string;
  original_url: string;
  published_at: string;
  political?: {
    model: string;
    economic_frame: number;
    left_probability: number;
    center_probability: number;
    right_probability: number;
    axis_version: string;
    confidence: number;
    relevant: boolean;
    label: 'left' | 'center_left' | 'neutral' | 'center_right' | 'right' | 'unclear';
    rationale: string;
    evidence: string[];
    provider_id: string;
    provider_model: string;
    mentions: {
      party_slug: string;
      stance: number;
      confidence: number;
      terms: string[];
    }[];
  };
};

export type KnowledgeEvent = {
  id: string;
  title: string;
  category: string;
  category_name_si: string;
  confidence: number;
  is_breaking: boolean;
  locked: boolean;
  algorithm_version: string;
  first_seen_at: string;
  last_update_at: string;
  articles: KnowledgeArticle[];
};

export type KnowledgeGraph = {
  generated_at: string;
  days: number;
  summary: {
    articles: number;
    events: number;
    multi_source_events: number;
    sources: number;
  };
  categories: {
    slug: string;
    name_si: string;
    name_en: string;
    articles: number;
    events: number;
  }[];
  events: KnowledgeEvent[];
  political: {
    axis: string;
    model: string;
    minimum_sample: number;
    parties: {
      slug: string;
      short_name: string;
      name_en: string;
      name_si: string;
      economic_position: number;
      confidence: number;
      rationale: string;
      evidence_urls: string[];
    }[];
    sources: {
      source_id: string;
      source: string;
      source_icon: string;
      economic_frame: number;
      confidence: number;
      relevant_events: number;
      scored_articles: number;
      qualified: boolean;
    }[];
  };
};

export type QueueItem = {
  id: string;
  headline: string;
  public_status: string;
  source: string;
  received_at: string;
  confidence: number | null;
  model: string | null;
  category: string | null;
};

export type AdminArticleListItem = {
  id: string;
  headline: string;
  snippet?: string;
  original_url?: string;
  public_status: string;
  source: string;
  source_icon: string;
  category: string | null;
  received_at: string;
  published_at: string;
  pipeline_status: string | null;
  current_step: string | null;
  pipeline_finished_at: string | null;
};

export type PipelineLog = {
  id: number;
  level: "info" | "warning" | "error";
  event: string;
  message: string;
  details: Record<string, unknown>;
  created_at: string;
};

export type PipelineStep = {
  id: string;
  name: string;
  position: number;
  status: "queued" | "running" | "succeeded" | "failed" | "skipped";
  attempt: number;
  max_attempts: number;
  started_at: string | null;
  finished_at: string | null;
  duration_ms: number | null;
  error_detail: string | null;
  output: Record<string, unknown>;
  logs: PipelineLog[];
};

export type LLMCall = {
  id: number;
  pipeline_run_id: string | null;
  pipeline_step_id: string | null;
  task: string;
  provider_id: string;
  model: string;
  input_tokens: number | null;
  output_tokens: number | null;
  latency_ms: number | null;
  first_token_ms: number | null;
  outcome: string;
  streamed: boolean;
  response_text: string;
  finish_reason: string;
  error_detail: string | null;
  created_at: string;
  completed_at: string | null;
};

export type PipelineRun = {
  id: string;
  status: "queued" | "running" | "succeeded" | "failed";
  trigger: string;
  provider_id: string;
  model: string;
  current_step: string | null;
  attempt: number;
  last_error: string | null;
  created_at: string;
  started_at: string | null;
  finished_at: string | null;
  steps: PipelineStep[];
};

export type QueueJobStatus =
  | "queued"
  | "processing"
  | "completed"
  | "partially_completed"
  | "failed";

export type QueueJob = {
  id: string;
  job_id: number | null;
  run_id: string | null;
  article_id: string | null;
  title: string;
  source: string | null;
  source_icon: string;
  kind: string;
  queue: string;
  status: QueueJobStatus;
  river_state: string;
  trigger: string | null;
  attempt: number;
  max_attempts: number;
  current_step: string | null;
  created_at: string;
  started_at: string | null;
  finished_at: string | null;
  duration_ms: number | null;
  error_detail: string | null;
  error_trace: string | null;
  steps: PipelineStep[];
};

export type QueueMonitor = PageResponse<QueueJob> & {
  summary: {
    total: number;
    queued: number;
    processing: number;
    completed: number;
    partially_completed: number;
    failed: number;
  };
};

export type QueueCleanupScope = "failed" | "all";

export type QueueCleanupPreview = {
  scope: QueueCleanupScope;
  confirmation: string;
  river_jobs: number;
  pipeline_runs: number;
  pipeline_steps: number;
  pipeline_logs: number;
  total_records: number;
  active_jobs_protected: boolean;
};

export type QueueJobArtifact = {
  id: string;
  role: "input" | "output";
  kind: string;
  title: string;
  description: string;
  data: Record<string, unknown>;
};

export type QueueJobArtifacts = {
  job_id: string;
  inputs: QueueJobArtifact[];
  outputs: QueueJobArtifact[];
};

export type CronJobHealth =
  | "healthy"
  | "running"
  | "degraded"
  | "failed"
  | "overdue"
  | "unknown";

export type CronJobStatistic = {
  kind: string;
  name: string;
  description: string;
  queue: string;
  interval_seconds: number;
  run_on_start: boolean;
  health: CronJobHealth;
  state: string;
  currently_running: number;
  last_job_id: number | null;
  last_run_at: string | null;
  last_finished_at: string | null;
  next_run_at: string | null;
  last_duration_ms: number | null;
  average_duration_ms: number | null;
  runs_24h: number;
  successful_runs_24h: number;
  failed_runs_24h: number;
  success_rate_24h: number | null;
  attempt: number;
  max_attempts: number;
  worker_id: string | null;
  last_error: string | null;
};

export type CronMonitor = {
  items: CronJobStatistic[];
  summary: {
    total: number;
    running: number;
    healthy: number;
    attention: number;
  };
  worker: {
    status: "online" | "stale" | "offline";
    leader_id: string | null;
    elected_at: string | null;
    lease_expires_at: string | null;
    max_concurrency: number;
    queues: {
      name: string;
      max_workers: number;
      paused: boolean;
    }[];
  };
  checked_at: string;
};

export type QueueMonitorSnapshot = {
  queue: QueueMonitor;
  cron: CronMonitor;
};

export type AdminArticleDetail = {
  id: string;
  headline: string;
  description: string;
  public_status: string;
  source_id: string;
  source: string;
  source_icon: string;
  original_url: string;
  canonical_url: string;
  author: string;
  category: string | null;
  category_name: string | null;
  publisher_category: string;
  classification_model: string | null;
  classification_confidence: number | null;
  endpoint_id: string;
  endpoint_url: string;
  rights_mode: string;
  published_at: string;
  received_at: string;
  event: {
    id: string;
    title: string;
    confidence: number;
    algorithm_version: string;
  } | null;
  political: KnowledgeArticle["political"] | null;
  pipeline_runs: PipelineRun[];
  llm_calls: LLMCall[];
  content: {
    body_text: string;
    acquisition_method: "feed_content" | "api_content" | "html_static";
    source_url: string;
    extractor_version: string;
    fetched_at: string;
    retention_until: string | null;
    characters: number;
  } | null;
  analysis_document: {
    original_text: string;
    cleaned_text: string;
    summary_text: string;
    summary_points: string[];
    cleaner_version: string;
    cleaner_provider: string;
    cleaner_model: string;
    summary_provider: string;
    summary_model: string;
    cleaned_at: string;
    summarized_at: string | null;
  } | null;
  event_analysis: EventNarrativeAnalysis | null;
};

export type AdminComplaint = {
  id: string;
  entity_type: string;
  entity_id: string;
  reason: string;
  contact: string | null;
  status: string;
};

export type PaginationMeta = {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
};

export type PageResponse<T> = {
  items: T[];
  pagination: PaginationMeta;
};

export type WatchTowerThread = {
  id: string;
  title: string;
  message_count: number;
  last_message: string;
  created_at: string;
  updated_at: string;
};

export type WatchTowerCitation = {
  number: number;
  article_id: string;
  headline: string;
  source: string;
  category: string;
  published_at: string;
  original_url: string;
};

export type WatchTowerMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  citations: WatchTowerCitation[];
  suggestions: string[];
  provider?: string;
  model?: string;
  search?: {
    label: string;
    from: string;
    to: string;
    article_count: number;
  };
  created_at: string;
};

export type WatchTowerConversation = {
  thread: WatchTowerThread;
  messages: WatchTowerMessage[];
};

export type WatchTowerExchange = {
  thread: WatchTowerThread;
  user: WatchTowerMessage;
  assistant: WatchTowerMessage;
};

export type WatchTowerSettings = {
  response_language: "si" | "en";
  updated_at: string;
};

export type LogRetentionSummaryKey =
  | "pipeline_runs" | "pipeline_logs" | "model_calls" | "ingestion_runs" | "crawl_attempts" | "log_files";

export type LogRetentionSettings = {
  retention_days: number;
  last_run_at: string | null;
  last_run_summary: Partial<Record<LogRetentionSummaryKey, number>>;
  last_run_error: string | null;
  updated_at: string;
};

export type NewsletterSubscriberStatus = "active" | "paused" | "unsubscribed";

export type NewsletterSubscriber = {
  id: string;
  email: string;
  name: string;
  status: NewsletterSubscriberStatus;
  consent_source: string;
  consented_at: string;
  created_at: string;
  updated_at: string;
};

export type NewsletterSubscriberInput = {
  email: string;
  name: string;
  status: NewsletterSubscriberStatus;
  consent_confirmed?: boolean;
};

export type NewsletterSettings = {
  enabled: boolean;
  timezone: string;
  send_hour: number;
  max_stories: number;
  lead_story_count: number;
  subject_template: string;
  preheader_template: string;
  intro_text: string;
  footer_text: string;
  updated_at: string;
};

export type NewsletterSubscriberList = {
  items: NewsletterSubscriber[];
  summary: {
    total: number;
    active: number;
    paused: number;
    unsubscribed: number;
  };
  settings: NewsletterSettings;
};

export type NewsletterTestInput = {
  mode: "preview" | "send";
  window_mode: "latest_24h" | "scheduled";
  recipient_email?: string;
  recipient_name?: string;
};

export type NewsletterTestRun = {
  id: string;
  mode: "preview" | "send";
  window_mode: "latest_24h" | "scheduled";
  status: "succeeded" | "failed";
  recipient_email?: string;
  provider_id: string;
  model: string;
  subject: string;
  preheader: string;
  story_count: number;
  article_count: number;
  event_count: number;
  source_count: number;
  duration_ms: number;
  provider_message_id?: string;
  error_detail?: string;
  created_at: string;
  completed_at: string;
};

export type NewsletterTestResult = NewsletterTestRun & {
  html: string;
  text: string;
};

export type AgentWorkflow = {
  task: string;
  name: string;
  purpose: string;
  category: string;
  custom_instructions: string;
  personality: string;
  learning_notes: string;
  tone: string;
  response_language: string;
  audience: string;
  enabled: boolean;
  revision: number;
  provider_id: string;
  model: string;
  reasoning_effort: ReasoningEffort | "";
  updated_at: string;
};

export type AgentWorkflowInput = Pick<AgentWorkflow,
  "custom_instructions" | "personality" | "tone" | "response_language" | "audience" | "enabled" | "provider_id" | "model" | "reasoning_effort"
>;

export type AgentFeedback = {
  id: string;
  workflow_task: string;
  workflow_name: string;
  rating: "helpful" | "needs_improvement";
  category: "accuracy" | "tone" | "relevance" | "formatting" | "safety" | "other";
  message: string;
  status: "new" | "reviewed" | "applied" | "dismissed";
  created_at: string;
  reviewed_at: string | null;
};

export type AdminTableQuery = {
  page?: number;
  per_page?: number;
  search?: string;
  [filter: string]: string | number | undefined;
};

function withQuery(path: string, params: object = {}) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") search.set(key, String(value));
  }
  const suffix = search.toString();
  return suffix ? `${path}?${suffix}` : path;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const method = init?.method?.toUpperCase() ?? "GET";
  const response = await fetch(path, {
    credentials: "include",
    ...init,
    headers: {
      ...(typeof init?.body === "string" ? { "Content-Type": "application/json" } : {}),
      ...(method !== "GET" && method !== "HEAD" ? { "X-SNAP-CSRF": "1" } : {}),
      ...init?.headers
    }
  });
  if (!response.ok) {
    throw new Error(`${path} failed: ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export type SocialPostStatus = "pending_review" | "approved" | "publishing" | "published" | "needs_attention" | "failed" | "skipped" | "rejected";
export type SocialSettings = {
  enabled: boolean; auto_publish: boolean; max_posts_per_hour: number; max_posts_per_day: number;
  min_gap_minutes: number; quiet_hours_start: number; quiet_hours_end: number; min_score: number;
  lookback_hours: number; candidates_per_run: number; include_single_articles: boolean; timezone: string; updated_at: string;
  realtime_enabled: boolean; min_review_score: number;
};
export type SocialChannelKey = "facebook";
export type SocialTrigger = "scheduled" | "realtime" | "manual";
export type SocialReviewStatus = "not_reviewed" | "passed" | "failed" | "unavailable";
export type SocialReviewIssue = { severity: "blocker" | "major" | "minor"; field: string; message: string };
export type SocialAccount = {
  id: string; page_id: string; page_name: string; picture: string; status: string; connected_at: string;
  last_verified_at: string | null; token_expires_at: string | null; last_error: string | null;
};
export type SocialPost = {
  id: string; account_id: string | null; subject_kind: string; event_id: string | null; article_id: string | null;
  subject_title: string; category: string; status: SocialPostStatus; score: number; selection_reason: string;
  kind: string; headline_si: string; summary_si: string; caption: string; hashtags: string[]; link_url: string;
  source_names: string[]; source_count: number; spectrum: { left: number; center: number; right: number } | null;
  image_key: string | null; image_url: string; image_rendered_at: string | null; scheduled_for: string | null;
  approved_by: string | null; approved_at: string | null; rejected_by: string | null; rejected_at: string | null;
  published_at: string | null; fb_post_id: string | null; fb_permalink: string | null; attempts: number;
  last_error: string | null; llm_call_id: number | null; evaluated_source_count: number;
  created_by: string; created_at: string; updated_at: string;
  channel: SocialChannelKey; trigger: SocialTrigger; review_status: SocialReviewStatus; review_score: number | null;
  review_summary: string; review_issues: SocialReviewIssue[]; reviewed_at: string | null;
  hold_reason: string | null; review_revisions: number; plan?: SocialPublishPlan;
};
/** Whether and when a post goes out; `at` is an estimate when `estimated` is true. */
export type SocialPublishPlan = {
  state: "published" | "publishing" | "scheduled" | "paused" | "in_review" | "needs_editor" | "not_publishing";
  at: string | null; estimated: boolean; note: string;
};
export type SocialChannel = { key: string; name: string; available: boolean; status: string };
export type SocialContentList = { items: SocialPost[]; from: string; to: string };
export type SocialStatus = {
  configured: boolean; connected: boolean; account: SocialAccount | null; settings: SocialSettings;
  counts: Partial<Record<SocialPostStatus, number>>; next_publish_eligible_at: string | null;
};
export type SocialPostList = { items: SocialPost[]; next_cursor: string };
export type SocialEditInput = Pick<SocialPost, "headline_si" | "summary_si" | "caption" | "hashtags">;
export type SocialCreateInput = { event_id: string; article_id?: never } | { article_id: string; event_id?: never };
export type SocialPendingPage = { id: string; name: string; picture: string };

export function createClient(baseUrl = "") {
  const url = (path: string) => `${baseUrl}${path}`;
  return {
    listNews: (params: Record<string, string> = {}) => {
      const search = new URLSearchParams();
      for (const [key, value] of Object.entries(params)) {
        if (value) search.set(key, value);
      }
      const suffix = search.toString() ? `?${search}` : "";
      return request<CursorPage<PublicArticle>>(url(`/api/v1/news${suffix}`));
    },
    getNews: (id: string) => request<PublicArticle>(url(`/api/v1/news/${id}`)),
    categories: () =>
      request<{ items: PublicCategory[] }>(url("/api/v1/categories")),
    sources: () => request<{ items: PublicSource[] }>(url("/api/v1/sources")),
    getSource: (id: string) =>
      request<PublicSource>(url(`/api/v1/sources/${id}`)),
    listEvents: () => request<{ items: PublicEvent[] }>(url("/api/v1/events")),
    getEvent: (id: string) => request<PublicEvent>(url(`/api/v1/events/${id}`)),
    publicKnowledgeGraph: (params: {
      days?: 1 | 7 | 30;
      category?: string;
      source?: string;
      from?: string;
      to?: string;
    } = { days: 1 }) =>
      request<PublicKnowledgeGraph>(url(withQuery("/api/v1/knowledge-graph", params))),
    breaking: () => request<{ items: PublicEvent[] }>(url("/api/v1/breaking")),
    frontPage: () => request<FrontPage>(url("/api/v1/front-page")),
    editorials: () => request<{ items: EditorialSummary[] }>(url("/api/v1/editorials")),
    editorial: (id: string) => request<Editorial>(url(`/api/v1/editorials/${encodeURIComponent(id)}`)),
    /** Up to six recent stories to read next, same section first. */
    relatedStories: (kind: "event" | "article", id: string) =>
      request<{ items: FrontPageStory[] }>(url(`/api/v1/related/${kind}/${encodeURIComponent(id)}`)),
    /** The latest desk report and illustration for a story, if one has been written. */
    storyReport: (kind: "event" | "article", id: string) =>
      request<{ report: StoryReport; image: StoryImage | null }>(url(`/api/v1/reports/${kind}/${encodeURIComponent(id)}`)),
    adminFrontPage: () => request<AdminFrontPage>(url("/api/admin/front-page")),
    updateFrontPage: (body: { placements: FrontPagePlacementInput[] }) =>
      request<AdminFrontPage>(url("/api/admin/front-page"), { method: "PUT", body: JSON.stringify(body) }),
    rebuildFrontPage: () =>
      request<{ ok: boolean }>(url("/api/admin/front-page/rebuild"), { method: "POST", body: JSON.stringify({}) }),
    setFrontPageSource: (id: string, body: { front_page: boolean }) =>
      request<FrontPageSource>(url(`/api/admin/front-page/sources/${encodeURIComponent(id)}`), { method: "PATCH", body: JSON.stringify(body) }),
    adminEditorials: () => request<{ items: AdminEditorial[] }>(url("/api/admin/editorials")),
    createEditorial: (body: EditorialInput) =>
      request<AdminEditorial>(url("/api/admin/editorials"), { method: "POST", body: JSON.stringify(body) }),
    updateEditorial: (id: string, body: EditorialInput) =>
      request<AdminEditorial>(url(`/api/admin/editorials/${encodeURIComponent(id)}`), { method: "PATCH", body: JSON.stringify(body) }),
    setEditorialStatus: (id: string, action: "publish" | "unpublish" | "archive") =>
      request<AdminEditorial>(url(`/api/admin/editorials/${encodeURIComponent(id)}/${action}`), { method: "POST", body: JSON.stringify({}) }),
    deleteEditorial: (id: string) =>
      request<{ ok: boolean }>(url(`/api/admin/editorials/${encodeURIComponent(id)}`), { method: "DELETE" }),
    brief: () =>
      request<{
        date?: string;
        title_si: string;
        body_si: string;
        model: string;
      }>(url("/api/v1/brief")),
    complain: (body: {
      entity_type: string;
      entity_id: string;
      reason: string;
      contact?: string;
    }) =>
      request<{ ok: boolean }>(url("/api/v1/complaints"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    login: (email: string, password: string) =>
      request<{
        ok: boolean;
        user: { id: string; email: string; name: string; role: string };
      }>(url("/api/admin/login"), {
        method: "POST",
        body: JSON.stringify({ email, password })
      }),
    logout: () =>
      request<{ ok: boolean }>(url("/api/admin/logout"), { method: "POST" }),
    me: () =>
      request<{ email: string; name: string; role: string }>(
        url("/api/admin/me")
      ),
    overview: () => request<Overview>(url("/api/admin/overview")),
    overviewTrends: (days: 7 | 30 | 90 = 90) =>
      request<{ items: OverviewTrendPoint[] }>(
        url(`/api/admin/overview/trends?days=${days}`)
      ),
    knowledgeGraph: (params: {
      days?: 1 | 7 | 30;
      category?: string;
      from?: string;
      to?: string;
    } = { days: 1 }) =>
      request<KnowledgeGraph>(
        url(withQuery("/api/admin/knowledge-graph", params))
      ),
    queue: (params: AdminTableQuery = {}) =>
      request<PageResponse<QueueItem>>(
        url(withQuery("/api/admin/queue", params))
      ),
    queueJobs: (params: AdminTableQuery = {}) =>
      request<QueueMonitor>(url(withQuery("/api/admin/jobs", params))),
    queueCleanupPreview: (scope: QueueCleanupScope) =>
      request<QueueCleanupPreview>(url(withQuery("/api/admin/jobs/cleanup", { scope }))),
    cleanupQueue: (scope: QueueCleanupScope, confirmation: string) =>
      request<QueueCleanupPreview>(url("/api/admin/jobs/cleanup"), {
        method: "POST",
        body: JSON.stringify({ scope, confirmation })
      }),
    queueJobArtifacts: (id: string) =>
      request<QueueJobArtifacts>(url(`/api/admin/jobs/${encodeURIComponent(id)}`)),
    queueMonitorStreamURL: (params: AdminTableQuery = {}) =>
      url(withQuery("/api/admin/jobs/stream", params)),
    cronJobs: () => request<CronMonitor>(url("/api/admin/cron-jobs")),
    adminArticles: (params: AdminTableQuery = {}) =>
      request<PageResponse<AdminArticleListItem>>(
        url(withQuery("/api/admin/articles", params))
      ),
    adminArticle: (id: string) =>
      request<AdminArticleDetail>(url(`/api/admin/articles/${encodeURIComponent(id)}`)),
    articleLLMCalls: (id: string, params: AdminTableQuery = {}) =>
      request<PageResponse<LLMCall>>(
        url(withQuery(`/api/admin/articles/${encodeURIComponent(id)}/llm-calls`, params))
      ),
    deleteArticleLLMCall: (articleId: string, callId: number) =>
      request<{ ok: boolean }>(
        url(`/api/admin/articles/${encodeURIComponent(articleId)}/llm-calls/${callId}`),
        { method: "DELETE" }
      ),
    runArticlePipeline: (id: string, step = "") =>
      request<{ ok: boolean }>(url(`/api/admin/articles/${id}/pipeline/run`), {
        method: "POST",
        body: JSON.stringify({ step })
      }),
    quarantine: () =>
      request<{
        items: {
          id: string;
          endpoint_id: string;
          reason: string;
          sample: string | null;
        }[];
      }>(url("/api/admin/quarantine")),
    setArticleStatus: (id: string, status: string, reason: string) =>
      request<{ ok: boolean }>(url(`/api/admin/articles/${id}/status`), {
        method: "POST",
        body: JSON.stringify({ status, reason })
      }),
    reviewArticle: (
      id: string,
      review: { status?: string; category?: string; reason?: string }
    ) =>
      request<{ ok: boolean }>(url(`/api/admin/articles/${id}/review`), {
        method: "POST",
        body: JSON.stringify(review)
      }),
    deleteArticle: (id: string, reason = "Deleted from article management") =>
      request<{ ok: boolean }>(url(`/api/admin/articles/${id}`), {
        method: "DELETE",
        body: JSON.stringify({ reason })
      }),
    setArticleCategory: (id: string, slug: string) =>
      request<{ ok: boolean }>(url(`/api/admin/articles/${id}/category`), {
        method: "POST",
        body: JSON.stringify({ slug })
      }),
    setArticleNote: (id: string, note: string) =>
      request<{ ok: boolean }>(url(`/api/admin/articles/${id}/note`), {
        method: "POST",
        body: JSON.stringify({ note })
      }),
    complaints: (params: AdminTableQuery = {}) =>
      request<PageResponse<AdminComplaint>>(
        url(withQuery("/api/admin/complaints", params))
      ),
    resolveComplaint: (id: string, status: string, resolution: string) =>
      request<{ ok: boolean }>(url(`/api/admin/complaints/${id}`), {
        method: "POST",
        body: JSON.stringify({ status, resolution })
      }),
    adminSources: (params: AdminTableQuery = {}) =>
      request<PageResponse<AdminSource>>(
        url(withQuery("/api/admin/sources", params))
      ),
    adminSource: (id: string) =>
      request<AdminSource>(url(`/api/admin/sources/${id}`)),
    sourcePerformance: (id: string, days: 7 | 30 | 90 = 30) =>
      request<SourcePerformance>(
        url(`/api/admin/sources/${id}/performance?days=${days}`)
      ),
    createSource: (body: AdminSourceInput) =>
      request<AdminSource>(url("/api/admin/sources"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    updateSource: (id: string, body: AdminSourceInput) =>
      request<{ ok: boolean }>(url(`/api/admin/sources/${id}`), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    uploadSourceLogo: (id: string, file: File) => {
      const body = new FormData();
      body.set("file", file);
      return request<{ icon_url: string }>(url(`/api/admin/sources/${id}/logo`), {
        method: "POST",
        body
      });
    },
    removeSourceLogo: (id: string) =>
      request<{ icon_url: string }>(url(`/api/admin/sources/${id}/logo`), {
        method: "DELETE"
      }),
    archiveSource: (id: string) =>
      request<{ ok: boolean }>(url(`/api/admin/sources/${id}/archive`), {
        method: "POST"
      }),
    setSourceActive: (id: string, active: boolean) =>
      request<{ ok: boolean }>(url(`/api/admin/sources/${id}/active`), {
        method: "POST",
        body: JSON.stringify({ active })
      }),
    adminEndpoints: (sourceId: string, params: AdminTableQuery = {}) =>
      request<PageResponse<AdminEndpoint>>(
        url(withQuery(`/api/admin/sources/${sourceId}/endpoints`, params))
      ),
    createEndpoint: (
      sourceId: string,
      body: { endpoint_type: string; url: string }
    ) =>
      request<AdminEndpoint>(url(`/api/admin/sources/${sourceId}/endpoints`), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    updateEndpoint: (
      endpointId: string,
      body: { polling_interval_seconds: number; verified_official: boolean }
    ) =>
      request<{ ok: boolean }>(url(`/api/admin/endpoints/${endpointId}`), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    adminRights: (sourceId: string, params: AdminTableQuery = {}) =>
      request<PageResponse<AdminRights>>(
        url(withQuery(`/api/admin/sources/${sourceId}/rights`, params))
      ),
    createRights: (
      sourceId: string,
      body: { endpoint_id: string; mode: string; attribution: string }
    ) =>
      request<AdminRights>(url(`/api/admin/sources/${sourceId}/rights`), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    sourceCollections: (sourceId: string) =>
      request<{ items: AdminCollectionProfile[] }>(
        url(`/api/admin/sources/${sourceId}/collection`)
      ),
    saveSourceCollection: (
      sourceId: string,
      body: Omit<AdminCollectionProfile, "id" | "source_id" | "version" | "created_by" | "activated_at" | "created_at">
    ) =>
      request<AdminCollectionProfile>(url(`/api/admin/sources/${sourceId}/collection`), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    sourceCompliance: (sourceId: string) =>
      request<AdminComplianceReview>(url(`/api/admin/sources/${sourceId}/compliance`)),
    saveSourceCompliance: (
      sourceId: string,
      body: Omit<AdminComplianceReview, "id" | "source_id" | "version" | "reviewed_by" | "reviewed_at" | "created_at">
    ) =>
      request<AdminComplianceReview>(url(`/api/admin/sources/${sourceId}/compliance`), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    pauseEndpoint: (endpointId: string, paused: boolean) =>
      request<{ ok: boolean }>(
        url(`/api/admin/endpoints/${endpointId}/pause`),
        { method: "POST", body: JSON.stringify({ paused }) }
      ),
    testEndpoint: (endpointId: string) =>
      request<{
        status: number;
        contentType: string;
        parseable?: boolean;
        latest?: string;
      }>(url(`/api/admin/endpoints/${endpointId}/test`), { method: "POST" }),
    runEndpoint: (endpointId: string) =>
      request<{ ok: boolean }>(url(`/api/admin/endpoints/${endpointId}/run`), {
        method: "POST"
      }),
    llmProvider: () =>
      request<LlmProvider>(url("/api/admin/llm/providers")),
    llmModels: () =>
      request<{ items: LlmModel[] }>(url("/api/admin/llm/models")),
    llmProfiles: () =>
      request<{ items: LlmProfile[] }>(url("/api/admin/llm/profiles")),
    updateLlmProfile: (body: { task: string; provider_id: "codex_cli"; model: string; reasoning_effort?: ReasoningEffort | "" }) =>
      request<{ ok: boolean }>(url("/api/admin/llm/profiles"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    codexStatus: () =>
      request<CodexStatus>(url("/api/admin/settings/codex")),
    analysisBackfillPreview: (body: AnalysisBackfillRequest) =>
      request<{ articles: number }>(url(withQuery("/api/admin/settings/analysis-backfills/preview", body))),
    analysisBackfills: () =>
      request<{ items: AnalysisBackfillRun[] }>(url("/api/admin/settings/analysis-backfills")),
    analysisBackfillStreamURL: () =>
      url("/api/admin/settings/analysis-backfills/stream"),
    analysisBackfill: (id: string) =>
      request<AnalysisBackfillRun>(url(`/api/admin/settings/analysis-backfills/${encodeURIComponent(id)}`)),
    pauseAnalysisBackfill: (id: string) =>
      request<AnalysisBackfillRun>(url(`/api/admin/settings/analysis-backfills/${encodeURIComponent(id)}/pause`), {
        method: "POST"
      }),
    resumeAnalysisBackfill: (id: string) =>
      request<AnalysisBackfillRun>(url(`/api/admin/settings/analysis-backfills/${encodeURIComponent(id)}/resume`), {
        method: "POST"
      }),
    cancelAnalysisBackfill: (id: string) =>
      request<AnalysisBackfillRun>(url(`/api/admin/settings/analysis-backfills/${encodeURIComponent(id)}/cancel`), {
        method: "POST"
      }),
    deleteAnalysisBackfill: (id: string) =>
      request<{ ok: boolean }>(url(`/api/admin/settings/analysis-backfills/${encodeURIComponent(id)}`), {
        method: "DELETE"
      }),
    createAnalysisBackfill: (body: AnalysisBackfillRequest) =>
      request<AnalysisBackfillRun>(url("/api/admin/settings/analysis-backfills"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    watchTowerThreads: () =>
      request<{ items: WatchTowerThread[] }>(url("/api/admin/watch-tower/threads")),
    createWatchTowerThread: (title: string) =>
      request<WatchTowerThread>(url("/api/admin/watch-tower/threads"), {
        method: "POST",
        body: JSON.stringify({ title })
      }),
    watchTowerConversation: (id: string) =>
      request<WatchTowerConversation>(
        url(`/api/admin/watch-tower/threads/${encodeURIComponent(id)}`)
      ),
    askWatchTower: (id: string, content: string) =>
      request<WatchTowerExchange>(
        url(`/api/admin/watch-tower/threads/${encodeURIComponent(id)}/messages`),
        { method: "POST", body: JSON.stringify({ content }) }
      ),
    deleteWatchTowerThread: (id: string) =>
      request<{ ok: boolean }>(
        url(`/api/admin/watch-tower/threads/${encodeURIComponent(id)}`),
        { method: "DELETE" }
      ),
    logRetentionSettings: () =>
      request<LogRetentionSettings>(url("/api/admin/settings/log-retention")),
    updateLogRetentionSettings: (body: { retention_days: number }) =>
      request<LogRetentionSettings>(url("/api/admin/settings/log-retention"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    runLogCleanup: () =>
      request<{ ok: boolean }>(url("/api/admin/settings/log-retention/run"), {
        method: "POST",
        body: JSON.stringify({})
      }),
    watchTowerSettings: () =>
      request<WatchTowerSettings>(url("/api/admin/settings/watch-tower")),
    updateWatchTowerSettings: (body: { response_language: "si" | "en" }) =>
      request<WatchTowerSettings>(url("/api/admin/settings/watch-tower"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    updateWatchTowerModel: (body: { provider_id: "codex_cli"; model: string; reasoning_effort?: ReasoningEffort | "" }) =>
      request<{ ok: boolean }>(url("/api/admin/settings/watch-tower/model"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    newsletterSubscribers: () =>
      request<NewsletterSubscriberList>(url("/api/admin/newsletter/subscribers")),
    createNewsletterSubscriber: (body: NewsletterSubscriberInput) =>
      request<NewsletterSubscriber>(url("/api/admin/newsletter/subscribers"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    updateNewsletterSubscriber: (id: string, body: NewsletterSubscriberInput) =>
      request<NewsletterSubscriber>(
        url(`/api/admin/newsletter/subscribers/${encodeURIComponent(id)}`),
        { method: "POST", body: JSON.stringify(body) }
      ),
    deleteNewsletterSubscriber: (id: string) =>
      request<{ ok: boolean }>(
        url(`/api/admin/newsletter/subscribers/${encodeURIComponent(id)}`),
        { method: "DELETE" }
      ),
    newsletterSettings: () =>
      request<NewsletterSettings>(url("/api/admin/newsletter/settings")),
    updateNewsletterSettings: (body: NewsletterSettings) =>
      request<NewsletterSettings>(url("/api/admin/newsletter/settings"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    newsletterTests: () =>
      request<{ items: NewsletterTestRun[] }>(url("/api/admin/newsletter/tests")),
    runNewsletterTest: (body: NewsletterTestInput) =>
      request<NewsletterTestResult>(url("/api/admin/newsletter/tests"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    socialStatus: () => request<SocialStatus>(url("/api/admin/social/status")),
    socialSettings: () => request<SocialSettings>(url("/api/admin/social/settings")),
    updateSocialSettings: (body: SocialSettings) => request<SocialSettings>(url("/api/admin/social/settings"), { method: "POST", body: JSON.stringify(body) }),
    connectFacebook: () => request<{ authorize_url: string }>(url("/api/admin/social/facebook/connect"), { method: "POST", body: "{}" }),
    pendingFacebookPages: (state_id: string) => request<{ items: SocialPendingPage[] }>(url(withQuery("/api/admin/social/facebook/pending-pages", { state_id }))),
    selectFacebookPage: (body: { state_id: string; page_id: string }) => request<{ ok: boolean }>(url("/api/admin/social/facebook/select"), { method: "POST", body: JSON.stringify(body) }),
    disconnectFacebook: () => request<{ ok: boolean }>(url("/api/admin/social/facebook/disconnect"), { method: "POST", body: "{}" }),
    socialPosts: (params: { status?: SocialPostStatus; limit?: number; cursor?: string } = {}) => request<SocialPostList>(url(withQuery("/api/admin/social/posts", params))),
    socialPost: (id: string) => request<SocialPost>(url(`/api/admin/social/posts/${encodeURIComponent(id)}`)),
    createSocialPost: (body: SocialCreateInput) => request<SocialPost>(url("/api/admin/social/posts"), { method: "POST", body: JSON.stringify(body) }),
    editSocialPost: (id: string, body: Partial<SocialEditInput>) => request<SocialPost>(url(`/api/admin/social/posts/${encodeURIComponent(id)}`), { method: "PATCH", body: JSON.stringify(body) }),
    approveSocialPost: (id: string, body: { scheduled_for?: string } = {}) => request<SocialPost>(url(`/api/admin/social/posts/${encodeURIComponent(id)}/approve`), { method: "POST", body: JSON.stringify(body) }),
    rejectSocialPost: (id: string) => request<SocialPost>(url(`/api/admin/social/posts/${encodeURIComponent(id)}/reject`), { method: "POST", body: "{}" }),
    retrySocialPost: (id: string, confirm_not_published = false) => request<SocialPost>(url(`/api/admin/social/posts/${encodeURIComponent(id)}/retry`), { method: "POST", body: JSON.stringify({ confirm_not_published }) }),
    renderSocialPost: (id: string) => request<SocialPost>(url(`/api/admin/social/posts/${encodeURIComponent(id)}/render`), { method: "POST", body: "{}" }),
    reviewSocialPost: (id: string) => request<SocialPost>(url(`/api/admin/social/posts/${encodeURIComponent(id)}/review`), { method: "POST", body: "{}" }),
    socialChannels: () => request<{ items: SocialChannel[] }>(url("/api/admin/social/channels")),
    socialContent: (params: { from: string; to: string; channel?: string; status?: SocialPostStatus }) => request<SocialContentList>(url(withQuery("/api/admin/social/content", params))),
    runSocialSelection: () => request<{ ok: boolean }>(url("/api/admin/social/run-selection"), { method: "POST", body: "{}" }),
    agentWorkflows: () =>
      request<{ items: AgentWorkflow[] }>(url("/api/admin/workflows")),
    updateAgentWorkflow: (task: string, body: AgentWorkflowInput) =>
      request<AgentWorkflow>(url(`/api/admin/workflows/${encodeURIComponent(task)}`), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    agentFeedback: () =>
      request<{ items: AgentFeedback[] }>(url("/api/admin/workflows/feedback")),
    createAgentFeedback: (body: Pick<AgentFeedback, "workflow_task" | "rating" | "category" | "message">) =>
      request<AgentFeedback>(url("/api/admin/workflows/feedback"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    reviewAgentFeedback: (id: string, status: "reviewed" | "applied" | "dismissed") =>
      request<AgentFeedback>(url(`/api/admin/workflows/feedback/${encodeURIComponent(id)}`), {
        method: "POST",
        body: JSON.stringify({ status })
      })
  };
}
