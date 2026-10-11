# Front page and editorials

The public front page reads like an edited newspaper. A **chief editor** (`chief_editor` role) chooses the lead and the top stories and writes the editorial (කතුවැකිය). Places the editor leaves open fill automatically, so the page is never empty. Administrators have the same rights.

## Roles

Migration `000060_editorial_front_page` adds `chief_editor` to `admin_users.role`. There is no user-management screen yet; to appoint a chief editor, update the user's role in the database:

```sql
UPDATE admin_users SET role = 'chief_editor' WHERE email = 'editor@example.com';
```

Chief editors see **Front page** and **Editorials** in the admin sidebar, plus the standard newsroom pages. Every change is recorded in `audit_logs`.

## Front page

`GET /api/v1/front-page` returns:

- `lead`: one story;
- `top`: six or more stories;
- `more`: twelve or more stories;
- `editorial`: the latest published editorial.

A story is an event (one news item reported by one or more outlets) or a single article. Every story lists each outlet that reported it, with that outlet's headline and a link to the original article, so readers can always go to the source.

**Curated places.** The chief editor builds the page in **Admin → Front page** and publishes it with one save:

- a lead, up to 12 top stories and up to 24 more stories;
- an optional short note for readers on each story;
- an optional "keep until" time, after which the story leaves the page.

Placements live in `front_page_placements`. Each story can be placed once. A placed story disappears from the page if it stops being public, for example when its rights are withdrawn or its articles are held.

**Automatic places.** Open places fill from events updated in the last 36 hours that have at least one public article from a source allowed on the front page. Stories are ranked by:

- the number of outlets reporting the story (counted up to six);
- the section: politics first, then the economy, official notices and local news; entertainment last;
- a bonus for breaking news;
- freshness: the score drops every hour after the story's last update.

A story whose headline or summary largely matches one already on the page is skipped. This catches the same news split across two clusters. Matching compares five-character word stems, so different Sinhala word endings still match.

**Sources on the front page.** `sources.front_page` (default on) controls whether an outlet's stories may fill automatic places. The editor can still place an excluded outlet's story by hand. Excluded outlets still appear in sections, search and story pages.

The public response is cached for 30 seconds. Any editor change clears the cache at once.

## AI editions

The worker job `front_page.edition` runs every 30 minutes from 05:00 to midnight Colombo time (`edition` queue, one at a time). It skips a run when the top stories and the editor's placements have not changed and the last edition is under three hours old. It also runs at once after the chief editor changes placements or sources, and from **Rebuild edition now** in the admin. To build one edition by hand, run `go run ./cmd/edition` (add `-images=false` to skip drawing).

Each run works in four steps:

1. **Layout.** The layout editor (`edition_layout` task) receives up to 36 ranked candidates and the editor's placements. It returns 16–24 stories in reading order, each with a block size:

   | Size | Count | Report length |
   | --- | --- | --- |
   | `lead` | 1 | about 450 words |
   | `feature` | 2–3 | about 300 words |
   | `standard` | 4–6 | about 180 words |
   | `short` | 4–6 | about 110 words, uses the story summary |
   | `brief` | 4–8 | about 60 words, uses the story summary instead of a written report |

   It marks at most three stories for an illustration. The result is normalised: one lead, no repeats, every placement included at a size matching its slot, and no images on short or brief stories. If the layout editor fails, the ranked list is used instead.
2. **Reports.** The desk writer (`story_report` task) writes an original Sinhala report for every lead, feature and standard block (at most about ten), two at a time, close to the block's length. It works only from the stories' public evidence: each outlet's headline, its summary, and the stored article text where the outlet's rights allow storing it (up to six outlets, 2,500 characters each).
   - The reviewer (`story_report_review` task) checks every report against the evidence. A report needs a score of 80 or more and no blocker or major issue.
   - A failing report gets one rewrite from the reviewer's issues. If it still fails, it is not printed and the block shows the story summary instead.
   - Reports are stored in `story_reports` and reused while the story is unchanged, meaning the same update time and number of outlets.
3. **Illustrations.** For each image place, the Codex CLI's built-in image tool draws an 18th-century woodcut-style drawing from the report's `illustration_prompt`. It needs no API key, just the ChatGPT login.
   - Prompts forbid real people's likenesses, violence and injuries, and stories about crime victims, casualties or named individuals are never illustrated.
   - Images are converted to 960-pixel-wide greyscale JPEGs (quality 72), stored under media key `illustrations/<report>.jpg` and served at `/api/v1/illustrations/<file>`. Codex's own copies are deleted.
   - Every illustration is captioned "චිත්‍රය: AI නිර්මාණයකි — සැබෑ ඡායාරූපයක් නොවේ" (an AI illustration, not a real photograph). A story keeps its illustration in later editions.
   - `SNAP_CODEX_IMAGE_MODEL` chooses the Codex model; the default is the first allowed model.
4. **Publish.** The edition is saved in `editions` and served in `GET /api/v1/front-page` as `blocks` (size, story, report, image). The lead, top and more lists are derived from those blocks. Editions older than 12 hours are ignored, and the page falls back to the automatic ranking. `GET /api/v1/reports/{event|article}/{id}` returns a story's longest report and its illustration for the story page.

## Editorials

Editorials (`editorials` table) are the paper's own opinion and are labelled as such on the site:

- **Fields:** a title, an optional standfirst (summary under the title), a byline and a plain-text body. Separate paragraphs with a blank line.
- **Linked story:** optionally, the event the editorial is about.
- **Statuses:** `draft`, `published` and `archived`. Publishing sets `published_at` once; unpublishing or archiving keeps it.
- **Deleting:** only drafts that were never published can be deleted.

Public endpoints: `GET /api/v1/editorials` returns the latest 50 published editorials and `GET /api/v1/editorials/{id}` returns one. Admin endpoints live under `/api/admin/editorials`.

## Admin API

| Method and path | Purpose |
| --- | --- |
| `GET /api/admin/front-page` | Current placements, ranked candidates, sources, and a preview of the page |
| `PUT /api/admin/front-page` | Replace all placements (`{"placements":[{"slot","kind","id","note","expires_at"}]}`) |
| `PATCH /api/admin/front-page/sources/{id}` | `{"front_page": true\|false}` |
| `GET`/`POST /api/admin/editorials` | List, create |
| `PATCH`/`DELETE /api/admin/editorials/{id}` | Edit, delete draft |
| `POST /api/admin/editorials/{id}/{publish\|unpublish\|archive}` | Change status |

All admin routes require the `chief_editor` or `administrator` role and the `X-SNAP-CSRF: 1` header on writes.
