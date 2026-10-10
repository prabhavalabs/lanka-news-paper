import assert from 'node:assert/strict'
import test from 'node:test'
import { facebookAuthorizeURL, hashtagError, parseSocialHashtags, parseSocialSubject, runeCount, runeLimitError } from './social-publishing.ts'
const id = '12345678-1234-1234-1234-123456789abc'
test('extracts event and article IDs from public links and raw IDs', () => {
  assert.deepEqual(parseSocialSubject(` https://lankanewspaper.prabhavalabs.com/e/${id}?ref=social `, 'article'), { event_id: id })
  assert.deepEqual(parseSocialSubject(`/a/${id}`, 'event'), { article_id: id })
  assert.deepEqual(parseSocialSubject(id, 'article'), { article_id: id })
  for (const value of [`https://evil.example/e/${id}`, `https://lankanewspaper.prabhavalabs.com.evil/e/${id}`, '/e/invalid', `/e/${id}/more`, '']) assert.equal(parseSocialSubject(value, 'event'), null)
})
test('counts Unicode code points like Go rune limits, including marks and emoji', () => {
  assert.equal(runeCount('සිංහල😀'), 6)
  assert.equal(runeLimitError('😀'.repeat(90), 90), null)
  assert.ok(runeLimitError('😀'.repeat(91), 90))
  assert.ok(runeLimitError('  ', 90))
  assert.equal(runeLimitError(' x ', 1), null)
})
test('validates backend hashtag limits and Facebook redirect origin', () => {
  assert.deepEqual(parseSocialHashtags('#සිංහල #News'), ['සිංහල', 'News'])
  assert.equal(hashtagError(['සිංහල', 'News']), null)
  for (const tags of [['a', 'A'], ['a', 'b', 'c', 'd'], ['a-b'], ['²'], ['x'.repeat(61)]]) assert.ok(hashtagError(tags))
  assert.ok(facebookAuthorizeURL('https://www.facebook.com/v26.0/dialog/oauth'))
  for (const url of ['http://www.facebook.com/', 'https://www.facebook.com.evil/', 'javascript:alert(1)', 'https://evil@www.facebook.com/']) assert.equal(facebookAuthorizeURL(url), null)
})
