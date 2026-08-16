# UI Design Guidelines — Sinhala News Aggregation Platform (SNAP)

**Audience:** This document is a design specification intended to be handed to an AI coding agent (Claude Code) or a human frontend developer. Follow it exactly. Where a rule conflicts with a shadcn/ui default, this document wins.

**Design direction in one sentence:** A digital broadsheet — the restraint, typography, and rhythm of a printed Sinhala newspaper, rendered with modern web technology. No decoration that ink could not print.

---

## 1. Design Principles

1. **Ink on paper.** The entire interface uses one "ink" color on one "paper" color, plus a limited set of grays. No brand blues, no gradients, no colored accents except functional states (links visited, errors).
2. **Typography is the interface.** Hierarchy comes from type size, weight, and spacing — never from colored boxes, shadows, or pills.
3. **Rules, not cards.** Content is separated by hairline rules (borders) the way newspaper columns are, not by floating cards with shadows and rounded corners.
4. **Density with dignity.** Newspapers are dense but never cramped. Generous line-height for Sinhala script, tight but consistent spacing between items.
5. **Nothing moves unless the reader moves it.** No autoplaying carousels, no attention-grabbing animation. Transitions are instant or near-instant (≤150 ms, opacity/transform only).
6. **The source is sacred.** Publisher attribution is part of the content, styled like a newspaper byline — always visible, never hidden behind hover.

---

## 2. Color Tokens

Monochrome newsprint palette. Define as CSS variables consumed by shadcn's theme system.

### 2.1 Light theme ("Day edition" — default)

| Token | Value | Usage |
|---|---|---|
| `--paper` | `#F7F5F0` | Page background (warm newsprint, not pure white) |
| `--paper-raised` | `#FDFCF9` | Elevated surfaces: dialogs, popovers |
| `--ink` | `#141414` | Primary text, headlines |
| `--ink-secondary` | `#4A4A4A` | Body summaries, secondary text |
| `--ink-tertiary` | `#767470` | Metadata: timestamps, bylines, captions |
| `--rule` | `#D8D5CD` | Hairline dividers, borders |
| `--rule-strong` | `#141414` | Structural rules: masthead border, section headers |
| `--tint` | `#EEEBE3` | Hover background, selected states, kickers |
| `--link-visited` | `#5A5650` | Visited article links (subtle, like read newsprint) |
| `--error` | `#8B2500` | Errors, "removed/unavailable" states only |

### 2.2 Dark theme ("Night edition")

Not inverted-white. Dark charcoal paper, off-white ink — like reading under a lamp.

| Token | Value |
|---|---|
| `--paper` | `#191917` |
| `--paper-raised` | `#211F1D` |
| `--ink` | `#E8E6E1` |
| `--ink-secondary` | `#B3B0AA` |
| `--ink-tertiary` | `#807D77` |
| `--rule` | `#33312E` |
| `--rule-strong` | `#E8E6E1` |
| `--tint` | `#232220` |
| `--link-visited` | `#8F8B84` |
| `--error` | `#D9744F` |

### 2.3 Rules

- Both themes must pass WCAG 2.2 AA: `--ink` on `--paper` ≥ 12:1, `--ink-tertiary` on `--paper` ≥ 4.5:1. Verify at build time.
- Never introduce a new color without updating this document. Category labels, source-type labels, buttons — all monochrome.
- Source-type distinction (state media / private / government / international) is conveyed with **text labels and small-caps styling**, never color coding.

---

## 3. Typography

### 3.1 Font stack

Primary script is Sinhala. All fonts are open-source (Google Fonts / SIL OFL), self-hosted (no external font CDN requests; ship WOFF2 in the repo).

| Role | Font | Fallback chain |
|---|---|---|
| Headlines (Sinhala) | **Abhaya Libre** (SemiBold, Bold) | `"Abhaya Libre", "Noto Serif Sinhala", serif` |
| Body (Sinhala) | **Noto Serif Sinhala** (Regular, Medium) | `"Noto Serif Sinhala", serif` |
| UI / metadata / admin | **Noto Sans Sinhala** | `"Noto Sans Sinhala", system-ui, sans-serif` |
| Latin headlines (English fragments in headlines) | Abhaya Libre covers Latin | — |
| Numerals, timestamps, code | Tabular numerals via `font-variant-numeric: tabular-nums` | — |

