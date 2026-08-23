import {
  createClient,
  type AdminCollectionConfig,
  type AdminCollectionProfile,
  type AdminComplianceReview,
  type AdminEndpoint,
} from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bot, ChevronDown, FileText, ShieldCheck } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

const client = createClient()
const textareaClass = 'min-h-24 w-full resize-y rounded-2xl border border-input bg-transparent px-3 py-2 text-sm shadow-xs outline-none transition-[color,box-shadow] placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50'

function splitLines(value: string) {
  return value.split('\n').map((item) => item.trim()).filter(Boolean)
}

function lines(values?: string[]) {
  return (values ?? []).join('\n')
}

function label(value: string) {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (character) => character.toUpperCase())
}

function defaultConfig(endpoint: AdminEndpoint): AdminCollectionConfig {
  let host = ''
  try {
    host = new URL(endpoint.url).hostname
  } catch {
    host = ''
  }
  return {
    discovery_urls: [endpoint.url],
    allowed_hosts: host ? [host] : [],
    article_url_patterns: [],
    link_selector: '',
    title_selector: '',
    published_selector: '',
    author_selector: '',
    content_selector: '',
    exclude_selectors: [],
    pagination_mode: 'none',
    next_page_selector: '',
    page_parameter: '',
    user_agent: 'SNAPBot/1.0',
    min_content_characters: 200,
    minimum_sinhala_ratio: 0,
  }
}

function PermissionToggle({
  checked,
  disabled,
  title,
  description,
  onChange,
}: {
  checked: boolean
  disabled?: boolean
  title: string
  description: string
  onChange: (checked: boolean) => void
}) {
  return (
    <label className={cn('flex items-start gap-3 rounded-2xl border p-3', disabled && 'opacity-50')}>
      <input
        className="mt-0.5 size-4 accent-primary"
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span className="min-w-0">
        <span className="block text-sm font-medium">{title}</span>
        <span className="mt-0.5 block text-xs leading-relaxed text-muted-foreground">{description}</span>
      </span>
    </label>
  )
}

