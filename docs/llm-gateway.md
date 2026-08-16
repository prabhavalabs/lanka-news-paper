# LLM Gateway — Configurable Model Routing

**Goal:** All LLM usage in the pipeline goes through one internal gateway service. Routing, models, reasoning effort, and credentials are configured in the database via the admin portal — never hardcoded. Two route types: Codex CLI (subscription-backed) and OpenAI-compatible APIs (key-backed). Every task profile has a fallback chain.

---

## 1. Architecture

```
pipeline job (classify / summarize / brief / breaking)
        │
        ▼
   LLM Gateway (internal Go package + admin API)
        │  resolves task → active TaskProfile → ProviderRoute
        ├──► Route A: codex_cli  — spawns `codex exec` on the server
        └──► Route B: openai_api — HTTP to any OpenAI-compatible endpoint
                     (OpenAI, Gemini-compat, DeepSeek, OpenRouter, local proxy…)
```

The gateway is a Go package inside the monolith plus admin CRUD endpoints. It exposes one internal function:

```go
type Request struct {
    Task        string            // "classify" | "cluster_summary" | "daily_brief" | "breaking_check" | ...
    System      string
    Input       string
    JSONSchema  *jsonschema.Def   // optional structured output
}
Complete(ctx, Request) (Response, error)
```

Callers never name a model. The task name is the only coupling.

## 2. Database Schema

```sql
-- A credentialed way to reach models
CREATE TABLE llm_providers (
  id            TEXT PRIMARY KEY,          -- "codex-sub", "deepseek", "gemini", "openrouter"
  kind          TEXT NOT NULL,             -- 'codex_cli' | 'openai_api'
  base_url      TEXT,                      -- openai_api only
  api_key_ref   TEXT,                      -- name of secret in secret store / env, never the key itself
  enabled       BOOLEAN NOT NULL DEFAULT false,
  status        TEXT NOT NULL DEFAULT 'unknown',  -- 'healthy' | 'degraded' | 'auth_expired' | 'quota_exhausted'
  status_detail TEXT,
  checked_at    TIMESTAMPTZ
);

-- What each pipeline task uses, with ordered fallbacks
CREATE TABLE llm_task_profiles (
  task            TEXT NOT NULL,           -- "classify", "cluster_summary", ...
  priority        INT  NOT NULL,           -- 0 = primary, 1 = first fallback, ...
  provider_id     TEXT NOT NULL REFERENCES llm_providers(id),
  model           TEXT NOT NULL,           -- "gpt-5.6-luna", "deepseek-chat", "gemini-flash-latest"
  reasoning_effort TEXT,                   -- "minimal" | "low" | "medium" | "high" (mapped per provider)
  max_output_tokens INT,
  temperature     NUMERIC,
  timeout_seconds INT NOT NULL DEFAULT 60,
  enabled         BOOLEAN NOT NULL DEFAULT true,
  PRIMARY KEY (task, priority)
);

-- Every call, for cost/quota visibility in admin
CREATE TABLE llm_calls (
  id           BIGSERIAL PRIMARY KEY,
  task         TEXT, provider_id TEXT, model TEXT,
  input_tokens INT, output_tokens INT,
  latency_ms   INT, outcome TEXT,          -- 'ok' | 'error' | 'fallback' | 'quota'
  created_at   TIMESTAMPTZ DEFAULT now()
);
```

Fallback logic: try priority 0; on auth error, quota error, timeout, or 5xx → mark provider status, try next priority. All fallbacks logged. Admin dashboard shows per-provider health and daily token spend.

## 3. Route A — Codex CLI (subscription-backed)

### 3.1 Execution

- Server has `codex` CLI installed. Gateway runs headless invocations:
  `codex exec --json --model <model> -c model_reasoning_effort=<effort> "<prompt>"`
  with sandbox read-only, no repo context, working dir = empty scratch dir.
- Output parsed from `--json` event stream; last agent message is the completion.
- Concurrency limited to 1–2 processes; queue in front (River job or semaphore).
- Structured output: instruct JSON in prompt + validate; retry once on parse failure.

### 3.2 "Authenticate with Codex" button (admin portal)

