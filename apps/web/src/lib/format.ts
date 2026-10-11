import type { SourceType } from '@snap/api-client'

const TZ = 'Asia/Colombo'

export const sourceTypeLabels: Record<SourceType | string, string> = {
  private_media: 'පෞද්ගලික මාධ්‍ය',
  state_owned: 'රාජ්‍ය මාධ්‍ය',
  government: 'නිල නිවේදනය',
  independent: 'ස්වාධීන මාධ්‍ය',
  international: 'ජාත්‍යන්තර මාධ්‍ය',
  other: 'වෙනත්',
}

export function sourceTypeLabel(type: string) {
  return sourceTypeLabels[type] ?? type
}

function toDate(value: string | Date) {
  const date = typeof value === 'string' ? new Date(value) : value
  return Number.isNaN(date.getTime()) ? null : date
}

const SI_MONTHS = ['ජනවාරි', 'පෙබරවාරි', 'මාර්තු', 'අප්‍රේල්', 'මැයි', 'ජූනි', 'ජූලි', 'අගෝස්තු', 'සැප්තැම්බර්', 'ඔක්තෝබර්', 'නොවැම්බර්', 'දෙසැම්බර්']
const SI_WEEKDAYS = ['ඉරිදා', 'සඳුදා', 'අඟහරුවාදා', 'බදාදා', 'බ්‍රහස්පතින්දා', 'සිකුරාදා', 'සෙනසුරාදා']

/**
 * Browsers often ship without Sinhala date data (Intl falls back to English),
 * so Sinhala dates are composed from Colombo-time parts here.
 */
function colomboParts(date: Date) {
  const parts = new Intl.DateTimeFormat('en-GB', {
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
    weekday: 'short',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
    timeZone: TZ,
  }).formatToParts(date)
  const get = (type: string) => parts.find((part) => part.type === type)?.value ?? ''
  const weekdayIndex = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].indexOf(get('weekday'))
  return {
    year: get('year'),
    month: SI_MONTHS[Number(get('month')) - 1] ?? '',
    day: String(Number(get('day'))),
    weekday: SI_WEEKDAYS[weekdayIndex] ?? '',
    hour: Number(get('hour')),
    minute: get('minute'),
  }
}

function sinhalaClock(hour: number, minute: string) {
  const period = hour < 12 ? 'පෙ.ව.' : 'ප.ව.'
  const h12 = hour % 12 === 0 ? 12 : hour % 12
  return `${period} ${h12}.${minute}`
}

/** "2026 ඔක්තෝබර් 11 ඉරිදා" */
export function sinhalaLongDate(value: string | Date = new Date()) {
  const date = toDate(value)
  if (!date) return ''
  const p = colomboParts(date)
  return `${p.year} ${p.month} ${p.day} ${p.weekday}`
}

/** "Sunday, 11 October 2026" */
export function gregorianLongDate(value: string | Date = new Date()) {
  const date = toDate(value)
  if (!date) return ''
  return new Intl.DateTimeFormat('en-GB', {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    timeZone: TZ,
  }).format(date)
}

/** Edition dates come as YYYY-MM-DD; anchor them at Colombo noon so the day never shifts. */
export function editionDate(value: string | undefined) {
  if (value && /^\d{4}-\d{2}-\d{2}$/.test(value)) return new Date(`${value}T12:00:00+05:30`)
  return new Date()
}

export function sinhalaDateTime(value: string) {
  const date = toDate(value)
  if (!date) return ''
  const p = colomboParts(date)
  return `${p.month} ${p.day}, ${sinhalaClock(p.hour, p.minute)}`
}

export function sinhalaDate(value: string) {
  const date = toDate(value)
  if (!date) return ''
  const p = colomboParts(date)
  return `${p.year} ${p.month} ${p.day}`
}

/** "මිනිත්තු 20කට පෙර", "පැය 3කට පෙර"; older than a day falls back to the date. */
export function relativeTime(value: string, now = Date.now()) {
  const date = toDate(value)
  if (!date) return ''
  const minutes = Math.round((now - date.getTime()) / 60_000)
  if (minutes < 1) return 'මේ දැන්'
  if (minutes < 60) return `මිනිත්තු ${minutes}කට පෙර`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `පැය ${hours}කට පෙර`
  return sinhalaDateTime(value)
}

export function absoluteTitle(value: string) {
  const date = toDate(value)
  if (!date) return undefined
  const p = colomboParts(date)
  return `${p.year} ${p.month} ${p.day} ${p.weekday}, ${sinhalaClock(p.hour, p.minute)}`
}

const latinOnly = /^[\p{Script=Latin}\p{N}\p{P}\p{Zs}\p{S}]+$/u

export function isLatin(text: string) {
  return latinOnly.test(text)
}

export function outletsReported(count: number) {
  return count > 1 ? `මාධ්‍ය ආයතන ${count}ක් වාර්තා කළ පුවතකි` : 'එක් මාධ්‍ය ආයතනයක වාර්තාවකි'
}

export function paragraphs(text: string | null | undefined) {
  return (text ?? '')
    .split(/\n\s*\n/)
    .map((part) => part.replace(/\s*\n\s*/g, ' ').trim())
    .filter(Boolean)
}

/** First paragraph, shortened at a sentence boundary when longer than `max` characters. */
export function lede(text: string | null | undefined, max = 320) {
  const first = paragraphs(text)[0] ?? ''
  if (first.length <= max) return first
  const cut = first.slice(0, max)
  const stop = cut.lastIndexOf('. ')
  return stop > max * 0.5 ? cut.slice(0, stop + 1) : `${cut.slice(0, cut.lastIndexOf(' '))}…`
}

/**
 * Splits off the first Sinhala syllable (consonant + touching conjuncts + vowel signs)
 * so a drop cap never breaks a ZWJ conjunct or orphans a vowel sign.
 */
export function dropCapSplit(text: string): [string, string] {
  const sinhala = /^[අ-ෆ](?:්‍[ක-ෆ])*[ඁ-ඃ්ා-ෟෲෳ]*/u.exec(text)
  if (sinhala?.[0]) return [sinhala[0], text.slice(sinhala[0].length)]
  const segmenter = new Intl.Segmenter('si', { granularity: 'grapheme' })
  const first = segmenter.segment(text)[Symbol.iterator]().next().value?.segment ?? ''
  return [first, text.slice(first.length)]
}

export function isImageUrl(value: string | null | undefined): value is string {
  return Boolean(value && /^(https?:\/\/|\/)[^\s]+$/i.test(value))
}
