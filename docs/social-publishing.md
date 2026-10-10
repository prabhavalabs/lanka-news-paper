# Facebook Page publishing

The backend prepares original Sinhala image posts from publishable news evidence. Selection runs every 15 minutes, makes one `social_post_editorial` gateway call for the batch, and writes a review queue. Publishing checks the queue every three minutes and is also enqueued after approval. Both settings `enabled` and `auto_publish` default to false. Enabling selection alone keeps human approval required.

Cards contain our headline, summary and the credited sources placed on the coverage spectrum (left, centre and right by each source's average analysed lean; unanalysed sources are listed separately). Publisher logos come from the bundled copies in `services/api/internal/social/assets/source-logos` or local media storage; remote logo URLs are not fetched, so those sources show their initial. Captions credit the sources and never contain a URL; when `SNAP_SOCIAL_INCLUDE_LINK` is true the card footer shows the website name (for example `thulawanews.com`). Cards render at 2× (2160×2700) so text stays sharp after Facebook recompresses the image. Publisher images and verbatim publisher text are never used. The card renderer returns a 1080×1350 PNG; media storage saves it as `social/posts/{postId}.png`. Facebook receives the bytes through a multipart `/photos` upload, so media need not be public. The API and worker must share local media storage, or both use the same R2 bucket.

## Automated pipeline

The pipeline can run end to end without a person, but nothing publishes unless an AI quality review passes it.

1. **Arrival.** When an article's event synthesis is refreshed, the worker schedules a selection run two minutes later (`trigger: realtime`). Runs are unique per queue, so a burst of coverage produces one run. The 15-minute periodic run continues as a safety net (`trigger: scheduled`). Turn the arrival trigger off with `realtime_enabled`.
2. **Selection.** Candidates are recent breaking events or events covered by at least two sources. Breaking news always goes first; the rest are ranked by topic (politics first, then the economy, official announcements and national affairs; entertainment last), the number of sources and freshness, losing priority for every hour since the story last changed. The query fetches three times `candidates_per_run` and keeps the top-ranked ones. The editorial model (`social_post_editorial`) scores public interest and writes the Sinhala headline, summary and caption. Items below `min_score` are recorded as `skipped`.
   - **No repeats.** The model also receives the posts already published or queued in the last 48 hours and declines candidates that repeat a story, including follow-ups without a major new development, and avoids posting the same kind of story again and again. As a backstop, a draft whose headline or summary shares most of its words with a recent post is recorded as `skipped` with the post it repeats in the reason.
   - **Topic balance.** A post that passes review publishes automatically only while its category has had fewer than two posts in the last six hours. Otherwise it waits for an editor with the reason in `hold_reason`. Breaking news is exempt.
3. **Draft.** Selected items are inserted as `pending_review` and the card is rendered. No post is approved at insert time.
4. **AI quality review.** A second model call (`social_post_review`) checks the post against the evidence (event synthesis and the evidence articles' summaries) for accuracy, attribution, neutrality, natural Sinhala, legal safety and completeness. It returns pass/fail, a 0–100 score, a summary and issues graded blocker, major or minor. The result is saved on the post (`review_status`, `review_score`, `review_summary`, `review_issues`).
5. **Decision.** If `auto_publish` is on and the review passes (pass, score ≥ `min_review_score`, no blocker or major issue), the post is approved, scheduled immediately and the publisher is enqueued. `enabled` and `auto_publish` are re-checked under lock at that moment. Every other outcome, including a review that could not run, waits in `pending_review` with the reviewer's notes. Editing a post clears its review; editors can re-run it with **Run AI review**.
6. **Publishing.** Caps, minimum gap and quiet hours still apply, as described below. Breaking posts are uploaded first. A post approved automatically that is still queued 12 hours after it was drafted is dropped as stale (`skipped`); editor approvals are kept.

Post lists from `GET /api/admin/social/posts` and `GET /api/admin/social/content` include a `plan` for each post: whether it will publish (`published`, `publishing`, `scheduled`, `paused`, `needs_editor`, `not_publishing`), the time (an estimate for `scheduled`, replaying the queue against the caps, gap and quiet hours) and a plain-English note. The calendar and review queue show it. A failed or unavailable AI review never publishes automatically; an editor can fix the post and re-run the review, or approve it anyway.

The reviewer sees each evidence article's summary and up to 1,800 characters of its stored full text, accepts reasonable editorial inference (for example the month of a date from the reports' publication dates) and fails a post only for blocker or major issues: legal or safety risk, contradiction of the evidence, invented or wrong names, numbers, dates or outcomes, unattributed accusations, partisan wording or unclear Sinhala. Wording preferences and missing detail are minor. When a review fails with a blocker or major issue, the editor model rewrites the draft once to fix the listed issues (`review_revisions`), the card is re-rendered and the rewrite is reviewed again; only that fresh review decides.

Every post records its `channel` (currently only `facebook`) and its `trigger` (`scheduled`, `realtime` or `manual`). The admin **Content calendar** (`/social/content`) shows posts by the day they published, are scheduled, or were drafted, with a filterable content log and the channel list (Instagram, X, WhatsApp Channel and Telegram are listed as planned).

## Set up a new Meta Business app

This version is intended for the owner's own Page with a dedicated app kept in Development mode. It does not implement an App Review submission or support connecting arbitrary customers' Pages.

1. Open [Meta for Developers](https://developers.facebook.com/apps/) with the Facebook user who manages the Page. Create a new app, select the **Business** app type / **Manage everything on your Page** use case, and enter an app name and contact address. Meta's dashboard may group these choices under use cases.
2. In the app's basic settings, record the App ID and App Secret in the deployment environment. Keep the app in **Development** mode.
3. Add **Facebook Login for Business**. In its settings, enable web OAuth login and add this exact **Valid OAuth Redirect URI**:
   `https://admin.thulawanews.com/api/admin/social/facebook/callback`.
   Register any local callback separately when testing locally; local default is `http://127.0.0.1:5174/api/admin/social/facebook/callback`. Redirect URI, port, scheme and path must match the configured value exactly.
4. Optionally create a Facebook Login for Business login configuration using a **User access token**, with `pages_show_list`, `pages_read_engagement` and `pages_manage_posts`. Assign the managed Page asset and content creation task. Put its configuration ID in `SNAP_META_LOGIN_CONFIG_ID`. Without this value, the authorization request asks for the three permissions using `scope`.
5. Add the owner as an **app administrator**, accept any pending app-role invitation, and confirm that the same Facebook user has Page access that includes creating content / managing the Page. A user who only analyzes or advertises on the Page is insufficient.
6. Sign in to the news admin using the administrator role, open `/social`, and connect Facebook. Accept the login permissions and select the Page assets. The backend lists only Pages whose tasks include `CREATE_CONTENT` or `MANAGE`.
7. With one eligible Page the account connects immediately. With several, choose a Page within ten minutes. The backend verifies the token's app, expiry and Page permissions before saving it.
8. Create and inspect a review post, approve it, and check the resulting Page post and its permalink. Development-mode access and post visibility must be checked with the intended audience; a successful API upload alone does not establish public visibility. This workflow does not switch the Meta app to Live mode.

Meta references: [Facebook Login for Business](https://developers.facebook.com/docs/facebook-login/facebook-login-for-business/), [manual OAuth flow](https://developers.facebook.com/docs/facebook-login/guides/advanced/manual-flow/), [Page access tokens](https://developers.facebook.com/docs/pages-api/getting-started/), [securing Graph requests](https://developers.facebook.com/docs/graph-api/securing-requests/). The official documentation returned access/rate-limit errors during implementation, so verify dashboard labels and Development-mode visibility in the actual app before operating it.

## Environment

All variables are optional. `configured` requires App ID, App Secret and a valid token key; Chrome availability is independent.

| Variable | Default / purpose |
| --- | --- |
| `SNAP_META_APP_ID` | Dedicated Meta app ID |
| `SNAP_META_APP_SECRET` | App Secret; server only |
| `SNAP_META_REDIRECT_URL` | Production callback above; local callback on port 5174 |
| `SNAP_META_LOGIN_CONFIG_ID` | Optional Business Login configuration ID |
| `SNAP_META_GRAPH_VERSION` | `v26.0` |
| `SNAP_SOCIAL_TOKEN_KEY` | Base64 encoded, exactly 32 decoded bytes; AES-256-GCM |
| `SNAP_SOCIAL_INCLUDE_LINK` | `false`. When true, the card footer shows the website name from `SNAP_SOCIAL_PUBLIC_BASE_URL`. Captions never contain a URL. |
| `SNAP_SOCIAL_PUBLIC_BASE_URL` | `SNAP_NEWSLETTER_BASE_URL`, otherwise `https://thulawanews.com` |
| `SNAP_SOCIAL_CHROME_PATH` | Chrome/Chromium executable; production defaults to `/usr/bin/chromium`, local empty uses auto-detection |
| `SNAP_SOCIAL_CHROME_NO_SANDBOX` | Production defaults to `true`; renderer parses a boolean, root always requires no sandbox |

Generate the encryption key once with `openssl rand -base64 32` and save it privately. API and worker must use the same key. Preserve it across redeployments. Key version is currently `v1`; changing the key requires reconnecting the Page. Never place these secrets in client configuration or source control.

Production Compose forwards the variables to API and worker and shares `/data/media` with both. The admin Vite proxy forwards `/api` to port 8090. The admin frontend nginx forwards `/api/` to the API; the host nginx forwards the admin origin to that frontend. The callback therefore stays on the admin origin. The session cookie remains `SameSite=Strict`. The rate-limited OAuth callback authenticates only the single-use state and uses its stored initiating admin as the actor. The SPA's subsequent same-origin requests carry the session cookie; pending Page lookup and selection require the administrator session and matching state owner, with write-CSRF protection on selection.

## Selection and review

Events must have changed within the lookback window, have synthesis or member summaries, and be breaking or have at least two distinct publishable sources. Sources must be active, rights must not be disabled or internal-verification-only or expired, and article and event categories must not be held. Optional standalone articles require a summary and the same rights checks. If an event contains excluded member articles, selection falls back to the permitted member summaries and omits its aggregate spectrum to avoid using excluded material. Candidates rank by breaking status, source count, article count, then recency. The default window is six hours and the batch limit is eight.

The model scores public interest from 0 to 100, supplies an English selection reason, and writes Sinhala headline, summary and caption. The backend clamps scores and text lengths, allows at most three valid hashtags and rejects unknown IDs, duplicates and invalid publishable items. Missing/unavailable model providers leave the queue unchanged. At the default score threshold of 70, recommended items become `pending_review`; lower-scored/declined items become `skipped`. Auto-publish instead marks qualifying items `approved` and schedules them immediately, subject to all caps.

A subject with a queued, published or ambiguous post cannot be selected again. Rejected and failed subjects can be reselected. Skipped subjects qualify again only after newly becoming breaking or gaining at least two sources since the last skipped evaluation. Manual creation ignores lookback, the multi-source threshold and minimum score, keeps rights/category restrictions and dedupe, and always creates a review item.

Review the source evidence, headline, summary, caption and card. Edits are permitted for `pending_review`, `approved` and `failed` posts and invalidate/re-render the image. Captions are the complete text sent to Facebook; edits must retain the source credit line and site link. A render failure saves `last_error` without removing the post. Use Render again once Chrome/storage is available. An approved post without a readable image remains approved and does not upload. Rights and category availability for every snapshotted evidence article are checked again before upload; changed eligibility leaves the post approved with a diagnostic so it can be rejected and prepared again from current evidence.

## Caps and scheduling

Defaults: two posts per rolling hour, twelve per local calendar day, at least twenty minutes between posts, and quiet hours 00:00–06:00 in `Asia/Colombo`. The timezone is configurable and validated. Quiet windows can span midnight; equal start/end hours disable quiet hours. All successful posts count against caps, including manual approvals and auto-publish. The publisher selects the oldest due approved post, claims it using a row lock and increments attempts. River uniqueness, a single social worker queue and a database session advisory lock prevent simultaneous publishers across processes.

`scheduled_for` is an earliest publishing time, not a guarantee of exact delivery. Quiet hours, account state, caps and image readiness can postpone delivery. Disabling `enabled` pauses both scheduled selection and publishing. Manual creation/editing remain available. `next_publish_eligible_at` reports the earliest time permitted by account configuration, caps, quiet hours and approved schedules; it does not guarantee image readiness.

## Failure states

| State / failure | Behavior |
| --- | --- |
| `pending_review` | Await approval or rejection |
| `approved` | Await schedule, caps, connected account and image |
| `publishing` | Upload claimed; editing/retry blocked |
| `published` | Facebook post ID, publication time and optional permalink saved |
| `rejected` / `skipped` | No upload; reason/history retained |
| Authentication/permission error | Account becomes `needs_reconnect`; post returns to approved |
| Rate limit | Post returns to approved with a 30-minute account-wide cooldown |
| Transport failure before send | Post returns to approved with a five-minute delay |
| Timeout/network failure after send, malformed success, missing post ID | `needs_attention`; no automatic retry |
| Worker stopped while uploading | After ten minutes the claim becomes `needs_attention` |
| Definite Graph rejection | `failed`; manual retry available |
| Permalink lookup failure after confirmed upload | Remains published; post ID saved, diagnostic noted |

Before retrying `needs_attention`, check the actual Page for the post. Retry requires `confirm_not_published: true`. If it exists, do not retry; retain the ambiguous record for investigation. Disconnect deletes stored token ciphertext/nonce/key version and pending Page tokens. Reconnect safely replaces the previous account.

## Admin API

Except the state-authenticated OAuth callback, all routes use existing admin sessions, require the administrator role, and mutations require `X-SNAP-CSRF: 1` and a JSON object body, including `{}` for actions without fields. Unknown JSON fields are rejected.

- `GET /api/admin/social/status`: configuration, account metadata, settings, status counts, next eligible time.
- `GET` / `POST /api/admin/social/settings`: complete settings object.
- `POST /api/admin/social/facebook/connect`: `{}` → `{authorize_url}`.
- `GET /api/admin/social/facebook/callback?code=...&state=...`: 303 to `/social?facebook=connected`, `/social?facebook=choose_page&state_id=...`, or `/social?error=...`.
- `GET /api/admin/social/facebook/pending-pages?state_id=...`: `{items:[{id,name,picture}]}`; no tokens.
- `POST /api/admin/social/facebook/select`: `{state_id,page_id}`.
- `POST /api/admin/social/facebook/disconnect`: `{}`.
- `GET /api/admin/social/posts?status=&limit=30&cursor=`: `{items,next_cursor}`; limit 1–100. Cursor is opaque; stable newest-first ordering.
- `GET /api/admin/social/posts/{id}`: post, subject title/link and authenticated media URL.
- `PATCH /api/admin/social/posts/{id}`: optional `headline_si`, `summary_si`, `caption`, `hashtags`; requires at least one field.
- `POST /api/admin/social/posts/{id}/approve`: `{scheduled_for?}` (RFC3339 timestamp).
- `POST /api/admin/social/posts/{id}/reject`: `{}`.
- `POST /api/admin/social/posts/{id}/retry`: `{confirm_not_published?}`; failed or needs-attention only.
- `POST /api/admin/social/posts/{id}/render`: `{}`; rendering errors remain visible on the returned post.
- `POST /api/admin/social/posts`: exactly one of `{event_id}` or `{article_id}`; runs one editorial call and returns a review item.
- `POST /api/admin/social/run-selection`: `{}`; returns 202 after enqueueing; disabled settings make the job a no-op.
- `POST /api/admin/social/posts/{id}/review`: `{}`; re-runs the AI quality review on the current text. Never changes the post status.
- `GET /api/admin/social/content?from=&to=&channel=&status=`: RFC3339 range (at most 400 days), up to 1,000 posts by calendar time, newest first.
- `GET /api/admin/social/channels`: publishing channels and their status (`connected`, `needs_reconnect`, `disconnected` or `planned`).

## Security and verification

OAuth state is 32 random bytes; only SHA-256 is stored. It is bound to the signed-in admin, expires after ten minutes and is consumed atomically once. Page choices and their tokens are encrypted pending selection, never returned to the browser, and cleared on selection, expiry cleanup or disconnect. Page tokens use AES-256-GCM with random nonces and `social:{pageId}` AAD; pending choices use `social:oauth:{stateId}`. Derived long-lived Page tokens normally have no fixed expiry, but an expiry returned by token inspection is recorded and respected; token revocation and permission changes still require reconnecting.

Graph calls that use tokens send `appsecret_proof` (HMAC-SHA256 with the App Secret). Token-bearing redirects are refused; Page pagination uses Graph cursor values on the configured host. Stored/returned diagnostics exclude token values, OAuth codes and proof values. Callback redirects contain only a fixed error code or state row ID, never tokens. Administrator mutations write the existing audit log in the same transaction as the corresponding state change.

Migration `000054_social_publishing` adds settings, encrypted accounts, single-use OAuth states, the post queue and indexes, and the editorial task/workflow profile. Down migration removes those objects and its workflow history. Migration `000056_social_automation` adds `realtime_enabled` and `min_review_score` settings, the post `channel`, `trigger` and review columns, a calendar index, and the `social_post_review` task/workflow profile. Migration `000055_social_publishing_safety` persists the account-wide rate-limit cooldown and the exact article IDs used as evidence. Posts created before that snapshot must be rejected and prepared again before publishing. Integration tests follow `SNAP_TEST_DATABASE_URL` and create temporary schemas that clone the migrated tables, keeping fixture settings, accounts, articles and posts isolated. Unit tests cover crypto, fake Graph responses/proofs, editorial validation/captions, ranking/dedupe and scheduling. No live Meta app connection or real upload is part of the automated tests.
