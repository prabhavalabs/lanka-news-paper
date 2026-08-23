import assert from 'node:assert/strict'
import test from 'node:test'

import { formatCompactRelativeTime, formatDateTime } from './date-time.ts'

const now = new Date('2026-08-22T12:00:00Z').getTime()

test('formats recent past timestamps as compact relative time', () => {
  assert.equal(formatCompactRelativeTime('2026-08-22T11:59:45Z', now), 'now')
  assert.equal(formatCompactRelativeTime('2026-08-22T11:58:30Z', now), '1m ago')
  assert.equal(formatCompactRelativeTime('2026-08-22T09:00:00Z', now), '3h ago')
  assert.equal(formatCompactRelativeTime('2026-08-19T12:00:00Z', now), '3d ago')
  assert.equal(formatCompactRelativeTime('2026-08-01T12:00:00Z', now), '3w ago')
  assert.equal(formatCompactRelativeTime('2026-06-22T12:00:00Z', now), '2mo ago')
  assert.equal(formatCompactRelativeTime('2024-08-22T12:00:00Z', now), '2y ago')
})

test('formats future timestamps without producing negative values', () => {
  assert.equal(formatCompactRelativeTime('2026-08-22T12:05:00Z', now), 'in 5m')
})

test('handles missing and invalid timestamps', () => {
  assert.equal(formatCompactRelativeTime(null, now), 'Never')
  assert.equal(formatCompactRelativeTime('not-a-date', now), 'Never')
  assert.equal(formatDateTime(null), '—')
  assert.equal(formatDateTime('not-a-date'), '—')
})