`codex login` (ChatGPT sign-in) opens a browser and expects the OAuth redirect on `localhost:1455` **on the machine running the CLI**. On a headless VPS there are two workable flows; implement both, prefer Flow 1.

**Flow 1 — Remote login relay (button-driven):**
1. Admin clicks **Authenticate with Codex**.
2. Backend spawns `codex login` on the VPS, captures the printed auth URL, shows it in the admin UI as a link + copy button, along with one-line instructions.
3. The OAuth redirect targets `localhost:1455`. The admin UI instructs the admin to run one SSH command from their laptop first:
   `ssh -L 1455:localhost:1455 user@vps`
   With the tunnel up, the admin opens the auth URL in their local browser, signs in to ChatGPT, and the callback lands on the VPS through the tunnel.
4. Backend polls `codex login status`; on success, stores nothing itself — credentials live in `~/.codex/auth.json` (or OS keyring) owned by the service user. UI flips to "Authenticated as <account>".

**Flow 2 — Credential upload (fallback):**
1. Admin runs `codex login` on their own machine, completes browser sign-in.
2. Admin uploads their local `~/.codex/auth.json` through the admin UI (admin-only, MFA-gated endpoint).
3. Backend writes it to the service user's `CODEX_HOME` with `0600` permissions and verifies with `codex login status`.

**Status panel** in admin: authenticated account, token validity (run `codex login status` on demand and on a 6h schedule), last successful call, quota-exhaustion state with reset hint. On `auth_expired`, alert operator (email) and auto-fail-over to Route B profiles.

### 3.3 Honest constraints (surface these in the admin UI)

- Subscription auth is designed for interactive use; OpenAI's docs route programmatic/CI workloads to API keys. This route is best-effort: expect weekly usage caps and occasional re-login.
- Tokens refresh automatically while in use, but a revoked session requires the manual flow again. Never page the pipeline on this — fallback handles it.
- Do not use this route for the public-facing breaking-news path where latency and reliability matter most; keep it for batch tasks (daily brief drafting, cluster summaries) where a fallback retry an hour later is acceptable.

## 4. Route B — OpenAI-compatible API

- One HTTP client, provider differences = config only: `base_url`, `api_key_ref`, model name, and a per-provider quirk map (e.g., which parameter name controls reasoning effort; Gemini's OpenAI-compat layer vs DeepSeek vs OpenRouter).
- Secrets: `api_key_ref` names an environment variable / mounted secret on the VPS. Keys are never stored in Postgres, never returned by any API, write-only in the admin UI.
- Recommended launch config:
  - `classify` → Gemini Flash (free tier) → DeepSeek → skip (fallback to keyword rules, non-LLM)
  - `cluster_summary` → Codex CLI route → Gemini Flash → DeepSeek
  - `daily_brief` → Codex CLI route (higher effort) → DeepSeek reasoner
  - `breaking_check` → **no LLM** (rule-based: N sources / M minutes); LLM only optionally annotates after send

## 5. Admin UI Requirements

Settings → **AI & Routing** page:

1. **Providers list:** kind, health badge (text + icon, no color-only), base URL, key set/unset, enable toggle, "Test" button (runs a 1-token ping, shows latency + result).
2. **Codex panel:** authenticate button (Flow 1), upload fallback (Flow 2), account + quota status.
3. **Task profiles table:** per task, ordered provider/model/effort rows, drag-to-reorder fallbacks, edit inline. Changes take effect on next job run — no deploy, no restart.
4. **Usage view:** calls/tokens/estimated cost per provider per day (from `llm_calls`), fallback-trigger counts.
5. All mutations audited (who, when, before/after) per SRS FR-EDT-004.

## 6. Non-negotiables

- Pipeline must degrade gracefully to **zero-LLM mode**: classification falls back to publisher-category mapping + keyword rules; cluster summaries are simply omitted; the paper still publishes. LLM outage must never block ingestion or publication.
- Every AI-generated public text carries model attribution metadata and the "automated summary" label (see UI guidelines §4.6, SRS FR-CLS-004).
- No AI output may introduce facts not present in source metadata (SRS FR-CLS-008); prompts for summaries must include only clustered article metadata and instruct extractive, source-grounded writing.