function ComplianceEditor({
  review,
  pending,
  onSave,
}: {
  review: AdminComplianceReview
  pending: boolean
  onSave: (review: AdminComplianceReview) => void
}) {
  const [draft, setDraft] = useState(review)

  useEffect(() => setDraft(review), [review])
  const denied = draft.status === 'denied'
  const setPermission = (key: keyof AdminComplianceReview, value: boolean) => {
    setDraft((current) => ({ ...current, [key]: value }))
  }

  return (
    <Card className="min-w-0 shadow-sm">
      <CardHeader className="gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <CardTitle className="flex items-center gap-2 text-base"><ShieldCheck className="size-4" />Compliance gate</CardTitle>
          <p className="mt-1 text-sm text-muted-foreground">The active legal review controls collection, storage, and downstream processing.</p>
        </div>
        <Badge variant={draft.status === 'approved' ? 'secondary' : draft.status === 'denied' ? 'destructive' : 'outline'}>
          {label(draft.status)} · v{draft.version}
        </Badge>
      </CardHeader>
      <CardContent>
        <form
          className="grid min-w-0 gap-5"
          onSubmit={(event) => {
            event.preventDefault()
            onSave(draft)
          }}
        >
          <div className="grid min-w-0 gap-4 md:grid-cols-2">
            <Field>
              <FieldLabel htmlFor="compliance-status">Review status</FieldLabel>
              <Select
                value={draft.status}
                onValueChange={(value) => {
                  if (!value) return
                  setDraft((current) => ({
                    ...current,
                    status: value as AdminComplianceReview['status'],
                    ...(value === 'denied' ? {
                      allow_discovery: false,
                      allow_full_text_storage: false,
                      allow_ai_processing: false,
                      allow_embeddings: false,
                      allow_training: false,
                      allow_public_full_text: false,
                    } : {}),
                  }))
                }}
              >
                <SelectTrigger id="compliance-status" className="w-full"><SelectValue>{(value) => label(String(value))}</SelectValue></SelectTrigger>
                <SelectContent>
                  {['pending', 'approved', 'restricted', 'denied'].map((status) => <SelectItem key={status} value={status}>{label(status)}</SelectItem>)}
                </SelectContent>
              </Select>
            </Field>
            <Field>
              <FieldLabel htmlFor="compliance-review-on">Review again</FieldLabel>
              <Input
                id="compliance-review-on"
                type="date"
                value={draft.review_on?.slice(0, 10) ?? ''}
                onChange={(event) => setDraft((current) => ({ ...current, review_on: event.target.value ? new Date(`${event.target.value}T00:00:00Z`).toISOString() : null }))}
              />
            </Field>
            <Field className="md:col-span-2">
              <FieldLabel htmlFor="robots-url">robots.txt URL</FieldLabel>
              <Input id="robots-url" type="url" value={draft.robots_url} onChange={(event) => setDraft((current) => ({ ...current, robots_url: event.target.value }))} placeholder="https://publisher.lk/robots.txt" />
            </Field>
            <Field>
              <FieldLabel htmlFor="robots-checked">Robots checked at</FieldLabel>
              <Input
                id="robots-checked"
                type="datetime-local"
                value={draft.robots_checked_at?.slice(0, 16) ?? ''}
                onChange={(event) => setDraft((current) => ({ ...current, robots_checked_at: event.target.value ? new Date(event.target.value).toISOString() : null }))}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="robots-result">Crawler result</FieldLabel>
              <Select
                value={draft.robots_allowed === null ? 'unknown' : draft.robots_allowed ? 'allowed' : 'blocked'}
                onValueChange={(value) => setDraft((current) => ({ ...current, robots_allowed: value === 'unknown' ? null : value === 'allowed' }))}
              >
                <SelectTrigger id="robots-result" className="w-full"><SelectValue>{(value) => label(String(value))}</SelectValue></SelectTrigger>
                <SelectContent>
                  <SelectItem value="unknown">Not reviewed</SelectItem>
                  <SelectItem value="allowed">Allows configured crawler</SelectItem>
                  <SelectItem value="blocked">Disallows configured crawler</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field className="md:col-span-2">
              <FieldLabel htmlFor="terms-urls">Terms and policy URLs</FieldLabel>
              <textarea id="terms-urls" className={textareaClass} value={lines(draft.terms_urls)} onChange={(event) => setDraft((current) => ({ ...current, terms_urls: splitLines(event.target.value) }))} placeholder="One HTTPS URL per line" />
            </Field>
          </div>

          <div>
            <p className="mb-3 text-sm font-medium">Explicit permissions</p>
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              <PermissionToggle title="Discover metadata" description="Collect headlines, dates, source identity, and canonical links." checked={draft.allow_discovery} disabled={denied} onChange={(value) => setPermission('allow_discovery', value)} />
              <PermissionToggle title="Store full text" description="Persist approved article bodies in restricted storage." checked={draft.allow_full_text_storage} disabled={denied} onChange={(value) => setPermission('allow_full_text_storage', value)} />
              <PermissionToggle title="AI processing" description="Allow approved content to enter automated analysis workflows." checked={draft.allow_ai_processing} disabled={denied} onChange={(value) => setPermission('allow_ai_processing', value)} />
              <PermissionToggle title="Embeddings" description="Generate semantic vectors for similarity and retrieval." checked={draft.allow_embeddings} disabled={denied} onChange={(value) => setPermission('allow_embeddings', value)} />
              <PermissionToggle title="Model training" description="Use approved content for model training or fine-tuning." checked={draft.allow_training} disabled={denied} onChange={(value) => setPermission('allow_training', value)} />
              <PermissionToggle title="Publish full text" description="Expose full text publicly only when the agreement permits it." checked={draft.allow_public_full_text} disabled={denied} onChange={(value) => setPermission('allow_public_full_text', value)} />
            </div>
          </div>

          <Field>
            <FieldLabel htmlFor="compliance-notes">Evidence and reviewer notes</FieldLabel>
            <textarea id="compliance-notes" className={textareaClass} value={draft.notes} onChange={(event) => setDraft((current) => ({ ...current, notes: event.target.value }))} placeholder="Record permission evidence, restrictions, contacts, and review decisions." />
          </Field>
          <div className="flex justify-end"><Button type="submit" disabled={pending}>{pending ? 'Saving…' : 'Save compliance review'}</Button></div>
        </form>
      </CardContent>
    </Card>
  )
}

