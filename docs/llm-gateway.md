# Codex-only LLM gateway

All backend inference uses the official Codex CLI with ChatGPT-managed login.
The sole active provider is `codex_cli`; no API key or HTTP-provider fallback
is used. Historical provider/call records remain readable.

## Configuration and routing

`llm_task_profiles` assigns each task its provider, configured model, timeout,
enabled flag, and optional `reasoning_effort` (`low`, `medium`, `high`; empty
uses the model default). AI routing reads profiles dynamically, so newly added
tasks appear without a frontend task registry. Workflow Manager versions task
instructions and model/effort assignments. Watch Tower updates its retrieval
and answer profiles together.

`SNAP_CODEX_MODELS` defines the model allowlist exposed by the models and Codex
settings endpoints. Default: `gpt-6-luna,gpt-6.1-sol,gpt-6-sol,gpt-5.6-luna,gpt-5.6-terra`.
It is configuration, not proof that the account can access each model.
`SNAP_CODEX_CONCURRENCY` defaults to 2 and limits simultaneous CLI executions
within each process; API and worker do not share a global semaphore. Waiting
for a slot respects the parent context; the task timeout starts after acquisition.
River analysis workers match this limit (bounded to 64), and administrative
workers are capped at it. Pipeline and administrative jobs allow 45 minutes for
queueing, a 240-second combined call, and standalone fallback steps. Stale-run
recovery waits 50 minutes; newsletter jobs allow 20 minutes. Authenticated Watch
Tower messages and newsletter tests have a 20-minute request budget, 21-minute
Go write deadline and 22-minute proxy read budget. Newsletter test inference
has a 15-minute parent budget, including slot waiting.

## Article processing and fallback

The `article_analysis` task combines content cleaning, category, summary/key points, and narration
analysis into one structured Codex call per article. Pipeline steps consume the
combined result; when a required section is missing or invalid, the affected
step uses its existing individual task/fallback. Deterministic category rules,
model-applied political relevance, and event clustering remain in the pipeline. Successful steps
remain durable across retries. The admin renders profiles/workflows from their
endpoints rather than assuming a fixed list of analysis tasks.

Ordinary gateway calls can use their task fallback when completion fails:
classification has keyword rules, while other tasks receive an empty `none`
response and apply their caller-specific policy. Explicit model calls for
administrative backfills are strict so errors remain retryable. There is no
alternate LLM provider.

## Execution and telemetry

The gateway launches `codex exec` in a disposable empty workspace, with ephemeral
sessions, user config/rules ignored, read-only sandboxing, web search and
shell/browser/computer/app/multi-agent tools disabled. It sends input as data
and writes private instruction/schema files in the temporary directory. Child
environment variables are allowlisted, excluding database and provider secrets.
The workspace is removed after completion. The CLI manages authentication in `CODEX_HOME` and may update its caches and
logs there even for ephemeral runs. See deployment documentation for the shared
credential-volume refresh risk.

JSON events supply the completed response and usage. Pipeline completion details
include `input_tokens`, `output_tokens`, `cached_input_tokens`, and
`reasoning_tokens`. Cached/reasoning usage is in log details, not extra columns
on the historical `llm_calls` response. New calls do not emit
`provider_first_token`; inspectors tolerate absent first-token timing. Existing
streaming records and legacy provider labels remain visible.

## Readiness and cooldown

Provider readiness checks installation, CLI version and confirmed ChatGPT login.
API-key authentication does not satisfy readiness. Successful ChatGPT login
confirmation is cached for five minutes and invalidated on auth failure. Every
completion also enforces the CLI's `forced_login_method="chatgpt"` policy, so an
API-key switch during the cache interval cannot execute API-key inference. Provider status can be
`ready`, `unavailable`, `disabled`, `auth_expired`, or `quota_exhausted`.
`SNAP_CODEX_QUOTA_COOLDOWN` defaults to 15 minutes and applies to both auth and
quota errors. Persisted status coordinates pipeline blocking; blocked requests
do not extend the timestamp. A login probe does not clear a quota failure.
After the cooldown a completion may retry and successful execution clears the
stored failure. See [deployment operations](deployment.md) for login and rotation.

## Operating stance and account boundary

The intended operating stance is ChatGPT-managed authentication on a trusted
private server using a single owner account. This is not a claim that a ChatGPT
subscription grants unlimited inference or blanket approval for every deployment;
the owner must review the applicable account terms before operating it.
Execution is limited to trusted scheduled ingestion and authenticated editorial
administration. Public readers must not be able to trigger CLI execution or
submit arbitrary tool instructions. Do not expose this as a public inference
service or share/resell account access. Treat `auth.json` as a secret, restrict
volume access and backups, and use the official device login/refresh flow.
