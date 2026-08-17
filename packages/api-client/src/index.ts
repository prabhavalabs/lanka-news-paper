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
};

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

export type LlmProvider = {
  id: string;
  kind: string;
  base_url: string;
  enabled: boolean;
  status: string;
  key_set: boolean;
};

export type LlmProfile = {
  task: string;
  priority: number;
  provider_id: string;
  model: string;
  timeout_seconds: number;
  enabled: boolean;
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
      mentioned_articles: number;
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

export type AdminTableQuery = {
  page?: number;
  per_page?: number;
  search?: string;
  [filter: string]: string | number | undefined;
};

function withQuery(path: string, params: AdminTableQuery = {}) {
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
    breaking: () => request<{ items: PublicEvent[] }>(url("/api/v1/breaking")),
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
    createSource: (body: Omit<AdminSource, "id">) =>
      request<AdminSource>(url("/api/admin/sources"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    updateSource: (id: string, body: Omit<AdminSource, "id">) =>
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
    llmProviders: (params: AdminTableQuery = {}) =>
      request<PageResponse<LlmProvider>>(
        url(withQuery("/api/admin/llm/providers", params))
      ),
    upsertProvider: (body: {
      id: string;
      kind: string;
      base_url: string;
      enabled: boolean;
      api_key_ref: string;
    }) =>
      request<{ ok: boolean }>(url("/api/admin/llm/providers"), {
        method: "POST",
        body: JSON.stringify(body)
      }),
    llmProfiles: (params: AdminTableQuery = {}) =>
      request<PageResponse<LlmProfile>>(
        url(withQuery("/api/admin/llm/profiles", params))
      ),
    upsertProfile: (body: LlmProfile) =>
      request<{ ok: boolean }>(url("/api/admin/llm/profiles"), {
        method: "POST",
        body: JSON.stringify(body)
      })
  };
}
