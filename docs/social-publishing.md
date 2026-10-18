# Facebook Page publishing

The backend prepares original Sinhala image posts from publishable news evidence. Selection runs every 15 minutes, makes one `social_post_editorial` gateway call for the batch, and writes a review queue. Publishing checks the queue every three minutes and is also enqueued after approval. Both settings `enabled` and `auto_publish` default to false. Enabling selection alone keeps human approval required.

Cards contain our headline, summary and the credited sources placed on the coverage spectrum (left, centre and right by each source's average analysed lean; unanalysed sources are listed separately). Publisher logos come from the bundled copies in `services/api/internal/social/assets/source-logos` or local media storage; remote logo URLs are not fetched, so those sources show their initial. Captions credit the sources and never contain a URL; when `SNAP_SOCIAL_INCLUDE_LINK` is true the card footer shows the website name (for example `thulawanews.com`). Cards render at 2× (2160×2700) so text stays sharp after Facebook recompresses the image. Publisher images and verbatim publisher text are never used. The card renderer returns a 1080×1350 PNG; media storage saves it as `social/posts/{postId}.png`. The publisher hands the PNG to [Loomcast](https://loomcast.prabhavalabs.com/llms.txt), which posts it on the Facebook Page through its own published Meta app; media need not be public. The API and worker must share local media storage, or both use the same R2 bucket.

## Automated pipeline

The pipeline runs end to end without a person. Nothing publishes unless the AI quality review passes it, and the AI desk fixes what the reviewer finds instead of waiting for an editor.

1. **Arrival.** When an article's event synthesis is refreshed, the worker schedules a selection run two minutes later (`trigger: realtime`). Runs are unique per queue, so a burst of coverage produces one run. The 15-minute periodic run continues as a safety net (`trigger: scheduled`). Turn the arrival trigger off with `realtime_enabled`.
2. **Selection.** Candidates are recent breaking events or events covered by at least two sources; with **Include single-source stories** (`include_single_articles`) on, single-outlet events and standalone articles qualify too. Most stories reach the site from one outlet, so without it the queue can stay empty for hours. Breaking news always goes first; the rest are ranked by topic (politics first, then the economy, official announcements and national affairs; entertainment last), the number of sources and freshness, losing priority for every hour since the story last changed. The query fetches three times `candidates_per_run` and keeps the top-ranked ones. The editorial model (`social_post_editorial`) also receives `page_activity` (posts in the last 24 hours and hours since the last post) so a quiet Page gets a steady flow of public-interest stories instead of only dramatic ones. It judges news value and reader interest alone, on a fixed scale (90–100 major national news people need now, 80–89 important, 70–79 solid public interest, below 70 not posted), and is told never to decline a story because of the daily limit or the time of day, and never to lower the bar to fill a quiet Page: when nothing is good enough, nothing is posted. It scores public interest and writes the Sinhala headline, summary and caption. Items below `min_score` are recorded as `skipped`.
   - **No repeats.** The model also receives the posts already published or queued in the last 48 hours and declines candidates that repeat a story, including follow-ups without a major new development, and avoids posting the same kind of story again and again. As a backstop, a draft whose headline or summary shares most of its words with a recent post is recorded as `skipped` with the post it repeats in the reason.
   - **Topic balance.** The publisher lowers a queued post's rank for each post from the same category published in the last six hours, so the Page does not run one kind of story back to back. Nothing waits for an editor because of it.
3. **Draft.** Selected items are inserted as `pending_review` and the card is rendered. No post is approved at insert time.
4. **AI quality review.** A second model call (`social_post_review`) checks the post against the evidence (event synthesis and the evidence articles' summaries) for accuracy, attribution, neutrality, natural Sinhala, legal safety and completeness. It returns pass/fail, a 0–100 score, a summary and issues graded blocker, major or minor. The result is saved on the post (`review_status`, `review_score`, `review_summary`, `review_issues`).
5. **Automatic rewrite.** When the review fails (a blocker or major issue, or a score below `min_review_score`), the editor model rewrites the post to fix the listed issues, the card is re-rendered and the rewrite is reviewed again, up to two rewrites (`review_revisions`). The reviewer also receives the posts published or queued in the last 48 hours and sets `repeats_recent_post` when the post reports the same story without a major new development (the same incident from another outlet, or with a different or updated figure such as a death toll, is a repeat unless the earlier post is more than 12 hours old and the scale changed markedly); a repeat is dropped (`skipped`) rather than rewritten.
6. **Decision.** If `auto_publish` is on and the review passes (pass, score ≥ `min_review_score`, no blocker or major issue, not a repeat), the post is approved, scheduled immediately and the publisher is enqueued. `enabled` and `auto_publish` are re-checked under lock at that moment. A post that still fails after two rewrites is dropped with the reviewer's summary in `last_error`. When the model is unavailable the post waits, and each selection run first settles up to four waiting automatic posts (highest score first) before drafting new ones; posts still waiting 12 hours after drafting are dropped as stale. Manual posts keep one review and one rewrite and wait for their editor.
7. **Publishing.** The publisher sends the highest-ranked due post: breaking news first, then by selection score, lowered for categories just carried and by two points per hour of age. The limits are soft: breaking news and posts scored 85 or more may pass the daily and hourly limits, and breaking news may also publish during quiet hours with only a five-minute gap after the previous post. Other posts keep the hourly limit, the minimum gap and quiet hours. Just before upload, an automatic post that repeats one published since it was queued is dropped: first by word overlap, then by an AI check (the `social_post_review` model) against every post published since the post was approved or last checked (`repeat_checked_at`). This catches two outlets' reports of one incident that were both queued before either went out. If the AI check cannot run, the post publishes on the strength of its review. The post is then handed to Loomcast, scheduled about two and a half minutes ahead (Loomcast needs at least two); see [Publishing through Loomcast](#publishing-through-loomcast). A post approved automatically that is still queued 12 hours after it was drafted is dropped as stale (`skipped`); editor approvals are kept.

Post lists from `GET /api/admin/social/posts` and `GET /api/admin/social/content` include a `plan` for each post: whether it will publish (`published`, `publishing`, `scheduled`, `paused`, `in_review` for automatic posts the AI desk is still settling, `needs_editor`, `not_publishing`), the time (an estimate for `scheduled`, replaying the queue against the caps, gap and quiet hours) and a plain-English note. The calendar and review queue show it. A failed or unavailable AI review never publishes automatically; an editor can still fix a post and re-run the review, or approve it anyway.

The reviewer sees each evidence article's summary and up to 1,800 characters of its stored full text, accepts reasonable editorial inference (for example the month of a date from the reports' publication dates) and fails a post only for blocker or major issues: legal or safety risk, contradiction of the evidence, invented or wrong names, numbers, dates or outcomes, unattributed accusations, partisan wording or unclear Sinhala. Wording preferences and missing detail are minor. The review runs on the strongest model available (`gpt-6.1-sol`) at high reasoning effort. After a rewrite only the fresh review decides.

Every post records its `channel` (currently only `facebook`) and its `trigger` (`scheduled`, `realtime` or `manual`). The admin **Content calendar** (`/social/content`) shows posts by the day they published, are scheduled, or were drafted, with a filterable content log and the channel list (Instagram, X, WhatsApp Channel and Telegram are listed as planned).

## Publishing through Loomcast

Thulawa News does not hold a Facebook token. Loomcast's machine API publishes on the Page with Loomcast's own published Meta app, so posts are public without a Meta app of ours going through App Review or business verification.

### Connect

1. In Loomcast, connect the Thulawa News Facebook Page under **Settings → Channels** and turn its **Dry run** switch off.
2. Under **Settings → API clients**, create a client for this site, choose its project and grant the Thulawa News Facebook channel at **publish** access. With `write` access, or with **Requires approval** on the channel, every post waits for the owner in Loomcast's approvals (the admin Social page warns about this).
3. Put the key (`lc_api_…`, shown once) in the server's `.env` as `SNAP_LOOMCAST_API_KEY` and restart the API and worker. When the client is granted more than one Facebook channel, also set `SNAP_LOOMCAST_CHANNEL_ID`.
4. Open `/social` in the admin. The **Publishing through Loomcast** card shows the channel and says what to fix when publishing is not ready: a refused key, no or several granted Facebook channels, a channel that needs reconnecting, read-only access, or dry run. Publishing holds until it is ready; posts stay approved.

### How a post goes out

1. The publisher claims the next due post (`publishing`, attempts + 1, `handed_off_at` = now + 150 s) and creates an `image` post in Loomcast with the caption, the card (PNG, at most 4 MB), the channel, that time and `externalRef` = our post id. The `Idempotency-Key` is `thulawa-social-{postId}-{attempt}`, so a repeated request returns the same Loomcast post. The Loomcast post id is saved at once (`loomcast_post_id`).
2. It uploads the card to the presigned URL Loomcast returns and finalizes the post. Loomcast approves it on the owner's behalf (publish access) and queues it for its time.
3. Each run (every three minutes) first reads back every handed-off post and records the outcome: `published` with Facebook's post id and permalink; `failed` with Loomcast's reason; queued again when Loomcast marks the failure retryable (up to five attempts); a note while Loomcast waits for approval or holds the post for a Facebook rate-limit window.

A handed-off post counts against the caps and the minimum gap from its handoff time. Loomcast also keeps its own limit of 30 Facebook posts per channel in a rolling 24 hours; keep `max_posts_per_day` below that.

Loomcast publishes nothing before finalize. A handoff that fails part way is therefore withdrawn (`cancel`) and queued again, and a run that finds a post still awaiting its files in Loomcast withdraws it and starts over. An editor's retry withdraws the earlier Loomcast copy first; when Loomcast has already published or is publishing it, the retry is refused.

## Environment

All variables are optional. `configured` requires a Loomcast API key; Chrome availability is independent.

| Variable | Default / purpose |
| --- | --- |
| `SNAP_LOOMCAST_API_KEY` | Loomcast API client key (`lc_api_…`); server only |
| `SNAP_LOOMCAST_URL` | `https://loomcast.prabhavalabs.com` |
| `SNAP_LOOMCAST_CHANNEL_ID` | Only when the client is granted several Facebook channels: the Thulawa News channel id |
| `SNAP_SOCIAL_INCLUDE_LINK` | `false`. When true, the card footer shows the website name from `SNAP_SOCIAL_PUBLIC_BASE_URL`. Captions never contain a URL. |
| `SNAP_SOCIAL_PUBLIC_BASE_URL` | `SNAP_NEWSLETTER_BASE_URL`, otherwise `https://thulawanews.com` |
| `SNAP_SOCIAL_CHROME_PATH` | Chrome/Chromium executable; production defaults to `/usr/bin/chromium`, local empty uses auto-detection |
| `SNAP_SOCIAL_CHROME_NO_SANDBOX` | Production defaults to `true`; renderer parses a boolean, root always requires no sandbox |

Production Compose forwards the variables to the API and worker and shares `/data/media` with both. Never place the key in client configuration or source control.

## Selection and review

Events must have changed within the lookback window, have synthesis or member summaries, and be breaking, have at least two distinct publishable sources, or qualify through `include_single_articles`. Sources must be active, rights must not be disabled or internal-verification-only or expired, and article and event categories must not be held. Optional standalone articles require a summary and the same rights checks. If an event contains excluded member articles, selection falls back to the permitted member summaries and omits its aggregate spectrum to avoid using excluded material. Candidates rank by breaking status, source count, article count, then recency. The default window is six hours and the batch limit is eight.

The model scores public interest from 0 to 100, supplies an English selection reason, and writes Sinhala headline, summary and caption. The backend clamps scores and text lengths, allows at most three valid hashtags and rejects unknown IDs, duplicates and invalid publishable items. Missing/unavailable model providers leave the queue unchanged. At the default score threshold of 70, recommended items become `pending_review`; lower-scored/declined items become `skipped`. Auto-publish instead marks qualifying items `approved` and schedules them immediately, subject to all caps.

A subject with a queued, published or ambiguous post cannot be selected again. Rejected and failed subjects can be reselected. Skipped subjects qualify again only after newly becoming breaking or gaining at least two sources since the last skipped evaluation. Manual creation ignores lookback, the multi-source threshold and minimum score, keeps rights/category restrictions and dedupe, and always creates a review item.

Review the source evidence, headline, summary, caption and card. Edits are permitted for `pending_review`, `approved` and `failed` posts and invalidate/re-render the image. Captions are the complete text sent to Facebook; edits must retain the source credit line and site link. A render failure saves `last_error` without removing the post. Use Render again once Chrome/storage is available. An approved post without a readable image remains approved and does not upload. Rights and category availability for every snapshotted evidence article are checked again before upload; changed eligibility leaves the post approved with a diagnostic so it can be rejected and prepared again from current evidence.

## Caps and scheduling

Defaults: two posts per rolling hour, thirty per local calendar day (a soft limit, see step 7), at least twenty minutes between posts, and quiet hours 00:00–06:00 in `Asia/Colombo`. The timezone is configurable and validated. Quiet windows can span midnight; equal start/end hours disable quiet hours. All successful posts count against caps, including manual approvals and auto-publish. The publisher selects the highest-ranked due approved post (see step 7), claims it using a row lock and increments attempts. River uniqueness, a single social worker queue and a database session advisory lock prevent simultaneous publishers across processes.

`scheduled_for` is an earliest publishing time, not a guarantee of exact delivery. Quiet hours, account state, caps and image readiness can postpone delivery. Disabling `enabled` pauses both scheduled selection and publishing. Manual creation/editing remain available. `next_publish_eligible_at` reports the earliest time permitted by the Loomcast connection, a Loomcast-requested pause, caps, quiet hours and approved schedules; it does not guarantee image readiness.

## Failure states

| State / failure | Behavior |
| --- | --- |
| `pending_review` | Await approval or rejection |
| `approved` | Await schedule, caps, a ready Loomcast channel and image |
| `publishing` | Handed to Loomcast, waiting for it to go out; editing/retry blocked |
| `published` | Facebook post ID, publication time and permalink saved from Loomcast |
| `rejected` / `skipped` | Not published; reason/history retained |
| Loomcast refuses the key (401/403) | Post returns to approved; publishing pauses 30 minutes; the Social page shows the problem |
| Loomcast rate limit (429) | Post returns to approved; publishing pauses for `Retry-After` (at least a minute) |
| Loomcast unreachable or 5xx before finalize | Unfinished Loomcast post withdrawn; post returns to approved five minutes later |
| Loomcast refuses the post (validation, media) | `failed`; manual retry available |
| Finalize answer lost | Stays `publishing`; the next run reads it back, and withdraws and requeues it if it was never finalized |
| Worker stopped before saving the Loomcast id | After ten minutes the post returns to approved (nothing was finalized) |
| Loomcast reports failed, missed or cancelled | `failed` with the reason, or approved again when Loomcast marks it retryable |
| Loomcast published it as a dry run | `failed`; turn dry run off for the channel, then retry |
| Still not published six hours after handoff, or Loomcast no longer shows it | `needs_attention`; check Loomcast and the Page |

Before retrying `needs_attention`, check the actual Page for the post. Retry requires `confirm_not_published: true` and withdraws the earlier Loomcast copy first.

## Admin API

All routes use existing admin sessions, require the administrator role, and mutations require `X-SNAP-CSRF: 1` and a JSON object body, including `{}` for actions without fields. Unknown JSON fields are rejected.

- `GET /api/admin/social/status`: `configured`, `ready`, the Loomcast `channel`, `problem` / `warning`, `publish_retry_at`, settings, status counts, next eligible time.
- `GET` / `POST /api/admin/social/settings`: complete settings object.
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
- `GET /api/admin/social/channels`: publishing channels and their status (`connected`, `needs_attention`, `disconnected` or `planned`).

## Security and verification

The Loomcast key stays on the server: it is read from the environment, sent only to Loomcast as a bearer token, never to the presigned upload URL, and never stored or returned. Loomcast error messages are saved on posts without the key. Administrator mutations write the existing audit log in the same transaction as the corresponding state change.

Migration `000054_social_publishing` adds settings, the post queue and indexes, and the editorial task/workflow profile. Migration `000055_social_publishing_safety` persists the exact article IDs used as evidence. Migration `000056_social_automation` adds `realtime_enabled` and `min_review_score` settings, the post `channel`, `trigger` and review columns, a calendar index, and the `social_post_review` task/workflow profile. Migration `000069_loomcast_publishing` retires the Meta app's Page token and OAuth tables, and adds `loomcast_post_id` and `handed_off_at` to posts and the publishing pause `publish_retry_at` to settings. Integration tests follow `SNAP_TEST_DATABASE_URL` and create temporary schemas that clone the migrated tables. They drive the publisher against a fake Loomcast client; the Loomcast client itself is tested against a local HTTP server. No live Loomcast call or real Facebook post is part of the automated tests.
