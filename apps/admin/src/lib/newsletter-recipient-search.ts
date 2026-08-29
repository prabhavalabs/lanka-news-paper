import type { NewsletterSubscriber } from '@snap/api-client'

export const MAX_TEST_RECIPIENT_SUGGESTIONS = 20

export function normalizeTestEmail(value: string) {
  return value.trim().toLowerCase()
}

export function isValidTestEmail(value: string) {
  const email = normalizeTestEmail(value)
  return email.length > 0
    && email.length <= 254
    && /^[^\s@]+@[^\s@]+$/u.test(email)
}

export function findNewsletterRecipientByEmail(recipients: NewsletterSubscriber[], value: string) {
  const email = normalizeTestEmail(value)
  if (!email) return undefined
  return recipients.find((recipient) => normalizeTestEmail(recipient.email) === email)
}

export function filterNewsletterRecipientSuggestions(
  recipients: NewsletterSubscriber[],
  query: string,
  limit = MAX_TEST_RECIPIENT_SUGGESTIONS,
) {
  const term = normalizeTestEmail(query)
  const matches = term
    ? recipients.filter((recipient) => (
      recipient.email.toLowerCase().includes(term)
      || recipient.name.toLowerCase().includes(term)
    ))
    : recipients
  return matches.slice(0, Math.max(0, limit))
}

export function testEmailValidationMessage(recipients: NewsletterSubscriber[], value: string) {
  if (!value.trim()) return null
  if (!isValidTestEmail(value)) return 'Enter a complete email address.'
  const recipient = findNewsletterRecipientByEmail(recipients, value)
  if (recipient?.status === 'paused') return 'This recipient is paused. Resume delivery before sending a test.'
  if (recipient?.status === 'unsubscribed') return 'This recipient has unsubscribed and cannot receive a test email.'
  return null
}
