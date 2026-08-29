import assert from 'node:assert/strict'
import test from 'node:test'

import type { NewsletterSubscriber } from '@snap/api-client'

import {
  filterNewsletterRecipientSuggestions,
  findNewsletterRecipientByEmail,
  isValidTestEmail,
  normalizeTestEmail,
  testEmailValidationMessage,
} from './newsletter-recipient-search.ts'

function recipient(email: string, name: string, status: NewsletterSubscriber['status'] = 'active'): NewsletterSubscriber {
  return {
    id: email,
    email,
    name,
    status,
    consent_source: 'admin_confirmed',
    consented_at: '2026-08-29T00:00:00Z',
    created_at: '2026-08-29T00:00:00Z',
    updated_at: '2026-08-29T00:00:00Z',
  }
}

const recipients = [
  recipient('reader@example.com', 'Nimal Perera'),
  recipient('editor@example.com', 'Kumari Silva', 'paused'),
  recipient('former@example.com', 'Amal Fernando', 'unsubscribed'),
]

test('normalizes and validates one-time test email addresses', () => {
  assert.equal(normalizeTestEmail('  Reader@Example.com '), 'reader@example.com')
  assert.equal(isValidTestEmail('reader@example.com'), true)
  assert.equal(isValidTestEmail('reader.example.com'), false)
  assert.equal(isValidTestEmail('reader @example.com'), false)
})

test('filters recipient suggestions by name or email without case sensitivity', () => {
  assert.deepEqual(filterNewsletterRecipientSuggestions(recipients, 'NIMAL'), [recipients[0]])
  assert.deepEqual(filterNewsletterRecipientSuggestions(recipients, 'editor@'), [recipients[1]])
})

test('caps recipient suggestions before rendering', () => {
  assert.deepEqual(filterNewsletterRecipientSuggestions(recipients, '', 2), recipients.slice(0, 2))
})

test('finds an existing recipient using a normalized email address', () => {
  assert.equal(findNewsletterRecipientByEmail(recipients, ' READER@EXAMPLE.COM ')?.name, 'Nimal Perera')
})

test('blocks tests to paused and unsubscribed mailing-list recipients', () => {
  assert.equal(testEmailValidationMessage(recipients, 'new@example.com'), null)
  assert.equal(testEmailValidationMessage(recipients, 'invalid'), 'Enter a complete email address.')
  assert.equal(testEmailValidationMessage(recipients, 'editor@example.com'), 'This recipient is paused. Resume delivery before sending a test.')
  assert.equal(testEmailValidationMessage(recipients, 'former@example.com'), 'This recipient has unsubscribed and cannot receive a test email.')
})