Rationale: Abhaya Libre is the digitization of FM Abhaya, the most widely used Sinhala typeface in Sri Lankan print newspapers — it *is* the nostalgic newspaper voice. Noto Serif Sinhala has complete glyph coverage and better small-size legibility for body text.

### 3.2 Type scale

Base 17px (Sinhala script needs slightly larger base than Latin). Scale ratio ~1.25.

| Token | Size / line-height | Usage |
|---|---|---|
| `--text-masthead` | clamp(2.5rem, 6vw, 4.25rem) / 1.1 | Site masthead only |
| `--text-h1` | 2.125rem / 1.25 | Lead story headline, page titles |
| `--text-h2` | 1.625rem / 1.3 | Section headers, secondary story headlines |
| `--text-h3` | 1.3125rem / 1.35 | Standard card headlines |
| `--text-body` | 1.0625rem (17px) / 1.75 | Article summaries, reading text |
| `--text-meta` | 0.8125rem / 1.5 | Bylines, timestamps, labels |
| `--text-caption` | 0.75rem / 1.5 | Captions, footnotes, legal |

### 3.3 Sinhala-specific rules

- Line-height for body Sinhala: **minimum 1.7**. Sinhala has tall ascenders/descenders (කොම්බුව, පාපිල්ල); tight leading clips them.
- Never letter-space Sinhala text (`letter-spacing: 0` on all Sinhala). Small-caps/tracking effects apply to Latin metadata only.
- Do not justify Sinhala body text on widths under 40ch — rivers form badly. Use `text-align: left` (Sinhala is LTR) on mobile; justification permitted on desktop multi-column layouts with `text-wrap: pretty` where supported.
- Test rendering with zero-width joiners and touching letters (e.g., ක්‍ෂ, න්‍ද). The chosen fonts handle these; do not add CSS `font-feature-settings` that break conjuncts.
- `lang="si"` on `<html>` for public pages; `lang` switches on English admin pages.

### 3.4 Newspaper typographic devices

Use these; they carry the nostalgia:

- **Kickers:** small-caps, letter-spaced (Latin) or plain bold small text (Sinhala) category line above headlines: `දේශපාලන` in `--text-meta`, `--ink-tertiary`.
- **Datelines:** `කොළඹ — ` prefix pattern in summaries where location data exists.
- **Bylines:** `මූලාශ්‍රය: ලංකාදීප` — source attribution styled as byline, `--text-meta`, always below headline, before any summary.
- **Drop caps:** permitted on the daily brief page only (first paragraph), via `initial-letter` with fallback; not on cards.
- **Double rule** (thick over thin, `3px` + `1px` with `3px` gap) under the masthead and above the footer — the classic broadsheet signature.
- **Section dividers:** single hairline `--rule` between all list items; `--rule-strong` 2px above section headers.

---

## 4. Layout

### 4.1 Grid

- Max content width: **1280px**, centered, `24px` gutters mobile, `48px` desktop.
- Desktop (≥1024px): 12-column grid. Front page composes stories across it (see 4.3).
- Tablet (768–1023px): 8-column.
- Mobile (<768px): single column, full-width cards with hairline separators.

### 4.2 Masthead (site header)

Centered, stacked, like a broadsheet nameplate:

```
        ─────────────────────────────────────
                  [SITE NAME IN SINHALA]        ← Abhaya Libre Bold, --text-masthead
        2026 අගෝස්තු 15 සිකුරාදා | කොළඹ         ← date line, --text-meta, centered
        ═════════════════════════════════════   ← double rule
        පුවත් | දේශපාලන | ආර්ථික | ක්‍රීඩා | ලෝක ...  ← section nav, horizontal, scrollable on mobile
        ─────────────────────────────────────   ← hairline
```

- Section nav: `--text-meta` size, `--ink`, current section bold with 2px underline offset. Horizontally scrollable on mobile with no visible scrollbar, edge-fade mask.
- Search: icon-only button at masthead right edge; opens a full-width command palette (shadcn `Command`) — newspaper reading is uninterrupted.
- Sticky behavior: masthead scrolls away; a slim 48px condensed bar (site name small + section nav) pins after 300px scroll.

### 4.3 Front page composition

Newspaper hierarchy, not a uniform feed:

