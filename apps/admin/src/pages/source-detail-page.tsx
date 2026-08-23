import {
  createClient,
  type AdminEndpoint,
  type AdminRights,
} from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Activity,
  ArrowLeft,
  CheckCircle2,
  Clock3,
  ExternalLink,
  FileCheck2,
  Gauge,
  Pause,
  Play,
  Plus,
  RadioTower,
  RefreshCw,
  ShieldCheck,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { toast } from 'sonner'

import { SourceAvatar } from '@/components/source-avatar'
import { SourceCollectionControls } from '@/components/source-collection-controls'
import { SourcePerformanceCharts } from '@/components/source-performance-charts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'

const client = createClient()
const endpointTypes = ['rss', 'atom', 'json_feed', 'rest_api', 'webhook', 'youtube']
const rightsModes = [
  'discovery_only',
  'licensed_excerpt',
  'licensed_media',
  'full_syndication',
  'internal_verification',
  'disabled',
]

const modeDescriptions: Record<string, string> = {
  discovery_only: 'Publish the source name, headline, time, category, and original link.',
  licensed_excerpt: 'Also publish the publisher-approved excerpt with each item.',
  licensed_media: 'Also display media explicitly approved by the publisher.',
  full_syndication: 'Publish the fields covered by a negotiated syndication agreement.',
  internal_verification: 'Capture for editorial comparison without public publication.',
  disabled: 'Do not automatically retrieve or publish content from this endpoint.',
}

type Confirmation =
  | { kind: 'source'; nextActive: boolean }
  | { kind: 'endpoint'; endpoint: AdminEndpoint }

function label(value: string) {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (character) => character.toUpperCase())
}

function formatDate(value?: string | null) {
  if (!value) return 'No successful sync yet'
  return new Date(value).toLocaleString('en', {
    dateStyle: 'medium',
    timeStyle: 'short',
  })
}

function hostname(value: string) {
  try {
    return new URL(value).hostname
  } catch {
    return value
  }
}

function safeExternalURL(value: string) {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'https:' ? parsed.toString() : undefined
  } catch {
    return undefined
  }
}

function relativeTime(value?: string | null) {
  if (!value) return 'Never'
  const elapsed = Date.now() - new Date(value).getTime()
  if (elapsed < 60_000) return 'Just now'
  if (elapsed < 3_600_000) return `${Math.floor(elapsed / 60_000)} min ago`
  if (elapsed < 86_400_000) return `${Math.floor(elapsed / 3_600_000)} hr ago`
  return `${Math.floor(elapsed / 86_400_000)} days ago`
}

function latency(value: number | null) {
  if (value === null) return 'Not measured yet'
  if (value < 1000) return `${value} ms`
  return `${(value / 1000).toFixed(2)} s`
}

function interval(seconds: number) {
  if (seconds % 3600 === 0) return `${seconds / 3600} hr`
  if (seconds % 60 === 0) return `${seconds / 60} min`
  return `${seconds} sec`
}

function StatusDot({ state }: { state: 'good' | 'warning' | 'muted' }) {
  return (
    <span
      aria-hidden="true"
      className={
        state === 'good'
          ? 'size-2 rounded-full bg-emerald-500'
          : state === 'warning'
            ? 'size-2 rounded-full bg-amber-500'
            : 'size-2 rounded-full bg-muted-foreground'
      }
    />
  )
}

function PerformanceMetric({
  label: metricLabel,
  value,
  description,
}: {
  label: string
  value: string
  description: string
}) {
  return (
    <div className="min-w-0 px-4 py-5 first:pl-0 last:pr-0 md:px-6 md:not-first:border-l">
      <p className="text-sm text-muted-foreground">{metricLabel}</p>
      <p className="mt-1 truncate text-2xl font-semibold tracking-tight tabular-nums">{value}</p>
      <p className="mt-1 truncate text-xs text-muted-foreground">{description}</p>
    </div>
  )
}

