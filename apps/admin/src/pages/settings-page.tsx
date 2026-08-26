import {
  createClient,
  type AdminArticleListItem,
  type AnalysisBackfillProvider,
  type AnalysisBackfillRequest,
  type AnalysisBackfillRun,
  type AnalysisBackfillScope,
  type AnalysisBackfillWorkflow,
} from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  Bot,
  CheckCircle2,
  CircleAlert,
  Clock3,
  DatabaseZap,
  ExternalLink,
  FileText,
  LoaderCircle,
  EllipsisVertical,
  Pause,
  RefreshCw,
  Search,
  ServerCog,
  Sparkles,
  Square,
  TerminalSquare,
  Trash2,
  Wifi,
  WifiOff,
} from 'lucide-react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

const client = createClient()
const dateTime = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' })
type BackfillStreamState = 'connecting' | 'live' | 'reconnecting'
type DestructiveBackfillAction = { kind: 'cancel' | 'delete'; run: AnalysisBackfillRun }

export function SettingsPage() {
  const queryClient = useQueryClient()
  const [scope, setScope] = useState<AnalysisBackfillScope>('date_range')
  const [workflow, setWorkflow] = useState<AnalysisBackfillWorkflow>('full_pipeline')
  const [provider, setProvider] = useState<AnalysisBackfillProvider>('openrouter')
  const [selectedModel, setSelectedModel] = useState('')
  const [from, setFrom] = useState(() => dateInput(daysAgo(7)))
  const [to, setTo] = useState(() => dateInput(new Date()))
  const [confirmation, setConfirmation] = useState('')
  const [articleSearch, setArticleSearch] = useState('')
  const [submittedSearch, setSubmittedSearch] = useState('')
  const [selectedArticle, setSelectedArticle] = useState<AdminArticleListItem | null>(null)
  const [streamState, setStreamState] = useState<BackfillStreamState>('connecting')
  const [destructiveAction, setDestructiveAction] = useState<DestructiveBackfillAction | null>(null)

  const openRouter = useQuery({
    queryKey: ['llm-provider'],
    queryFn: () => client.llmProvider(),
    staleTime: 60_000,
  })
  const models = useQuery({
    queryKey: ['llm-models'],
    queryFn: () => client.llmModels(),
    staleTime: 60_000,
  })
  const profiles = useQuery({
    queryKey: ['llm-profiles'],
    queryFn: () => client.llmProfiles(),
    staleTime: 60_000,
  })
  const codex = useQuery({
    queryKey: ['codex-status'],
    queryFn: () => client.codexStatus(),
    staleTime: 30_000,
  })
  const runs = useQuery({
    queryKey: ['analysis-backfills'],
    queryFn: () => client.analysisBackfills(),
    refetchOnWindowFocus: false,
    refetchInterval: (query) => query.state.data?.items.some((run) => run.status === 'queued' || run.status === 'running') ? 3_000 : false,
  })
  const articles = useQuery({
    queryKey: ['backfill-article-search', submittedSearch],
    queryFn: () => client.adminArticles({ page: 1, per_page: 6, search: submittedSearch }),
    enabled: scope === 'article' && submittedSearch.trim().length >= 2,
    staleTime: 30_000,
  })

  const openRouterModels = useMemo(() => {
    return (models.data?.items ?? []).filter((model) =>
      model.output_modalities.includes('text') &&
      (model.supported_parameters.includes('structured_outputs') || model.supported_parameters.includes('response_format')),
    )
  }, [models.data?.items])
  const defaultOpenRouterModel = profiles.data?.items.find((profile) => profile.task === 'narration_framing')?.model ?? openRouterModels[0]?.id ?? ''
  const availableModels = provider === 'codex_cli' ? (codex.data?.models ?? []) : openRouterModels.map((model) => model.id)
  const model = availableModels.includes(selectedModel)
    ? selectedModel
    : provider === 'openrouter' && availableModels.includes(defaultOpenRouterModel)
      ? defaultOpenRouterModel
      : availableModels[0] ?? ''

  const backfillRequest = useMemo<AnalysisBackfillRequest>(() => ({
    scope,
    workflow,
    provider,
    model,
    ...(scope === 'date_range' ? { from, to } : {}),
    ...(scope === 'article' && selectedArticle ? { article_id: selectedArticle.id } : {}),
  }), [from, model, provider, scope, selectedArticle, to, workflow])
  const previewRequest = useMemo<AnalysisBackfillRequest>(() => ({
    scope,
    workflow,
    provider,
    model: model || 'preview',
    ...(scope === 'date_range' ? { from, to } : {}),
    ...(scope === 'article' && selectedArticle ? { article_id: selectedArticle.id } : {}),
  }), [from, model, provider, scope, selectedArticle, to, workflow])
  const previewReady = Boolean(
    (scope !== 'date_range' || (from && to && from <= to)) &&
    (scope !== 'article' || selectedArticle),
  )
  const preview = useQuery({
    queryKey: ['analysis-backfill-preview', previewRequest],
    queryFn: () => client.analysisBackfillPreview(previewRequest),
    enabled: previewReady,
    staleTime: 10_000,
  })
  const providerReady = provider === 'openrouter' ? Boolean(openRouter.data?.available) : Boolean(codex.data?.ready)
  const catalogConfirmed = scope !== 'catalog' || confirmation === 'BACKFILL ENTIRE CATALOG'

  const start = useMutation({
    mutationFn: () => client.createAnalysisBackfill({
      ...backfillRequest,
      ...(scope === 'catalog' ? { confirmation } : {}),
    }),
    onSuccess: async (run) => {
      await queryClient.invalidateQueries({ queryKey: ['analysis-backfills'] })
      toast.success(`Backfill queued for ${run.total_articles.toLocaleString()} articles`)
      setConfirmation('')
    },
    onError: () => toast.error('Could not create the administrative backfill'),
  })

  const controlBackfill = useMutation({
    mutationFn: ({ id, action }: { id: string; action: 'pause' | 'resume' | 'cancel' }) => {
      if (action === 'pause') return client.pauseAnalysisBackfill(id)
      if (action === 'resume') return client.resumeAnalysisBackfill(id)
      return client.cancelAnalysisBackfill(id)
    },
    onSuccess: async (run, variables) => {
      queryClient.setQueryData<{ items: AnalysisBackfillRun[] }>(['analysis-backfills'], (current) => ({
        items: current?.items.map((item) => item.id === run.id ? run : item) ?? [run],
      }))
      await queryClient.invalidateQueries({ queryKey: ['analysis-backfills'] })
      toast.success(variables.action === 'pause' ? 'Backfill paused' : variables.action === 'resume' ? 'Backfill resumed' : 'Backfill stopped')
      setDestructiveAction(null)
    },
    onError: () => toast.error('Could not update the administrative backfill'),
  })

  const deleteBackfill = useMutation({
    mutationFn: (id: string) => client.deleteAnalysisBackfill(id),
    onSuccess: async (_, id) => {
      queryClient.setQueryData<{ items: AnalysisBackfillRun[] }>(['analysis-backfills'], (current) => ({
        items: current?.items.filter((item) => item.id !== id) ?? [],
      }))
      await queryClient.invalidateQueries({ queryKey: ['analysis-backfills'] })
      toast.success('Backfill record deleted')
      setDestructiveAction(null)
    },
    onError: () => toast.error('Could not delete the administrative backfill record'),
  })

  useEffect(() => {
	const events = new EventSource(client.analysisBackfillStreamURL(), { withCredentials: true })
	const updateRuns = (event: MessageEvent<string>) => {
	  try {
		queryClient.setQueryData(['analysis-backfills'], JSON.parse(event.data) as { items: AnalysisBackfillRun[] })
		setStreamState('live')
	  } catch {
		// EventSource reconnects automatically; retain the last valid snapshot.
	  }
	}
	events.onopen = () => setStreamState('live')
	events.onerror = () => setStreamState('reconnecting')
	events.addEventListener('analysis-backfills', updateRuns as EventListener)
    return () => events.close()
  }, [queryClient])

  const refreshProviders = () => void Promise.all([openRouter.refetch(), models.refetch(), profiles.refetch(), codex.refetch()])

  return (
    <section className="flex min-w-0 flex-col gap-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="min-w-0">
          <Badge variant="outline" className="mb-2"><ServerCog /> Administration</Badge>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">System settings</h1>
          <p className="mt-1 max-w-2xl text-sm text-muted-foreground">
            Verify local inference tools and run exceptional, isolated analysis backfills.
          </p>
        </div>
        <Button variant="outline" onClick={refreshProviders} disabled={openRouter.isFetching || codex.isFetching}>
          <RefreshCw className={cn((openRouter.isFetching || codex.isFetching) && 'animate-spin')} /> Refresh readiness
        </Button>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <ProviderCard
          icon={Sparkles}
          name="OpenRouter"
          description="Default provider for regular workflows and administrative backfills."
          ready={Boolean(openRouter.data?.available)}
          pending={openRouter.isPending}
          detail={openRouter.data?.status_detail ?? 'Checking API-key availability…'}
          metadata={openRouter.data ? `${openRouter.data.latency_ms} ms · ${models.data?.items.length ?? 0} models` : ''}
        />
        <ProviderCard
          icon={TerminalSquare}
          name="Codex CLI"
          description="Pluggable local provider for single-pass analysis and complete editorial backfills."
          ready={Boolean(codex.data?.ready)}
          pending={codex.isPending}
          detail={codex.data?.detail ?? 'Checking installation and authentication…'}
          metadata={codex.data?.installed ? `${codex.data.version || 'Version unknown'} · ${authLabel(codex.data.auth_method)}` : ''}
        />
      </div>

      <Card className="gap-0 overflow-hidden py-0 shadow-sm">
        <CardHeader className="border-b py-6">
          <div className="flex items-start gap-3">
            <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary"><DatabaseZap /></span>
            <div>
              <CardTitle>Bulk editorial processing</CardTitle>
              <CardDescription className="mt-1 max-w-3xl">
                Run the complete editorial analysis workflow for one article, a date range, or the catalog. Each article is tracked independently with retries and an audit owner.
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent className="grid gap-6 p-5 sm:p-6 xl:grid-cols-[minmax(0,1fr)_minmax(18rem,0.42fr)]">
          <div className="grid min-w-0 gap-5 sm:grid-cols-2">
            <Field label="Scope" htmlFor="backfill-scope">
              <Select value={scope} onValueChange={(value) => { setScope(value as AnalysisBackfillScope); setSelectedArticle(null) }}>
                <SelectTrigger id="backfill-scope" className="w-full"><SelectValue>{scopeLabel(scope)}</SelectValue></SelectTrigger>
                <SelectContent>
                  <SelectItem value="date_range">Date range</SelectItem>
                  <SelectItem value="catalog">Entire catalog</SelectItem>
                  <SelectItem value="article">Individual article</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field label="Workflow" htmlFor="backfill-workflow">
              <Select value={workflow} onValueChange={(value) => setWorkflow(value as AnalysisBackfillWorkflow)}>
                <SelectTrigger id="backfill-workflow" className="w-full"><SelectValue>{workflowLabel(workflow)}</SelectValue></SelectTrigger>
                <SelectContent>
                  <SelectItem value="full_pipeline">Full editorial pipeline</SelectItem>
                  <SelectItem value="single_pass">Single-pass insight</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            {workflow === 'full_pipeline' ? (
              <div className="sm:col-span-2">
                <PipelineStages provider={provider} model={model} />
              </div>
            ) : null}
            <Field label="Provider" htmlFor="backfill-provider">
              <Select value={provider} onValueChange={(value) => { setProvider(value as AnalysisBackfillProvider); setSelectedModel('') }}>
                <SelectTrigger id="backfill-provider" className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="openrouter">OpenRouter (default)</SelectItem>
                  <SelectItem value="codex_cli">Codex CLI</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field label="Model" htmlFor="backfill-model">
              <Select value={model} onValueChange={(value) => setSelectedModel(value ?? '')} disabled={availableModels.length === 0}>
                <SelectTrigger id="backfill-model" className="w-full min-w-0"><SelectValue placeholder="No compatible models available" /></SelectTrigger>
                <SelectContent className="max-w-[calc(100vw-2rem)] sm:max-w-xl">
                  {availableModels.map((item) => <SelectItem key={item} value={item} className="max-w-full"><span className="block max-w-[70vw] truncate sm:max-w-lg">{modelName(item, models.data?.items)}</span></SelectItem>)}
                </SelectContent>
              </Select>
            </Field>

            {scope === 'date_range' ? (
              <>
                <Field label="From" htmlFor="backfill-from"><Input id="backfill-from" type="date" value={from} max={to} onChange={(event) => setFrom(event.target.value)} /></Field>
                <Field label="To" htmlFor="backfill-to"><Input id="backfill-to" type="date" value={to} min={from} onChange={(event) => setTo(event.target.value)} /></Field>
              </>
            ) : null}

            {scope === 'article' ? (
              <div className="sm:col-span-2">
                <Label htmlFor="article-search">Find an article</Label>
                <form className="mt-2 flex min-w-0 gap-2" onSubmit={(event) => { event.preventDefault(); setSubmittedSearch(articleSearch.trim()) }}>
                  <div className="relative min-w-0 flex-1">
                    <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input id="article-search" className="pl-9" value={articleSearch} onChange={(event) => setArticleSearch(event.target.value)} placeholder="Search headline or article ID…" />
                  </div>
                  <Button type="submit" variant="outline" disabled={articleSearch.trim().length < 2}>Search</Button>
                </form>
                <ArticleResults items={articles.data?.items ?? []} pending={articles.isFetching} selected={selectedArticle} onSelect={setSelectedArticle} />
              </div>
            ) : null}

            {scope === 'catalog' ? (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/5 p-4 sm:col-span-2">
                <div className="flex items-start gap-3">
                  <AlertTriangle className="mt-0.5 size-5 shrink-0 text-amber-600" />
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium">Full-catalog confirmation</p>
                    <p className="mt-1 text-xs text-muted-foreground">Type <strong>BACKFILL ENTIRE CATALOG</strong> to enable this exceptional operation.</p>
                    <Input className="mt-3" value={confirmation} onChange={(event) => setConfirmation(event.target.value)} placeholder="BACKFILL ENTIRE CATALOG" />
                  </div>
                </div>
              </div>
            ) : null}
          </div>

          <div className="flex min-w-0 flex-col justify-between gap-5 rounded-2xl border bg-muted/20 p-5">
            <div>
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">Eligible articles</p>
              {preview.isFetching ? <Skeleton className="mt-3 h-10 w-28" /> : <p className="mt-2 font-heading text-4xl font-semibold tabular-nums">{preview.data?.articles.toLocaleString() ?? '—'}</p>}
              <p className="mt-3 text-xs leading-5 text-muted-foreground">Only articles with current full content and an active source approval for AI processing are included.</p>
            </div>
            {!providerReady ? <InlineNotice message={`${provider === 'openrouter' ? 'OpenRouter' : 'Codex CLI'} is not ready.`} /> : null}
            {preview.isError ? <InlineNotice message="This scope cannot be previewed. Check the dates or selected article." /> : null}
            <Button
              size="lg"
              className="w-full"
              disabled={!model || !providerReady || !catalogConfirmed || !preview.data?.articles || start.isPending}
              onClick={() => start.mutate()}
            >
              {start.isPending ? <LoaderCircle className="animate-spin" /> : <DatabaseZap />}
              {start.isPending ? 'Creating jobs…' : workflow === 'full_pipeline' ? 'Run full pipeline' : 'Run single-pass analysis'}
            </Button>
          </div>
        </CardContent>
      </Card>

      <BackfillRuns
        runs={runs.data?.items ?? []}
        pending={runs.isPending}
        streamState={streamState}
        busyRunID={controlBackfill.isPending ? controlBackfill.variables?.id : deleteBackfill.isPending ? deleteBackfill.variables : undefined}
        onCancel={(run) => setDestructiveAction({ kind: 'cancel', run })}
        onDelete={(run) => setDestructiveAction({ kind: 'delete', run })}
      />
      <BackfillConfirmationDialog
        action={destructiveAction}
        pending={controlBackfill.isPending || deleteBackfill.isPending}
        onOpenChange={(open) => !open && setDestructiveAction(null)}
        onConfirm={() => {
          if (!destructiveAction) return
          if (destructiveAction.kind === 'cancel') {
            controlBackfill.mutate({ id: destructiveAction.run.id, action: 'cancel' })
          } else {
            deleteBackfill.mutate(destructiveAction.run.id)
          }
        }}
      />
    </section>
  )
}

