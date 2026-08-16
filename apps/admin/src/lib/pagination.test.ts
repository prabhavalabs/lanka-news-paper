import assert from 'node:assert/strict'
import test from 'node:test'

import { getPaginationPages } from './pagination.ts'

test('keeps nearby pages and both boundaries', () => {
  assert.deepEqual(getPaginationPages(1, 4), [1, 2, 4])
  assert.deepEqual(getPaginationPages(15, 30), [1, 14, 15, 16, 30])
  assert.deepEqual(getPaginationPages(30, 30), [1, 29, 30])
})