function RightsEditor({
  endpoint,
  profile,
  sourceName,
  pending,
  onSave,
}: {
  endpoint: AdminEndpoint
  profile?: AdminRights
  sourceName: string
  pending: boolean
  onSave: (endpointId: string, mode: string, attribution: string) => void
}) {
  const [mode, setMode] = useState(profile?.mode ?? 'discovery_only')
  const [attribution, setAttribution] = useState(profile?.attribution ?? `මූලාශ්‍රය: ${sourceName}`)

  useEffect(() => {
    setMode(profile?.mode ?? 'discovery_only')
    setAttribution(profile?.attribution ?? `මූලාශ්‍රය: ${sourceName}`)
  }, [profile, sourceName])

  return (
    <form
      className="flex h-full flex-col gap-5 p-5 sm:p-6"
      onSubmit={(event) => {
        event.preventDefault()
        onSave(endpoint.id, mode, attribution)
      }}
    >
      <div className="flex items-center justify-between gap-3">
        <div>
          <h3 className="font-semibold">Publishing rights</h3>
          <p className="mt-1 text-sm text-muted-foreground">How content from this endpoint may be used.</p>
        </div>
        <Badge variant={profile ? 'secondary' : 'destructive'}>
          {profile ? 'Configured' : 'Required'}
        </Badge>
      </div>

      <FieldGroup>
        <Field>
          <FieldLabel htmlFor={`mode-${endpoint.id}`}>Operating mode</FieldLabel>
          <Select value={mode} onValueChange={(value) => value && setMode(value)}>
            <SelectTrigger id={`mode-${endpoint.id}`} className="w-full">
              <SelectValue>{(value) => label(String(value))}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              {rightsModes.map((item) => (
                <SelectItem key={item} value={item}>
                  {label(item)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className="text-xs leading-relaxed text-muted-foreground">{modeDescriptions[mode]}</p>
        </Field>
        <Field>
          <FieldLabel htmlFor={`attribution-${endpoint.id}`}>Attribution</FieldLabel>
          <Input
            id={`attribution-${endpoint.id}`}
            value={attribution}
            onChange={(event) => setAttribution(event.target.value)}
            required
          />
          <p className="text-xs text-muted-foreground">Shown with published or discovered content.</p>
        </Field>
      </FieldGroup>

      <div className="mt-auto flex justify-end">
        <Button type="submit" disabled={pending}>
          {pending ? 'Saving…' : 'Save rights'}
        </Button>
      </div>
    </form>
  )
}

export function SourceDetailPage() {
  const { id = '' } = useParams()
  const queryClient = useQueryClient()
  const [addEndpointOpen, setAddEndpointOpen] = useState(false)
  const [feedType, setFeedType] = useState('rss')
  const [feedUrl, setFeedUrl] = useState('')
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null)

  const source = useQuery({
    queryKey: ['source', id],
    queryFn: () => client.adminSource(id),
    enabled: Boolean(id),
  })
  const performance = useQuery({
    queryKey: ['source-performance', id, 30],
    queryFn: () => client.sourcePerformance(id, 30),
    enabled: Boolean(id),
  })
  const endpoints = useQuery({
    queryKey: ['endpoints', id, 'detail'],
    queryFn: () => client.adminEndpoints(id, { per_page: 100 }),
    enabled: Boolean(id),
  })
  const rights = useQuery({
    queryKey: ['rights', id, 'detail'],
    queryFn: () => client.adminRights(id, { per_page: 100 }),
    enabled: Boolean(id),
  })

  const invalidateSource = () => {
    void queryClient.invalidateQueries({ queryKey: ['source', id] })
    void queryClient.invalidateQueries({ queryKey: ['source-performance', id] })
    void queryClient.invalidateQueries({ queryKey: ['endpoints', id] })
  }

  const addEndpoint = useMutation({
    mutationFn: () => client.createEndpoint(id, { endpoint_type: feedType, url: feedUrl }),
    onSuccess: () => {
      toast.success('Endpoint added and paused for review')
      setFeedUrl('')
      setAddEndpointOpen(false)
      invalidateSource()
    },
    onError: () => toast.error('Could not add endpoint'),
  })
  const saveRights = useMutation({
    mutationFn: ({ endpointId, mode, attribution }: { endpointId: string; mode: string; attribution: string }) =>
      client.createRights(id, { endpoint_id: endpointId, mode, attribution }),
    onSuccess: () => {
      toast.success('Publishing rights saved')
      void queryClient.invalidateQueries({ queryKey: ['rights', id] })
    },
    onError: () => toast.error('Could not save publishing rights'),
  })
  const run = useMutation({
    mutationFn: (endpointId: string) => client.runEndpoint(endpointId),
    onSuccess: () => {
      toast.success('Endpoint sync completed')
      invalidateSource()
    },
    onError: () => toast.error('Endpoint sync failed'),
  })
  const test = useMutation({
    mutationFn: (endpointId: string) => client.testEndpoint(endpointId),
    onSuccess: (result) =>
      toast.success(
        result.parseable === false
          ? `HTTP ${result.status} · response could not be parsed`
          : `HTTP ${result.status} · endpoint is reachable and parseable`,
      ),
    onError: () => toast.error('Endpoint test failed'),
  })
  const pause = useMutation({
    mutationFn: ({ endpointId, paused }: { endpointId: string; paused: boolean }) =>
      client.pauseEndpoint(endpointId, paused),
    onSuccess: (_, variables) => {
      toast.success(variables.paused ? 'Endpoint paused' : 'Endpoint resumed')
      setConfirmation(null)
      invalidateSource()
    },
    onError: () => toast.error('Could not update endpoint'),
  })
  const toggleSource = useMutation({
    mutationFn: (active: boolean) => client.setSourceActive(id, active),
    onSuccess: (_, active) => {
      toast.success(active ? 'Source activated' : 'Source held')
      setConfirmation(null)
      invalidateSource()
    },
    onError: () => toast.error('Could not update source'),
  })

  const sourceData = source.data
  const endpointItems = endpoints.data?.items ?? []
  const isRunning = Boolean(sourceData?.active && endpointItems.some((endpoint) => !endpoint.paused))
  const publishedRate = performance.data?.total_captured
    ? Math.round((performance.data.published / performance.data.total_captured) * 100)
    : 0

  if (source.isError) {
    return (
      <section className="flex min-h-[50vh] flex-col items-center justify-center gap-4 text-center">
        <RadioTower className="size-8 text-muted-foreground" />
        <div>
          <h1 className="text-xl font-semibold">Source not found</h1>
          <p className="mt-1 text-sm text-muted-foreground">This source may have been removed.</p>
        </div>
        <Button variant="outline" nativeButton={false} render={<Link to="/sources" />}>
          <ArrowLeft />
          Back to sources
        </Button>
      </section>
    )
  }

  return (
    <section className="flex flex-col gap-6">
      <Button
        className="-ml-2 w-fit"
        variant="ghost"
        size="sm"
        nativeButton={false}
        render={<Link to="/sources" />}
      >
        <ArrowLeft />
        Back to sources
      </Button>

      <div className="flex flex-col gap-5 border-b pb-6 lg:flex-row lg:items-start lg:justify-between">
        <div className="flex min-w-0 items-start gap-4">
          {source.isPending ? (
            <Skeleton className="size-16 rounded-2xl" />
          ) : (
            <SourceAvatar
              className="size-16 rounded-2xl"
              name={sourceData?.name ?? 'Source'}
              website={sourceData?.website}
              iconUrl={sourceData?.icon_url}
            />
          )}
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              {source.isPending ? (
                <Skeleton className="h-8 w-48" />
              ) : (
                <h1 className="truncate text-2xl font-semibold tracking-tight sm:text-3xl">{sourceData?.name}</h1>
              )}
              {sourceData ? (
                <Badge variant="outline" className="gap-1.5">
                  <StatusDot state={sourceData.active ? 'good' : 'warning'} />
                  {sourceData.active ? 'Active' : 'Held'}
                </Badge>
              ) : null}
            </div>
            <p className="mt-1 text-sm text-muted-foreground">{sourceData?.legal_name}</p>
            <div className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-2 text-sm">
              <span className="text-muted-foreground">{sourceData ? label(sourceData.source_type) : 'Loading…'}</span>
              {sourceData?.website && safeExternalURL(sourceData.website) ? (
                <a
                  href={safeExternalURL(sourceData.website)}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1.5 text-primary hover:underline"
                >
                  {hostname(sourceData.website)}
                  <ExternalLink className="size-3.5" />
                </a>
              ) : null}
              <span className="inline-flex items-center gap-2">
                <StatusDot state={isRunning ? 'good' : 'muted'} />
                {isRunning ? 'Ingestion running' : 'Ingestion stopped'}
              </span>
            </div>
          </div>
        </div>

        {sourceData ? (
          <Button
            variant={sourceData.active ? 'outline' : 'default'}
            onClick={() => setConfirmation({ kind: 'source', nextActive: !sourceData.active })}
          >
            {sourceData.active ? <Pause /> : <Play />}
            {sourceData.active ? 'Hold source' : 'Activate source'}
          </Button>
        ) : null}
      </div>

      <div>
        <h2 className="text-base font-semibold">Performance overview</h2>
        <div className="mt-3 grid grid-cols-2 border-y md:grid-cols-4">
          {performance.isPending ? (
            Array.from({ length: 4 }, (_, index) => (
              <div key={index} className="px-4 py-5 first:pl-0 md:px-6 md:not-first:border-l">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="mt-2 h-8 w-20" />
                <Skeleton className="mt-2 h-3 w-28" />
              </div>
            ))
          ) : (
            <>
              <PerformanceMetric
                label="Total captured"
                value={(performance.data?.total_captured ?? 0).toLocaleString()}
                description="Stored news items"
              />
              <PerformanceMetric
                label="Captured today"
                value={(performance.data?.captured_today ?? 0).toLocaleString()}
                description="Asia/Colombo day"
              />
              <PerformanceMetric
                label="Published"
                value={`${publishedRate}%`}
                description={`${(performance.data?.published ?? 0).toLocaleString()} of ${(performance.data?.total_captured ?? 0).toLocaleString()} items`}
              />
              <PerformanceMetric
                label="Last successful sync"
                value={relativeTime(performance.data?.last_success_at)}
                description={formatDate(performance.data?.last_success_at)}
              />
            </>
          )}
        </div>
      </div>

      {performance.isPending ? (
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1.7fr)_minmax(320px,0.8fr)]">
          <Skeleton className="h-[370px] rounded-4xl" />
          <Skeleton className="h-[370px] rounded-4xl" />
        </div>
      ) : performance.isError ? (
        <Card className="shadow-sm">
          <CardContent className="flex h-48 items-center justify-center text-sm text-muted-foreground">
            Performance history is temporarily unavailable.
          </CardContent>
        </Card>
      ) : (
        <SourcePerformanceCharts daily={performance.data.daily} />
      )}

      {!endpoints.isPending && !endpoints.isError ? (
        <SourceCollectionControls sourceID={id} endpoints={endpointItems} />
      ) : null}

      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h2 className="text-lg font-semibold">Endpoints & publishing rights</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Monitor each connection and control how its content may be published.
            </p>
          </div>
          <Dialog open={addEndpointOpen} onOpenChange={setAddEndpointOpen}>
            <DialogTrigger render={<Button size="sm" />}>
              <Plus />
              Add endpoint
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Add endpoint</DialogTitle>
                <DialogDescription>Add an official machine-readable feed for this publisher.</DialogDescription>
              </DialogHeader>
              <form
                onSubmit={(event) => {
                  event.preventDefault()
                  addEndpoint.mutate()
                }}
              >
                <FieldGroup>
                  <Field>
                    <FieldLabel htmlFor="endpoint-type">Type</FieldLabel>
                    <Select value={feedType} onValueChange={(value) => value && setFeedType(value)}>
                      <SelectTrigger id="endpoint-type" className="w-full">
                        <SelectValue>{(value) => label(String(value))}</SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        {endpointTypes.map((type) => (
                          <SelectItem key={type} value={type}>
                            {label(type)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="endpoint-url">HTTPS URL</FieldLabel>
                    <Input
                      id="endpoint-url"
                      type="url"
                      value={feedUrl}
                      onChange={(event) => setFeedUrl(event.target.value)}
                      placeholder="https://publisher.lk/feed/"
                      required
                    />
                  </Field>
                  <DialogFooter>
                    <Button type="button" variant="outline" onClick={() => setAddEndpointOpen(false)}>
                      Cancel
                    </Button>
                    <Button type="submit" disabled={addEndpoint.isPending}>
                      {addEndpoint.isPending ? 'Adding…' : 'Add endpoint'}
                    </Button>
                  </DialogFooter>
                </FieldGroup>
              </form>
            </DialogContent>
          </Dialog>
        </div>

        {endpoints.isPending || rights.isPending ? (
          <Skeleton className="h-[420px] rounded-4xl" />
        ) : endpoints.isError || rights.isError ? (
          <Card className="shadow-sm">
            <CardContent className="flex h-48 items-center justify-center text-sm text-muted-foreground">
              Endpoint configuration is temporarily unavailable.
            </CardContent>
          </Card>
        ) : endpointItems.length === 0 ? (
          <Card className="shadow-sm">
            <CardContent className="flex min-h-52 flex-col items-center justify-center gap-3 text-center">
              <RadioTower className="size-7 text-muted-foreground" />
              <div>
                <h3 className="font-semibold">No endpoint configured</h3>
                <p className="mt-1 text-sm text-muted-foreground">Add an official feed to start capturing news.</p>
              </div>
              <Button size="sm" onClick={() => setAddEndpointOpen(true)}>
                <Plus />
                Add endpoint
              </Button>
            </CardContent>
          </Card>
        ) : (
          endpointItems.map((endpoint, index) => {
            const profile = rights.data?.items.find((item) => item.endpoint_id === endpoint.id)
            const endpointRunning = sourceData?.active && !endpoint.paused
            return (
              <Card key={endpoint.id} className="gap-0 overflow-hidden py-0 shadow-sm">
                <div className="grid xl:grid-cols-[minmax(0,1.05fr)_minmax(380px,0.95fr)]">
                  <div className="p-5 sm:p-6">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="font-semibold">Endpoint {index + 1}</h3>
                        <Badge variant="outline" className="gap-1.5">
                          <StatusDot state={endpoint.health_state === 'healthy' ? 'good' : 'warning'} />
                          {label(endpoint.health_state)}
                        </Badge>
                        <Badge variant="outline" className="gap-1.5">
                          <StatusDot state={endpointRunning ? 'good' : 'muted'} />
                          {endpointRunning ? 'Running' : 'Paused'}
                        </Badge>
                      </div>
                      <a
                        href={safeExternalURL(endpoint.url)}
                        target="_blank"
                        rel="noreferrer"
                        className="mt-2 inline-flex max-w-full items-center gap-1.5 break-all text-sm text-primary hover:underline"
                      >
                        {endpoint.url}
                        <ExternalLink className="size-3.5 shrink-0" />
                      </a>
                    </div>

                    <dl className="mt-6 grid grid-cols-2 gap-x-6 gap-y-5 border-y py-5 text-sm">
                      <div>
                        <dt className="flex items-center gap-2 text-muted-foreground"><RadioTower className="size-4" />Type</dt>
                        <dd className="mt-1 font-medium">{label(endpoint.endpoint_type)}</dd>
                      </div>
                      <div>
                        <dt className="flex items-center gap-2 text-muted-foreground"><Clock3 className="size-4" />Last sync</dt>
                        <dd className="mt-1 font-medium">{relativeTime(endpoint.last_success_at)}</dd>
                      </div>
                      <div>
                        <dt className="flex items-center gap-2 text-muted-foreground"><RefreshCw className="size-4" />Polling interval</dt>
                        <dd className="mt-1 font-medium">Every {interval(endpoint.polling_interval_seconds)}</dd>
                      </div>
                      <div>
                        <dt className="flex items-center gap-2 text-muted-foreground"><Gauge className="size-4" />Last latency</dt>
                        <dd className="mt-1 font-medium tabular-nums">{latency(endpoint.last_latency_ms)}</dd>
                      </div>
                      <div>
                        <dt className="flex items-center gap-2 text-muted-foreground"><FileCheck2 className="size-4" />Captured</dt>
                        <dd className="mt-1 font-medium tabular-nums">{endpoint.total_captured.toLocaleString()} items</dd>
                      </div>
                      <div>
                        <dt className="flex items-center gap-2 text-muted-foreground"><Activity className="size-4" />Latest fetch</dt>
                        <dd className="mt-1 font-medium tabular-nums">{endpoint.last_new_item_count} new · {endpoint.last_item_count} read</dd>
                      </div>
                      <div className="col-span-2">
                        <dt className="flex items-center gap-2 text-muted-foreground"><ShieldCheck className="size-4" />Official verification</dt>
                        <dd className="mt-1 flex items-center gap-2 font-medium">
                          {endpoint.verified_official ? <CheckCircle2 className="size-4 text-emerald-500" /> : null}
                          {endpoint.verified_official ? 'Verified official endpoint' : 'Not verified'}
                        </dd>
                      </div>
                    </dl>

                    {endpoint.last_error ? (
                      <p className="mt-4 rounded-2xl bg-destructive/10 px-3 py-2 text-sm text-destructive">
                        {endpoint.last_error}
                      </p>
                    ) : null}

                    <div className="mt-5 flex flex-wrap gap-2">
                      <Button
                        size="sm"
                        disabled={run.isPending && run.variables === endpoint.id}
                        onClick={() => run.mutate(endpoint.id)}
                      >
                        <Play />
                        {run.isPending && run.variables === endpoint.id ? 'Running…' : 'Run now'}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={test.isPending && test.variables === endpoint.id}
                        onClick={() => test.mutate(endpoint.id)}
                      >
                        <Activity />
                        {test.isPending && test.variables === endpoint.id ? 'Testing…' : 'Test'}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setConfirmation({ kind: 'endpoint', endpoint })}
                      >
                        {endpoint.paused ? <Play /> : <Pause />}
                        {endpoint.paused ? 'Resume' : 'Pause'}
                      </Button>
                    </div>
                  </div>

                  <div className="border-t bg-muted/15 xl:border-t-0 xl:border-l">
                    <RightsEditor
                      endpoint={endpoint}
                      profile={profile}
                      sourceName={sourceData?.name ?? 'Source'}
                      pending={saveRights.isPending && saveRights.variables?.endpointId === endpoint.id}
                      onSave={(endpointId, nextMode, attribution) =>
                        saveRights.mutate({ endpointId, mode: nextMode, attribution })
                      }
                    />
                  </div>
                </div>
              </Card>
            )
          })
        )}
      </div>

      <Dialog
        open={confirmation !== null}
        onOpenChange={(open) => {
          if (!open && !pause.isPending && !toggleSource.isPending) setConfirmation(null)
        }}
      >
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>
              {confirmation?.kind === 'source'
                ? `${confirmation.nextActive ? 'Activate' : 'Hold'} ${sourceData?.name ?? 'source'}?`
                : `${confirmation?.endpoint.paused ? 'Resume' : 'Pause'} this endpoint?`}
            </DialogTitle>
            <DialogDescription>
              {confirmation?.kind === 'source'
                ? confirmation.nextActive
                  ? 'Future captured articles can be published according to the configured rights.'
                  : 'Newly captured articles will be held from publication until this source is activated again.'
                : confirmation?.endpoint.paused
                  ? 'Automatic polling will resume using the configured interval.'
                  : 'Automatic polling will stop until this endpoint is resumed.'}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmation(null)} disabled={pause.isPending || toggleSource.isPending}>
              Cancel
            </Button>
            <Button
              variant={
                confirmation?.kind === 'source' && !confirmation.nextActive
                  ? 'destructive'
                  : confirmation?.kind === 'endpoint' && !confirmation.endpoint.paused
                    ? 'destructive'
                    : 'default'
              }
              disabled={pause.isPending || toggleSource.isPending}
              onClick={() => {
                if (!confirmation) return
                if (confirmation.kind === 'source') {
                  toggleSource.mutate(confirmation.nextActive)
                  return
                }
                pause.mutate({ endpointId: confirmation.endpoint.id, paused: !confirmation.endpoint.paused })
              }}
            >
              {pause.isPending || toggleSource.isPending
                ? 'Working…'
                : confirmation?.kind === 'source'
                  ? `${confirmation.nextActive ? 'Activate' : 'Hold'} source`
                  : `${confirmation?.endpoint.paused ? 'Resume' : 'Pause'} endpoint`}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  )
}
