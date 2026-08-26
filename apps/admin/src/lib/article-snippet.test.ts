import assert from 'node:assert/strict'
import test from 'node:test'

import { compactArticleSnippet } from './article-snippet.ts'

test('returns an empty snippet when an older API omits the field', () => {
  assert.equal(compactArticleSnippet(undefined), '')
  assert.equal(compactArticleSnippet(null), '')
})

test('turns summary markdown into compact table text', () => {
  assert.equal(
    compactArticleSnippet('## Summary\n\n- Read the [full report](https://example.com).'),
    'Summary Read the full report.',
  )
})
