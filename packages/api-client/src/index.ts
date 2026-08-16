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
  polling_interval_seconds: number;
  verified_official: boolean;
};

export type AdminRights = {
  id: string;
  source_id: string;
  endpoint_id: string;
  mode: string;
  attribution: string;
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

export type QueueItem = {
  id: string;
  headline: string;
  public_status: string;
  source: string;
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

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: "include",
    ...init,
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
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
        mfa_required: boolean;
        mfa_setup: boolean;
        mfa_token: string;
        otpauth_url?: string;
        otpauth_qr?: string;
      }>(url("/api/admin/login"), {
        method: "POST",
        body: JSON.stringify({ email, password })
      }),
    verifyMfa: (mfa_token: string, code: string) =>
      request<{ ok: boolean }>(url("/api/admin/mfa"), {
        method: "POST",
        body: JSON.stringify({ mfa_token, code })
      }),
    logout: () =>
      request<{ ok: boolean }>(url("/api/admin/logout"), { method: "POST" }),
    me: () =>
      request<{ email: string; name: string; role: string }>(
        url("/api/admin/me")
      ),
    overview: () => request<Overview>(url("/api/admin/overview")),
    queue: () => request<{ items: QueueItem[] }>(url("/api/admin/queue")),
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
    complaints: () =>
      request<{ items: AdminComplaint[] }>(url("/api/admin/complaints")),
    resolveComplaint: (id: string, status: string, resolution: string) =>
      request<{ ok: boolean }>(url(`/api/admin/complaints/${id}`), {
        method: "POST",
        body: JSON.stringify({ status, resolution })
      }),
    adminSources: () =>
      request<{ items: AdminSource[] }>(url("/api/admin/sources")),
    adminSource: (id: string) =>
      request<AdminSource>(url(`/api/admin/sources/${id}`)),
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
    archiveSource: (id: string) =>
      request<{ ok: boolean }>(url(`/api/admin/sources/${id}/archive`), {
        method: "POST"
      }),
    setSourceActive: (id: string, active: boolean) =>
      request<{ ok: boolean }>(url(`/api/admin/sources/${id}/active`), {
        method: "POST",
        body: JSON.stringify({ active })
      }),
    adminEndpoints: (sourceId: string) =>
      request<{ items: AdminEndpoint[] }>(
        url(`/api/admin/sources/${sourceId}/endpoints`)
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
    adminRights: (sourceId: string) =>
      request<{ items: AdminRights[] }>(
        url(`/api/admin/sources/${sourceId}/rights`)
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
    llmProviders: () =>
      request<{ items: LlmProvider[] }>(url("/api/admin/llm/providers")),
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
    llmProfiles: () =>
      request<{ items: LlmProfile[] }>(url("/api/admin/llm/profiles")),
    upsertProfile: (body: LlmProfile) =>
      request<{ ok: boolean }>(url("/api/admin/llm/profiles"), {
        method: "POST",
        body: JSON.stringify(body)
      })
  };
}