- **Lead story** (top cluster or top item): spans 8 of 12 columns, `--text-h1` headline, 2–3 line summary if rights permit, source byline, related-coverage count ("තවත් වාර්තා 4ක්" — 4 more reports).
- **Secondary column** (right 4 columns): 3–4 headline-only items, hairline-separated.
- **Below the fold:** section blocks. Each block: section header (small caps / bold + `--rule-strong` top border), then 3–6 items in 2–3 text columns using CSS `columns` where content is headline-only.
- **No images in MVP** (rights profiles forbid); layout must look complete and intentional without them. When licensed thumbnails arrive later, they are small, inline-start, grayscale-filtered (`filter: grayscale(1)`) to preserve monotone until hover/focus removes the filter.

### 4.4 Article card anatomy

Every card, strictly this order:

1. Kicker (category, optional)
2. Headline (link to detail page; publisher's original headline, never truncated with CSS ellipsis — wrap fully)
3. Byline: source name + source-type label in small text (`රාජ්‍ය මාධ්‍ය` state media, etc.)
4. Summary/excerpt — only if rights profile permits
5. Meta row: relative time ("මිනිත්තු 20කට පෙර") + absolute time in `title` attr + multi-source badge if clustered
6. Original-link action: `මුල් ලිපිය කියවන්න ↗` — text link with external-arrow, `--ink`, underlined on hover. Must be visible without hover.

Cards have **no border-radius, no shadow, no background** — separated by hairlines. Hover state: `--tint` background, no movement.

### 4.5 Pagination — "pages, not scroll"

- Public lists use **numbered pagination**, not infinite scroll. Newspaper metaphor: "පිටුව 1 2 3 … ඊළඟ →".
- Page changes scroll to top instantly.
- Keyboard: `←`/`→` navigate pages when list has focus.

### 4.6 Event/cluster page ("coverage comparison")

- Cluster title as `--text-h1` (platform-generated titles must be visually distinct from publisher headlines: prefixed label "සිදුවීම" (event) in kicker position).
- Sources presented as a vertical timeline of cards, each with full byline and timestamp, chronological order. Never merge or paraphrase one source's report under another's name.
- Balance/coverage summary (when AI analysis exists): rendered in a bordered box (`--rule` 1px border, `--tint` bg) explicitly labeled "ස්වයංක්‍රීය සාරාංශය" (automated summary) with model attribution in caption text. Newspaper equivalent: the editor's note box.

---

## 5. Components (shadcn/ui mapping)

Base library: shadcn/ui. Global overrides:

- `--radius: 0rem` — **zero border radius everywhere.** Newspapers have no rounded corners.
- Remove all `box-shadow` except: `Dialog`/`Popover` get a 1px `--rule-strong` border + `4px 4px 0` hard offset shadow in `--ink` at 8% opacity (letterpress feel), never blurred soft shadows.
- All component color tokens remap to the palette in §2.

| Need | shadcn component | Customization |
|---|---|---|
| Section nav | custom + `NavigationMenu` primitives | underline current, no pills |
| Search | `Command` (⌘K palette) | full-screen on mobile; Sinhala IME-safe input |
| Filters (source, type, date) | `Select`, `Popover` + `Checkbox` list | square checkboxes, hairline borders |
| Multi-source badge | `Badge` | outline variant only: 1px `--ink` border, transparent bg, small-caps |
| Date picker | `Calendar` | monochrome, square day cells |
| Dialogs (report problem) | `Dialog` | letterpress shadow, serif title |
| Toasts | `Sonner` | bottom-center, paper-raised bg, 1px border, no icon color |
| Admin tables | `Table` + TanStack Table | hairline row separators, tabular-nums, sticky header |
| Admin forms | `Form` + `Input`, `Textarea` | 1px bottom-border style inputs (ledger feel) in admin; full 1px box border in dialogs |
| Skeletons | `Skeleton` | `--tint` shimmer-free (static block, opacity pulse only) |
| Breaking banner | custom | full-width bar under masthead: `--ink` bg, `--paper` text, "විශේෂ පුවත්" label, dismissible, no animation beyond fade-in |

### 5.1 Admin data-table contract

- Every user-facing table uses API-backed pagination, including small result sets.
- Page, page size, search, and filter state live in the browser URL so views are linkable and survive refreshes.
- Search, filtering, counts, and row slicing run on the server; React must not paginate a fully fetched collection.
- List endpoints return the shared `{ items, pagination }` response and validate query parameters at the HTTP boundary.
- Database queries use a stable ordering before `LIMIT` and `OFFSET`.
- Admin tables reuse the shared URL query hook and shadcn table toolbar/pagination controls.

**Buttons:** two variants only.
- Primary: `--ink` background, `--paper` text, square, `--text-meta` size, medium weight.
- Ghost: transparent, 1px `--ink` border.
- No secondary colors, no destructive-red fills (destructive confirm dialogs use ghost button + explicit typed confirmation for dangerous admin actions).

---

## 6. Motion

- Allowed: opacity fades ≤150 ms, transform slides ≤200 ms for drawers/dialogs. Easing `ease-out`.
- Forbidden: parallax, auto-carousels, skeleton shimmer sweeps, hover scale/lift, scroll-triggered animation.
- Respect `prefers-reduced-motion: reduce` — all transitions drop to 0 ms.
- Live updates (new breaking item while reading): never reflow the page under the reader. Show a static "නව පුවත් — නැවුම් කරන්න" (new items — refresh) pill at top; reader clicks to load.

---

## 7. Responsive Behavior

| Breakpoint | Layout |
|---|---|
| <480px | Single column; masthead compact (site name + date only); nav scrollable row |
| 480–767px | Single column; lead story keeps larger headline scale |
| 768–1023px | 8-col; lead story 8-col, secondary items 2-up grid; section blocks 2 text columns |
| 1024–1279px | 12-col full front-page composition; 3 text columns in section blocks |
| ≥1280px | Fixed 1280px; identical to previous, more whitespace |

- Text must remain readable at 200% browser zoom without horizontal scroll (WCAG 2.2 AA / SRS NFR-ACC-003).
- Touch targets ≥44×44px on all interactive elements.
- Test Sinhala headline wrapping at every breakpoint — long compound words must wrap (`overflow-wrap: break-word` as safety), never overflow.

## 8. Accessibility (binding, from SRS NFR-ACC)

- WCAG 2.2 AA target. All functionality keyboard-operable; visible focus ring: 2px solid `--ink`, 2px offset (square, not rounded).
- Landmarks: `<header>`, `<nav>`, `<main>`, `<footer>`; skip-link as first focusable element.
- Every card is one `<article>` with `<h2>/<h3>` headline hierarchy; lists are `<ol>` (chronology matters).
- External links: `rel="noopener"`, and screen-reader text "(බාහිර සබැඳිය)" appended.
- Icons never appear without text or `aria-label`. Health/status indicators in admin use text + icon, never color alone.
- Sinhala screen-reader testing: verify NVDA + eSpeak-NG Sinhala and VoiceOver behavior on article lists before launch.

## 9. What This Design Is Not (anti-patterns)

Reject any generated UI that contains:

- Rounded cards floating on gray backgrounds with drop shadows
- Colored category chips / tag pills (blue "Politics", green "Sports"…)
- Hero carousels or image sliders
- Infinite scroll on public reading surfaces
- Hover-only reveals of attribution or actions
- Emoji in UI copy
- Gradient text, glassmorphism, neumorphism
- More than two font families on a page
- Colored favicons/logos of publishers (MVP is text attribution only, per rights profiles)

## 10. Page Inventory (public)

1. **මුල් පිටුව** Front page — composed hierarchy (§4.3)
2. **Section pages** — one per category; same block layout, paginated
3. **Article detail** — headline, byline, full permitted metadata, prominent original-link, cluster cross-links, "report a problem"
4. **Event page** — coverage timeline + comparison (§4.6)
5. **Source directory + source page** — masthead-style source header, ownership/type disclosure block (bordered editor's-note box), latest permitted items
6. **Search results** — same card anatomy, query echoed as page title, filters row
7. **Daily brief** ("උදෑසන සංග්‍රහය") — the 24h digest as a readable page; drop cap permitted; this page doubles as the email content source
8. **Static:** about, privacy, corrections policy, contact/complaint form

Admin portal reuses the same tokens with Noto Sans Sinhala / system UI, denser spacing, and no newspaper decoration — the newsroom back office, not the paper.

---

*End of UI guidelines. Version 1.0 — 2026-08-15. Amend this file, not ad-hoc styles.*
