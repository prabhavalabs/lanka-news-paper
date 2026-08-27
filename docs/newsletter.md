# Autonomous morning newsletter

The production worker creates and delivers one Sinhala morning briefing every
day at 08:00 in `Asia/Colombo`. No administrator or interactive agent needs to
be online at send time.

## Editorial contract

The autonomous workflow must follow these instructions:

1. Use the exact 24-hour window ending at the scheduled send time.
2. Select only published articles from active sources whose rights profile
   permits public use. Exclude held categories and expired or
   internal-verification-only material.
3. Group reports about the same event. Rank breaking coverage first, followed
   by stories corroborated by more independent sources, then by recency.
4. Reuse verified pipeline summaries. Write clear, neutral Sinhala, preserve
   uncertainty and attribution, and never invent names, numbers, quotations,
   causality, or conclusions that are absent from the source analysis.
5. Present up to five essential lead stories, followed by category sections.
   Every story must link to its event or article page and state whether it has
   multi-source or single-source coverage.
6. Render responsive HTML and plain text. Prefer `Noto Sans Sinhala`, with
   `Nirmala UI` and `Iskoola Pota` fallbacks, comfortable line height, and
   readable mobile sizing.
7. Send an individual message only to active, consented recipients. Include
   `List-Unsubscribe`, one-click unsubscribe, and a visible unsubscribe link.
8. Create at most one edition per local date and one idempotent delivery per
   edition and recipient. A retry or worker restart must not duplicate a
   successful delivery.

## Administrator responsibilities

Use **Mailing list** in the admin application to add consented recipients,
pause delivery, update an address, or remove a recipient. An unsubscribed
address cannot be silently reactivated; remove and re-add it only after renewed
consent.

The environment controls whether delivery runs and where it is sent from. The
database list controls who receives it. `SNAP_NEWSLETTER_RECIPIENT` is only a
one-time bootstrap import and is not the ongoing source of truth.
