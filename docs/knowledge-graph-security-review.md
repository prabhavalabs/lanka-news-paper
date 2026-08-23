# Public Knowledge Graph security review

Date: 2026-08-21

Scope: unauthenticated Knowledge Graph API and page, shared-URL construction, public article links, and existing HTTP/browser security controls.

## Executive summary

No Critical, High, or Medium severity findings remain in the implemented public Knowledge Graph flow. The public API uses a dedicated allowlisted response model and a public-only database query; it does not reuse or serialize the admin graph model. One Low availability-hardening item remains: add an edge rate limit before broad public promotion.

## Approved public data

The public response is limited to:

- graph generation time and selected time-window length;
- aggregate article, event, cross-source-event, and source counts;
- category slug and public Sinhala/English names with aggregate counts;
- event ID, public title, category, breaking flag, and last-update time;
- article ID, headline, public source ID/name, and publication time;
- high-level narrative label, economic-frame score, and confidence when analysis is relevant.

The explicit DTO is defined in `services/api/internal/publish/model.go:40-88`.

## Explicitly excluded data

The public DTO has no fields for:

- provider or model identifiers;
- prompts, pipeline/queue logs, internal errors, or request telemetry;
- cluster lock state, confidence, algorithm version, or first-seen metadata;
- narration rationale, evidence excerpts, mention terms, or party-curation evidence;
- source endpoints, rights configuration, health data, credentials, or administration status;
- publisher/original URLs or source icons.

The exclusion is regression-tested in `services/api/internal/httpapi/knowledge_graph_test.go:50-59`. Public article cards use internal relative routes in `apps/web/src/pages/knowledge-analysis-page.tsx:251-264`; the admin graph likewise routes to the internal admin article record in `apps/admin/src/pages/knowledge-graph-page.tsx:669-701`.

## Security controls reviewed

### Authorization boundary and data minimization

- The public route is intentionally registered outside the authenticated admin router at `services/api/internal/httpapi/router.go:40-56`.
- It returns the dedicated `publish.KnowledgeGraph` DTO, not the admin `desk.KnowledgeGraph` DTO.
- Narrative output is reduced to the three approved analytical values at `services/api/internal/publish/store.go:393-409`.

### Publication and rights enforcement

Both event discovery and returned article rows enforce all of the following in `services/api/internal/publish/store.go:338-382`:

- `public_status = 'published'`;
- active, non-archived source;
- rights mode is neither disabled nor internal-verification;
- rights profile has not expired;
- held categories are excluded;
- the requested publication window is enforced;
- the event scope is capped at 150 events.

Filtering values are SQL parameters; none are concatenated into SQL.

### Untrusted input handling

- Time windows accept only the existing 1/7/30-day presets or validated paired ISO dates, capped at 366 days (`services/api/internal/httpapi/admin.go:394-437`).
- Category input is length-bounded and used only as a SQL parameter (`services/api/internal/httpapi/news.go:92-115`).
- Source input must parse as a UUID before reaching the store (`services/api/internal/httpapi/news.go:108-114`).
- Public URL parameters are allowlisted and syntactically validated before use in the React page (`apps/web/src/pages/knowledge-analysis-page.tsx:18-32,285-299`). A requested node must also exist in the returned graph before it becomes selected.
- React text rendering is used throughout; there are no raw-HTML, `eval`, dynamic script, or `javascript:` URL sinks in the reviewed flow.

### Browser and HTTP controls

- API responses receive `Referrer-Policy`, `nosniff`, clickjacking denial, request IDs, recovery, a 1 MiB request-body cap, and origin-allowlisted CORS through the common router middleware (`services/api/internal/httpapi/router.go:98`, `services/api/internal/httpapi/middleware.go:22-28,49-77`).
- The public endpoint is read-only and cacheable for 60 seconds with stale revalidation (`services/api/internal/httpapi/news.go:120-121`).
- Production frontend responses set a restrictive CSP, permissions policy, referrer policy, `nosniff`, and frame denial (`infra/production/frontend.conf:9-13`).
- Share URLs are constructed from an allowlisted set of query keys and known local/production origins (`apps/admin/src/pages/knowledge-graph-page.tsx:238-255`).

## Findings

### KG-SEC-001 — No endpoint-specific public rate limit

- Severity: Low
- Location: `services/api/internal/httpapi/router.go:49`, `services/api/internal/httpapi/news.go:92-121`
- Evidence: the new route is intentionally unauthenticated and cached, but neither the API middleware nor production Nginx configuration applies an endpoint-specific request rate.
- Impact: a client could repeatedly force graph database reads after cache misses, creating avoidable availability pressure. The query is read-only, parameterized, time-bounded, and capped to 150 event scopes, so confidentiality and integrity are not affected.
- Recommended fix: apply a Cloudflare or Nginx per-IP rate limit to `GET /api/v1/knowledge-graph` before broad end-user promotion; keep the current cache policy.
- Current mitigation: 60-second public caching, bounded time windows, a 150-event scope cap, and no mutation path.

## Verification performed

- Full Go test suite passed.
- Admin and public production builds passed; admin graph unit tests passed.
- Invalid source UUID returned HTTP 400.
- Live public JSON was scanned for provider, model, original URL, algorithm, lock, evidence, rationale, endpoint, and rights field names; none were present.
- Live response headers contained the expected cache and API security headers.
- Browser validation confirmed the page is unauthenticated, node filters are encoded in the URL, related article links stay internal, reset removes focus filters, and the 390 px layout has no horizontal document overflow.