function ProviderCard({ icon: Icon, name, description, ready, pending, detail, metadata }: {
  icon: typeof Bot; name: string; description: string; ready: boolean; pending: boolean; detail: string; metadata: string
}) {
  return (
    <Card className="gap-0 overflow-hidden py-0 shadow-sm">
      <CardContent className="flex min-h-44 flex-col justify-between gap-5 p-5 sm:p-6">
        <div className="flex items-start gap-4">
          <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary"><Icon /></span>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="font-heading font-semibold">{name}</p>
              {pending ? <Badge variant="outline"><LoaderCircle className="animate-spin" /> Checking</Badge> : <HealthBadge ready={ready} />}
            </div>
            <p className="mt-1 text-sm text-muted-foreground">{description}</p>
          </div>
        </div>
        <div className="border-t pt-4">
          <p className="text-sm">{detail}</p>
          {metadata ? <p className="mt-1 break-all text-xs text-muted-foreground">{metadata}</p> : null}
        </div>
      </CardContent>
    </Card>
  )
}

function HealthBadge({ ready }: { ready: boolean }) {
  return (
    <Badge variant="outline" className={cn(ready ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400' : 'border-destructive/30 bg-destructive/10 text-destructive')}>
      {ready ? <CheckCircle2 /> : <CircleAlert />}{ready ? 'Ready' : 'Unavailable'}
    </Badge>
  )
}

function Field({ label, htmlFor, children }: { label: string; htmlFor: string; children: ReactNode }) {
  return <div className="min-w-0"><Label htmlFor={htmlFor}>{label}</Label><div className="mt-2">{children}</div></div>
}

function PipelineStages({ provider, model }: { provider: AnalysisBackfillProvider; model: string }) {
  const stages = ['Clean', 'Summarize', 'Categorize', 'Cluster', 'Evaluate stance', 'Synthesize event']
  return (
    <div className="rounded-xl border bg-muted/20 p-4">
      <div className="flex items-center gap-2">
        <Sparkles className="size-4 text-primary" />
        <p className="text-sm font-medium">Full editorial pipeline</p>
      </div>
      <p className="mt-1 text-xs leading-5 text-muted-foreground">
        Pins {providerLabel(provider)}{model ? ` · ${model}` : ''} to every AI-backed stage and refreshes cross-source event analysis as articles complete.
      </p>
      <div className="mt-4 grid gap-2 sm:grid-cols-3">
        {stages.map((stage, index) => (
          <div key={stage} className="flex items-center gap-2 rounded-lg border bg-background px-3 py-2 text-xs font-medium">
            <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-primary/10 text-[10px] text-primary">{index + 1}</span>
            {stage}
          </div>
        ))}
      </div>
    </div>
  )
}

function ArticleResults({ items, pending, selected, onSelect }: {
  items: AdminArticleListItem[]; pending: boolean; selected: AdminArticleListItem | null; onSelect: (article: AdminArticleListItem) => void
}) {
  if (pending) return <div className="mt-3 grid gap-2"><Skeleton className="h-16" /><Skeleton className="h-16" /></div>
  if (items.length === 0) return selected ? <SelectedArticle article={selected} /> : null
  return (
    <div className="mt-3 grid gap-2">
      {items.map((article) => (
        <button key={article.id} type="button" onClick={() => onSelect(article)} className={cn('flex min-w-0 items-start gap-3 rounded-xl border p-3 text-left transition-colors hover:bg-muted/50', selected?.id === article.id && 'border-primary bg-primary/5 ring-1 ring-primary')}>
          <FileText className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <span className="min-w-0"><span className="line-clamp-2 text-sm font-medium">{article.headline}</span><span className="mt-1 block text-xs text-muted-foreground">{article.source} · {dateTime.format(new Date(article.published_at))}</span></span>
        </button>
      ))}
    </div>
  )
}

function SelectedArticle({ article }: { article: AdminArticleListItem }) {
  return <div className="mt-3 rounded-xl border border-primary/30 bg-primary/5 p-3 text-sm font-medium">{article.headline}</div>
}

function BackfillRuns({ runs, pending, streamState, busyRunID, onCancel, onDelete }: {
  runs: AnalysisBackfillRun[]
  pending: boolean
  streamState: BackfillStreamState
  busyRunID?: string
  onCancel: (run: AnalysisBackfillRun) => void
  onDelete: (run: AnalysisBackfillRun) => void
}) {
  return (
    <Card className="gap-0 overflow-hidden py-0 shadow-sm">
      <CardHeader className="border-b py-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <CardTitle>Recent administrative backfills</CardTitle>
            <CardDescription>Live progress with controls for stopping bulk work that can be run again later.</CardDescription>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <BackfillConnectionStatus state={streamState} />
            <Button variant="outline" nativeButton={false} render={<Link to="/jobs?kind=admin.article.analysis&window=7d" />}><ExternalLink /> Open queue monitor</Button>
          </div>
        </div>
      </CardHeader>
      <CardContent className="p-0">
        {pending ? <div className="grid gap-3 p-6"><Skeleton className="h-24" /><Skeleton className="h-24" /></div> : null}
        {!pending && runs.length === 0 ? <div className="p-8 text-center text-sm text-muted-foreground">No administrative analysis backfills have been created.</div> : null}
        {runs.map((run) => (
          <BackfillRunRow
            key={run.id}
            run={run}
            busy={busyRunID === run.id}
            onCancel={onCancel}
            onDelete={onDelete}
          />
        ))}
      </CardContent>
    </Card>
  )
}

function BackfillConnectionStatus({ state }: { state: BackfillStreamState }) {
  const live = state === 'live'
  const Icon = live ? Wifi : WifiOff
  return (
    <Badge variant="outline" className={cn(live ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400' : 'text-muted-foreground')}>
      <Icon /> {live ? 'Live' : state === 'connecting' ? 'Connecting' : 'Reconnecting'}
    </Badge>
  )
}

function BackfillRunRow({ run, busy, onCancel, onDelete }: {
  run: AnalysisBackfillRun
  busy: boolean
  onCancel: (run: AnalysisBackfillRun) => void
  onDelete: (run: AnalysisBackfillRun) => void
}) {
  const terminal = run.succeeded_articles + run.failed_articles + run.cancelled_articles
  const percent = run.total_articles > 0 ? Math.min(100, Math.round((terminal / run.total_articles) * 100)) : 0
  const remaining = run.pending_articles + run.queued_articles + run.running_articles
  const deletable = ['completed', 'partially_completed', 'failed', 'cancelled'].includes(run.status)
  return (
    <div className="grid grid-cols-1 gap-x-4 gap-y-4 border-b p-5 last:border-b-0 sm:p-6 xl:grid-cols-[minmax(0,1fr)_minmax(18rem,0.7fr)_2.25rem] xl:items-center">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2"><BackfillStatus status={run.status} /><Badge variant="secondary">{workflowLabel(run.workflow)}</Badge><Badge variant="outline">{scopeLabel(run.scope)}</Badge></div>
        <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2">
          <p className="min-w-0 truncate font-medium" title={run.model}>{`${providerLabel(run.provider)} · ${run.model}`}</p>
          <BackfillRunControl run={run} busy={busy} onCancel={onCancel} />
        </div>
        <p className="mt-1 text-xs text-muted-foreground">Created by {run.created_by} · {dateTime.format(new Date(run.created_at))}</p>
        {run.error_detail ? <p className="mt-2 text-xs text-destructive">{run.error_detail}</p> : null}
      </div>
      <div className="min-w-0 xl:col-start-2 xl:row-start-1">
        <div className="flex items-center justify-between gap-3 text-xs"><span className="text-muted-foreground">{terminal.toLocaleString()} of {run.total_articles.toLocaleString()} finished</span><span className="font-medium tabular-nums">{percent}%</span></div>
        <div className="mt-2 h-2 overflow-hidden rounded-full bg-muted" role="progressbar" aria-valuenow={percent} aria-valuemin={0} aria-valuemax={100}><div className="h-full rounded-full bg-primary transition-[width]" style={{ width: `${percent}%` }} /></div>
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
          <span>{run.succeeded_articles.toLocaleString()} succeeded</span>
          <span>{run.failed_articles.toLocaleString()} failed</span>
          {run.cancelled_articles > 0 ? <span>{run.cancelled_articles.toLocaleString()} cancelled</span> : null}
          <span>{remaining.toLocaleString()} remaining</span>
          {(run.status === 'queued' || run.status === 'running') && (run.queued_articles > 0 || run.running_articles > 0) ? <span>{run.running_articles.toLocaleString()} active · {run.queued_articles.toLocaleString()} queued</span> : null}
        </div>
      </div>
      {deletable ? (
        <div className="flex min-h-8 items-center justify-end xl:col-start-3 xl:row-start-1">
          <BackfillRunActions run={run} busy={busy} onDelete={onDelete} />
        </div>
      ) : null}
    </div>
  )
}

function BackfillRunControl({ run, busy, onCancel }: {
  run: AnalysisBackfillRun
  busy: boolean
  onCancel: (run: AnalysisBackfillRun) => void
}) {
  const stoppable = run.status === 'queued' || run.status === 'running' || run.status === 'paused'
  if (!stoppable) return null
  return (
    <Button variant="outline" size="sm" disabled={busy} onClick={() => onCancel(run)}>
      {busy ? <LoaderCircle className="animate-spin" /> : <Square />} Stop
    </Button>
  )
}

function BackfillRunActions({ run, busy, onDelete }: {
  run: AnalysisBackfillRun
  busy: boolean
  onDelete: (run: AnalysisBackfillRun) => void
}) {
  const deletable = ['completed', 'partially_completed', 'failed', 'cancelled'].includes(run.status)
  if (!deletable) return null
  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="ghost" size="icon-sm" aria-label={`Actions for backfill ${run.id}`} disabled={busy} />}>
        {busy ? <LoaderCircle className="animate-spin" /> : <EllipsisVertical />}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        <DropdownMenuItem variant="destructive" onClick={() => onDelete(run)}><Trash2 /> Delete record</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function BackfillConfirmationDialog({ action, pending, onOpenChange, onConfirm }: {
  action: DestructiveBackfillAction | null
  pending: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
}) {
  const deleting = action?.kind === 'delete'
  return (
    <Dialog open={Boolean(action)} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={!pending}>
        <DialogHeader>
          <DialogTitle>{deleting ? 'Delete backfill record?' : 'Stop this backfill?'}</DialogTitle>
          <DialogDescription>
            {deleting
              ? 'This removes the administrative history record. Article content and analysis already produced by the backfill will be preserved.'
              : 'All unfinished articles will be cancelled and this run cannot be resumed. Analysis that already completed will be preserved.'}
          </DialogDescription>
        </DialogHeader>
        {action ? (
          <div className="rounded-2xl border bg-muted/20 p-4">
            <p className="truncate font-medium" title={action.run.model}>{providerLabel(action.run.provider)} · {action.run.model}</p>
            <p className="mt-1 text-xs text-muted-foreground">{action.run.total_articles.toLocaleString()} articles · created {dateTime.format(new Date(action.run.created_at))}</p>
          </div>
        ) : null}
        <DialogFooter>
          <Button variant="outline" disabled={pending} onClick={() => onOpenChange(false)}>Keep record</Button>
          <Button variant="destructive" disabled={pending} onClick={onConfirm}>
            {pending ? <LoaderCircle className="animate-spin" /> : deleting ? <Trash2 /> : <Square />}
            {pending ? 'Working…' : deleting ? 'Delete record' : 'Stop backfill'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function BackfillStatus({ status }: { status: AnalysisBackfillRun['status'] }) {
  const data = {
    queued: { label: 'Queued', icon: Clock3, className: 'border-slate-400/30 text-slate-600 dark:text-slate-300' },
    running: { label: 'Running', icon: LoaderCircle, className: 'border-blue-500/30 bg-blue-500/10 text-blue-700 dark:text-blue-400' },
    paused: { label: 'Paused', icon: Pause, className: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400' },
    completed: { label: 'Completed', icon: CheckCircle2, className: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400' },
    partially_completed: { label: 'Partial', icon: AlertTriangle, className: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400' },
    failed: { label: 'Failed', icon: CircleAlert, className: 'border-destructive/30 bg-destructive/10 text-destructive' },
    cancelled: { label: 'Stopped', icon: Square, className: 'border-slate-400/30 bg-slate-500/10 text-slate-600 dark:text-slate-300' },
  }[status]
  const Icon = data.icon
  return <Badge variant="outline" className={data.className}><Icon className={cn(status === 'running' && 'animate-spin')} />{data.label}</Badge>
}

function InlineNotice({ message }: { message: string }) {
  return <div className="flex items-start gap-2 rounded-lg border border-destructive/20 bg-destructive/5 p-3 text-xs text-destructive"><CircleAlert className="mt-0.5 size-4 shrink-0" />{message}</div>
}

function providerLabel(provider: AnalysisBackfillProvider) { return provider === 'pipeline' ? 'Editorial pipeline' : provider === 'codex_cli' ? 'Codex CLI' : 'OpenRouter' }
function workflowLabel(workflow: AnalysisBackfillRun['workflow']) { return workflow === 'full_pipeline' ? 'Full pipeline' : 'Single pass' }
function scopeLabel(scope: AnalysisBackfillScope) { return scope === 'catalog' ? 'Entire catalog' : scope === 'article' ? 'Single article' : 'Date range' }
function authLabel(value: string) { return value === 'chatgpt' ? 'ChatGPT authentication' : value === 'api_key' ? 'API key authentication' : 'Authenticated' }
function modelName(id: string, models: { id: string; name: string }[] | undefined) { return models?.find((model) => model.id === id)?.name ?? id }
function daysAgo(days: number) { const value = new Date(); value.setDate(value.getDate() - days); return value }
function dateInput(value: Date) { return value.toISOString().slice(0, 10) }
