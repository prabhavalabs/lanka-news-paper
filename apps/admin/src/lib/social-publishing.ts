export const socialTextLimits = { headline_si: 90, summary_si: 280, caption: 5000 } as const

// Go's utf8.RuneCountInString counts Unicode code points, including Sinhala marks.
export function runeCount(value: string) { return Array.from(value).length }
export function runeLimitError(value: string, limit: number) {
  return !value.trim() ? 'Required.' : runeCount(value.trim()) > limit ? `Use at most ${limit} characters.` : null
}

// Public reader origins: the Thulawa domain, plus the previous domain while it still serves.
const publicOrigins = ['https://thulawanews.com', 'https://www.thulawanews.com', 'https://lankanewspaper.prabhavalabs.com']

export function parseSocialSubject(value: string, kind: 'event' | 'article') {
  const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
  const input = value.trim()
  if (uuid.test(input)) return kind === 'event' ? { event_id: input } : { article_id: input }
  try {
    const url = new URL(input, 'https://thulawanews.com')
    if (!publicOrigins.includes(url.origin)) return null
    const match = /^\/(e|a)\/([^/]+)\/?$/.exec(url.pathname)
    if (!match || !match[2] || !uuid.test(match[2])) return null
    return match[1] === 'e' ? { event_id: match[2] } : { article_id: match[2] }
  } catch { return null }
}

export function parseSocialHashtags(value: string) {
  return value.trim() ? value.trim().split(/\s+/).map((tag) => tag.replace(/^#+/, '')) : []
}
export function hashtagError(tags: string[]) {
  if (tags.length > 3) return 'Use at most three hashtags.'
  if (tags.some((tag) => !tag || runeCount(tag) > 60 || !/^[\p{L}\p{Nd}\p{M}_\u200c\u200d]+$/u.test(tag))) return 'Each hashtag must use letters, digits or underscores and at most 60 characters.'
  if (new Set(tags.map((tag) => tag.toLowerCase())).size !== tags.length) return 'Remove duplicate hashtags.'
  return null
}
export function facebookAuthorizeURL(value: string) {
  try {
    const url = new URL(value)
    return url.protocol === 'https:' && url.hostname === 'www.facebook.com' && !url.port && !url.username && !url.password ? url.href : null
  } catch { return null }
}
export function safeSocialLink(value: string) {
  try { const url = new URL(value); return url.protocol === 'https:' ? url.href : undefined } catch { return undefined }
}