function CollectionEditor({
  endpoint,
  profile,
  pending,
  onSave,
}: {
  endpoint: AdminEndpoint
  profile?: AdminCollectionProfile
  pending: boolean
  onSave: (profile: AdminCollectionProfile) => void
}) {
  const initial = profile ?? {
    id: '', source_id: endpoint.source_id, endpoint_id: endpoint.id, version: 0,
    discovery_method: endpoint.endpoint_type as AdminCollectionProfile['discovery_method'],
    article_method: 'metadata_only' as const,
    config: defaultConfig(endpoint),
    min_delay_seconds: 5, max_requests_per_run: 25, max_pages: 3,
    request_timeout_seconds: 15, created_by: '', activated_at: null, created_at: '',
  }
  const [draft, setDraft] = useState<AdminCollectionProfile>(initial)
  useEffect(() => setDraft(profile ?? initial), [profile, endpoint.id])
  const updateConfig = <K extends keyof AdminCollectionConfig>(key: K, value: AdminCollectionConfig[K]) => {
    setDraft((current) => ({ ...current, config: { ...current.config, [key]: value } }))
  }

  return (
    <details className="group border-t">
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-5 py-4 sm:px-6">
        <span className="min-w-0">
          <span className="flex items-center gap-2 text-sm font-semibold"><Bot className="size-4" />Collection recipe</span>
          <span className="mt-1 block truncate text-xs text-muted-foreground">{label(draft.discovery_method)} discovery · {label(draft.article_method)} article body · version {draft.version || 'new'}</span>
        </span>
        <ChevronDown className="size-4 shrink-0 transition-transform group-open:rotate-180" />
      </summary>
      <form
        className="grid min-w-0 gap-5 border-t bg-muted/10 p-5 sm:p-6"
        onSubmit={(event) => { event.preventDefault(); onSave(draft) }}
      >
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <Field>
            <FieldLabel htmlFor={`discovery-method-${endpoint.id}`}>Discovery method</FieldLabel>
            <Select value={draft.discovery_method} onValueChange={(value) => value && setDraft((current) => ({ ...current, discovery_method: value as AdminCollectionProfile['discovery_method'] }))}>
              <SelectTrigger id={`discovery-method-${endpoint.id}`} className="w-full"><SelectValue>{(value) => label(String(value))}</SelectValue></SelectTrigger>
              <SelectContent>{['rss', 'atom', 'json_feed', 'rest_api', 'sitemap', 'listing_page', 'webhook', 'youtube'].map((value) => <SelectItem key={value} value={value}>{label(value)}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field>
            <FieldLabel htmlFor={`article-method-${endpoint.id}`}>Article method</FieldLabel>
            <Select value={draft.article_method} onValueChange={(value) => value && setDraft((current) => ({ ...current, article_method: value as AdminCollectionProfile['article_method'] }))}>
              <SelectTrigger id={`article-method-${endpoint.id}`} className="w-full"><SelectValue>{(value) => label(String(value))}</SelectValue></SelectTrigger>
              <SelectContent>{['metadata_only', 'feed_content', 'api_content', 'html_static'].map((value) => <SelectItem key={value} value={value}>{label(value)}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field><FieldLabel htmlFor={`delay-${endpoint.id}`}>Minimum delay (seconds)</FieldLabel><Input id={`delay-${endpoint.id}`} type="number" min={1} max={86400} value={draft.min_delay_seconds} onChange={(event) => setDraft((current) => ({ ...current, min_delay_seconds: Number(event.target.value) }))} /></Field>
          <Field><FieldLabel htmlFor={`timeout-${endpoint.id}`}>Request timeout (seconds)</FieldLabel><Input id={`timeout-${endpoint.id}`} type="number" min={3} max={60} value={draft.request_timeout_seconds} onChange={(event) => setDraft((current) => ({ ...current, request_timeout_seconds: Number(event.target.value) }))} /></Field>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <Field><FieldLabel htmlFor={`discovery-urls-${endpoint.id}`}>Discovery pages</FieldLabel><textarea id={`discovery-urls-${endpoint.id}`} className={textareaClass} value={lines(draft.config.discovery_urls)} onChange={(event) => updateConfig('discovery_urls', splitLines(event.target.value))} placeholder="One HTTPS URL per line" /></Field>
          <Field><FieldLabel htmlFor={`allowed-hosts-${endpoint.id}`}>Allowed source hosts</FieldLabel><textarea id={`allowed-hosts-${endpoint.id}`} className={textareaClass} value={lines(draft.config.allowed_hosts)} onChange={(event) => updateConfig('allowed_hosts', splitLines(event.target.value))} placeholder="publisher.lk" /></Field>
          <Field><FieldLabel htmlFor={`url-patterns-${endpoint.id}`}>Article URL patterns (RE2)</FieldLabel><textarea id={`url-patterns-${endpoint.id}`} className={textareaClass} value={lines(draft.config.article_url_patterns)} onChange={(event) => updateConfig('article_url_patterns', splitLines(event.target.value))} placeholder="^https://publisher\\.lk/news/" /></Field>
          <Field><FieldLabel htmlFor={`exclude-${endpoint.id}`}>Excluded CSS selectors</FieldLabel><textarea id={`exclude-${endpoint.id}`} className={textareaClass} value={lines(draft.config.exclude_selectors)} onChange={(event) => updateConfig('exclude_selectors', splitLines(event.target.value))} placeholder=".advertisement&#10;.related-posts" /></Field>
        </div>

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {([
            ['link_selector', 'Article link selector'],
            ['title_selector', 'Title selector'],
            ['published_selector', 'Publication date selector'],
            ['author_selector', 'Author selector'],
            ['content_selector', 'Main content selector'],
            ['next_page_selector', 'Next-page selector'],
          ] as [keyof AdminCollectionConfig, string][]).map(([key, title]) => (
            <Field key={key}><FieldLabel htmlFor={`${key}-${endpoint.id}`}>{title}</FieldLabel><Input id={`${key}-${endpoint.id}`} value={String(draft.config[key] ?? '')} onChange={(event) => updateConfig(key, event.target.value as never)} placeholder="CSS selector" /></Field>
          ))}
        </div>

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <Field>
            <FieldLabel htmlFor={`pagination-${endpoint.id}`}>Pagination</FieldLabel>
            <Select value={draft.config.pagination_mode} onValueChange={(value) => value && updateConfig('pagination_mode', value as AdminCollectionConfig['pagination_mode'])}>
              <SelectTrigger id={`pagination-${endpoint.id}`} className="w-full"><SelectValue>{(value) => label(String(value))}</SelectValue></SelectTrigger>
              <SelectContent>{['none', 'next_link', 'page_parameter'].map((value) => <SelectItem key={value} value={value}>{label(value)}</SelectItem>)}</SelectContent>
            </Select>
          </Field>
          <Field><FieldLabel htmlFor={`page-param-${endpoint.id}`}>Page parameter</FieldLabel><Input id={`page-param-${endpoint.id}`} value={draft.config.page_parameter} onChange={(event) => updateConfig('page_parameter', event.target.value)} placeholder="page" /></Field>
          <Field><FieldLabel htmlFor={`max-pages-${endpoint.id}`}>Maximum pages</FieldLabel><Input id={`max-pages-${endpoint.id}`} type="number" min={1} max={100} value={draft.max_pages} onChange={(event) => setDraft((current) => ({ ...current, max_pages: Number(event.target.value) }))} /></Field>
          <Field><FieldLabel htmlFor={`max-requests-${endpoint.id}`}>Requests per run</FieldLabel><Input id={`max-requests-${endpoint.id}`} type="number" min={1} max={500} value={draft.max_requests_per_run} onChange={(event) => setDraft((current) => ({ ...current, max_requests_per_run: Number(event.target.value) }))} /></Field>
          <Field><FieldLabel htmlFor={`user-agent-${endpoint.id}`}>Crawler user agent</FieldLabel><Input id={`user-agent-${endpoint.id}`} value={draft.config.user_agent} onChange={(event) => updateConfig('user_agent', event.target.value)} /></Field>
          <Field><FieldLabel htmlFor={`min-content-${endpoint.id}`}>Minimum body characters</FieldLabel><Input id={`min-content-${endpoint.id}`} type="number" min={100} max={50000} value={draft.config.min_content_characters} onChange={(event) => updateConfig('min_content_characters', Number(event.target.value))} /></Field>
          <Field><FieldLabel htmlFor={`sinhala-ratio-${endpoint.id}`}>Minimum Sinhala ratio</FieldLabel><Input id={`sinhala-ratio-${endpoint.id}`} type="number" min={0} max={1} step={0.05} value={draft.config.minimum_sinhala_ratio} onChange={(event) => updateConfig('minimum_sinhala_ratio', Number(event.target.value))} /></Field>
        </div>
        <p className="text-xs leading-relaxed text-muted-foreground">Static crawling is still blocked unless the active compliance review explicitly permits full-text storage and confirms robots.txt access. All outbound requests remain limited to these source hosts.</p>
        <div className="flex justify-end"><Button type="submit" disabled={pending}>{pending ? 'Saving…' : 'Save collection recipe'}</Button></div>
      </form>
    </details>
  )
}

function fallbackCompliance(sourceID: string): AdminComplianceReview {
  return {
    id: '', source_id: sourceID, version: 0, status: 'pending', robots_url: '',
    robots_checked_at: null, robots_allowed: null, terms_urls: [],
    allow_discovery: false, allow_full_text_storage: false, allow_ai_processing: false,
    allow_embeddings: false, allow_training: false, allow_public_full_text: false,
    notes: '', reviewed_by: '', reviewed_at: null, review_on: null, created_at: '',
  }
}

export function SourceCollectionControls({ sourceID, endpoints }: { sourceID: string; endpoints: AdminEndpoint[] }) {
  const queryClient = useQueryClient()
  const collections = useQuery({ queryKey: ['source-collections', sourceID], queryFn: () => client.sourceCollections(sourceID), enabled: Boolean(sourceID) })
  const compliance = useQuery({ queryKey: ['source-compliance', sourceID], queryFn: () => client.sourceCompliance(sourceID), enabled: Boolean(sourceID) })
  const saveCollection = useMutation({
    mutationFn: (profile: AdminCollectionProfile) => client.saveSourceCollection(sourceID, {
      endpoint_id: profile.endpoint_id,
      discovery_method: profile.discovery_method,
      article_method: profile.article_method,
      config: profile.config,
      min_delay_seconds: profile.min_delay_seconds,
      max_requests_per_run: profile.max_requests_per_run,
      max_pages: profile.max_pages,
      request_timeout_seconds: profile.request_timeout_seconds,
    }),
    onSuccess: () => { toast.success('Collection recipe activated'); void queryClient.invalidateQueries({ queryKey: ['source-collections', sourceID] }) },
    onError: () => toast.error('Could not save the collection recipe'),
  })
  const saveCompliance = useMutation({
    mutationFn: (review: AdminComplianceReview) => client.saveSourceCompliance(sourceID, {
      status: review.status,
      robots_url: review.robots_url,
      robots_checked_at: review.robots_checked_at,
      robots_allowed: review.robots_allowed,
      terms_urls: review.terms_urls,
      allow_discovery: review.allow_discovery,
      allow_full_text_storage: review.allow_full_text_storage,
      allow_ai_processing: review.allow_ai_processing,
      allow_embeddings: review.allow_embeddings,
      allow_training: review.allow_training,
      allow_public_full_text: review.allow_public_full_text,
      notes: review.notes,
      review_on: review.review_on,
    }),
    onSuccess: () => { toast.success('Compliance review activated'); void queryClient.invalidateQueries({ queryKey: ['source-compliance', sourceID] }) },
    onError: () => toast.error('Could not save the compliance review'),
  })

  return (
    <section className="flex min-w-0 flex-col gap-4">
      <div>
        <h2 className="flex items-center gap-2 text-lg font-semibold"><FileText className="size-5" />Collection & compliance</h2>
        <p className="mt-1 text-sm text-muted-foreground">Configure how this publisher is discovered and enforce the documented legal boundary before full content is stored or processed.</p>
      </div>
      {compliance.isPending ? <Skeleton className="h-96 rounded-4xl" /> : compliance.isError && !compliance.data ? (
        <ComplianceEditor review={fallbackCompliance(sourceID)} pending={saveCompliance.isPending} onSave={(review) => saveCompliance.mutate(review)} />
      ) : (
        <ComplianceEditor review={compliance.data ?? fallbackCompliance(sourceID)} pending={saveCompliance.isPending} onSave={(review) => saveCompliance.mutate(review)} />
      )}
      {collections.isPending ? <Skeleton className="h-32 rounded-4xl" /> : endpoints.map((endpoint) => (
        <Card key={endpoint.id} className="min-w-0 gap-0 overflow-hidden py-0 shadow-sm">
          <CollectionEditor endpoint={endpoint} profile={collections.data?.items.find((item) => item.endpoint_id === endpoint.id)} pending={saveCollection.isPending && saveCollection.variables?.endpoint_id === endpoint.id} onSave={(profile) => saveCollection.mutate(profile)} />
        </Card>
      ))}
    </section>
  )
}
