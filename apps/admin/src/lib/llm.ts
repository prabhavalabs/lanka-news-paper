import type { LlmProvider } from '@snap/api-client'

export function providerLabel(id: string) {
  // Preserve labels for historical OpenRouter call records.
  return id === 'codex_cli' ? 'Codex CLI' : id === 'openrouter' ? 'OpenRouter' : id
}

export function codexStatusDetail(provider: LlmProvider) {
  if (!provider.installed) return 'Not installed: deploy the API image containing Codex CLI.'
  if (provider.status === 'auth_expired' || !provider.authenticated) return 'Authentication expired or missing. On the VPS run: docker compose --profile tools run --rm codex-login'
  if (provider.status === 'quota_exhausted') return 'Quota exhausted. The pipeline is paused until the configured cooldown expires; requests can retry afterward.'
  return provider.status_detail
}
