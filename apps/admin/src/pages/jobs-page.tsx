import {
  createClient,
  type CronJobHealth,
  type CronJobStatistic,
  type CronMonitor,
  type PipelineStep,
  type QueueJob,
  type QueueJobArtifact,
  type QueueJobArtifacts,
  type QueueJobStatus,
  type QueueMonitorSnapshot,
} from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import {
  Activity,
  Braces,
  CalendarClock,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Circle,
  Clock3,
  Database,
  ExternalLink,
  FileInput,
  FileOutput,
  FileText,
  Inbox,
  ListTree,
  LoaderCircle,
  Newspaper,
  OctagonX,
  RefreshCw,
  RotateCcw,
  ServerCog,
  TriangleAlert,
} from 'lucide-react'
import { lazy, startTransition, Suspense, useEffect, useId, useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
import { SourceAvatar } from '@/components/source-avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useTableQuery } from '@/hooks/use-table-query'
import { cn } from '@/lib/utils'

const client = createClient()
const date = new Intl.DateTimeFormat('en', { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' })
const LazyRichArticleContent = lazy(async () => {
  const module = await import('@/components/rich-article-content')
  return { default: module.RichArticleContent }
})

const statuses: Record<QueueJobStatus, { label: string; icon: typeof Circle; className: string }> = {
  queued: { label: 'Queued', icon: Clock3, className: 'border-slate-300 bg-slate-50 text-slate-700 dark:bg-slate-900' },
  processing: { label: 'Processing', icon: LoaderCircle, className: 'border-blue-300 bg-blue-50 text-blue-700 dark:bg-blue-950' },
  completed: { label: 'Completed', icon: CheckCircle2, className: 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:bg-emerald-950' },
  partially_completed: { label: 'Partially completed', icon: TriangleAlert, className: 'border-amber-300 bg-amber-50 text-amber-700 dark:bg-amber-950' },
  failed: { label: 'Failed', icon: OctagonX, className: 'border-red-300 bg-red-50 text-red-700 dark:bg-red-950' },
}

const cronHealth: Record<CronJobHealth, { label: string; icon: typeof Circle; className: string }> = {
  healthy: { label: 'Healthy', icon: CheckCircle2, className: 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:bg-emerald-950' },
  running: { label: 'Running', icon: LoaderCircle, className: 'border-blue-300 bg-blue-50 text-blue-700 dark:bg-blue-950' },
  degraded: { label: 'Degraded', icon: TriangleAlert, className: 'border-amber-300 bg-amber-50 text-amber-700 dark:bg-amber-950' },
  failed: { label: 'Failed', icon: OctagonX, className: 'border-red-300 bg-red-50 text-red-700 dark:bg-red-950' },
  overdue: { label: 'Overdue', icon: Clock3, className: 'border-orange-300 bg-orange-50 text-orange-700 dark:bg-orange-950' },
  unknown: { label: 'Unknown', icon: Circle, className: 'border-slate-300 bg-slate-50 text-slate-700 dark:bg-slate-900' },
}

const stepLabels: Record<string, string> = {
  categorization: 'Categorization',
  event_clustering: 'Event clustering',
  narration_analysis: 'Narration analysis',
}

const kindLabels: Record<string, string> = {
  'article.pipeline': 'Article workflow',
  'article.content': 'Full article retrieval',
	'article.content.backfill': 'Full article backfill',
	'article.content.cleanup': 'Article content retention',
  'article.pipeline.dispatch': 'Pipeline dispatch',
  'admin.analysis.backfill.dispatch': 'Administrative AI backfill',
  'admin.article.analysis': 'Administrative article analysis',
  'ingest.poll': 'Source polling',
  'brief.daily': 'Daily brief',
  'newsletter.daily': 'Morning newsletter',
  'intelligence.narration': 'Narration sweep',
  'queue.history.cleanup': 'Queue history cleanup',
}

export function JobsPage() {
  const table = useTableQuery()
  const queryClient = useQueryClient()
  const status = table.filter('status')
  const queue = table.filter('queue')
  const kindFilter = table.filter('kind')
  const kind = kindFilter === 'all' ? '' : kindFilter || 'article.pipeline'
  const window = table.filter('window') || '7d'
  const [selectedID, setSelectedID] = useState<string | null>(null)
  const jobs = useQuery({
    queryKey: ['queue-jobs', table.page, table.perPage, table.search, status, queue, kind, window],
    queryFn: () => client.queueJobs({
      page: table.page,
      per_page: table.perPage,
      search: table.search,
      status,
      queue,
      kind,
      window,
    }),
    placeholderData: keepPreviousData,
    enabled: false,
    refetchOnWindowFocus: false,
    staleTime: Number.POSITIVE_INFINITY,
  })
  const cron = useQuery({
    queryKey: ['cron-jobs'],
    queryFn: () => client.cronJobs(),
    enabled: false,
    refetchOnWindowFocus: false,
    staleTime: Number.POSITIVE_INFINITY,
  })
  const streamStatus = useQueueMonitorStream({
    page: table.page,
    perPage: table.perPage,
    search: table.search,
    status,
    queue,
    kind,
    window,
    queryClient,
  })
  const selected = jobs.data?.items.find((job) => job.id === selectedID)
  const artifacts = useQuery({
    queryKey: ['queue-job-artifacts', selectedID],
    queryFn: () => client.queueJobArtifacts(selectedID!),
    enabled: Boolean(selectedID),
    refetchOnWindowFocus: false,
    staleTime: 30_000,
  })
  const failedStep = selected?.steps.find((step) => step.status === 'failed')
  const retry = useMutation({
    mutationFn: async () => {
      if (!selected?.article_id || !failedStep) throw new Error('No failed step is available')
      return client.runArticlePipeline(selected.article_id, failedStep.name)
    },
    onSuccess: () => {
      toast.success('Failed step queued for retry')
      setSelectedID(null)
      void queryClient.invalidateQueries({ queryKey: ['queue-jobs'] })
    },
    onError: () => toast.error('Could not retry the failed step'),
  })
  const items = jobs.data?.items ?? []

  return (
    <section className="flex min-w-0 flex-col gap-5">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Queue monitor</h1>
          <p className="mt-1 text-sm text-muted-foreground">Pipeline workflows from the last seven days. Include routine jobs with the filters.</p>
        </div>
        <div className="flex shrink-0 items-center gap-3">
          <LiveConnectionStatus status={streamStatus} />
          <Button variant="outline" size="icon" aria-label="Refresh queue" disabled={jobs.isFetching || cron.isFetching} onClick={() => void Promise.all([jobs.refetch(), cron.refetch()])}>
            <RefreshCw className={cn((jobs.isFetching || cron.isFetching) && 'animate-spin')} />
          </Button>
        </div>
      </div>

      <CronMonitorPanel monitor={cron.data} pending={cron.isPending} error={cron.isError} />

      <TelemetryStrip
        summary={jobs.data?.summary}
        selected={status}
        onSelect={(next) => table.setFilter('status', next === status ? '' : next)}
      />

      <Card className="gap-0 overflow-hidden py-0 shadow-sm">
        <CardContent className="px-0">
          <DataTableToolbar search={table.search} searchPlaceholder="Search jobs…" onSearch={table.setSearch}>
            <QueueSelect label="status" value={status} options={Object.entries(statuses).map(([value, item]) => [value, item.label])} onChange={(value) => table.setFilter('status', value)} />
            <QueueSelect label="queue" value={queue} options={[["admin-analysis", "Admin analysis queue"], ["analysis", "Analysis queue"], ["crawl", "Crawl queue"], ["default", "Default queue"]]} onChange={(value) => table.setFilter('queue', value)} />
            <QueueSelect label="job type" value={kind} options={Object.entries(kindLabels)} onChange={(value) => table.setFilter('kind', value || 'all')} />
            <QueueSelect label="time" value={window} options={[["24h", "Last 24 hours"], ["7d", "Last 7 days"]]} includeAll={false} onChange={(value) => table.setFilter('window', value)} />
          </DataTableToolbar>

          <div className="hidden @5xl/main:block">
            <Table className="table-fixed min-w-[1020px]">
              <TableHeader className="bg-muted/30">
                <TableRow>
                  <TableHead className="w-[28%]">Workflow</TableHead>
                  <TableHead className="w-[14%]">Overall status</TableHead>
                  <TableHead className="w-[22%]">Step progress</TableHead>
                  <TableHead className="w-[15%]">Queue / attempt</TableHead>
                  <TableHead className="w-[11%]">Started</TableHead>
                  <TableHead className="w-[6%]">Duration</TableHead>
                  <TableHead className="w-[8%] text-right">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {jobs.isPending ? <LoadingRows /> : null}
                {jobs.isError ? <MessageRow message="Queue telemetry is temporarily unavailable." /> : null}
                {!jobs.isPending && !jobs.isError && items.length === 0 ? <MessageRow message="No jobs match these filters." empty /> : null}
                {!jobs.isPending && !jobs.isError ? items.map((job) => (
                  <TableRow key={job.id} data-state={selectedID === job.id ? 'selected' : undefined}>
                    <TableCell className="whitespace-normal">
                      <JobTitle job={job} />
                    </TableCell>
                    <TableCell><StatusBadge status={job.status} /></TableCell>
                    <TableCell><StepRail job={job} /></TableCell>
                    <TableCell><QueueAttempt job={job} /></TableCell>
                    <TableCell className="text-muted-foreground"><JobStarted job={job} /></TableCell>
                    <TableCell className="tabular-nums text-muted-foreground">{formatDuration(job.duration_ms)}</TableCell>
                    <TableCell className="text-right"><Button variant="outline" size="sm" onClick={() => setSelectedID(job.id)}>Inspect</Button></TableCell>
                  </TableRow>
                )) : null}
              </TableBody>
            </Table>
          </div>

          <div className="divide-y @5xl/main:hidden">
            {jobs.isPending ? <div className="space-y-3 p-5"><Skeleton className="h-5 w-3/4" /><Skeleton className="h-16 w-full" /></div> : null}
            {jobs.isError ? <p className="p-8 text-center text-sm text-muted-foreground">Queue telemetry is temporarily unavailable.</p> : null}
            {!jobs.isPending && !jobs.isError && items.length === 0 ? <p className="p-8 text-center text-sm text-muted-foreground">No jobs match these filters.</p> : null}
            {!jobs.isPending && !jobs.isError ? items.map((job) => (
              <article key={job.id} className="space-y-4 p-5">
                <div className="flex items-start justify-between gap-3"><JobTitle job={job} /><StatusBadge status={job.status} /></div>
                <StepRail job={job} />
                <div className="flex items-center justify-between gap-3"><QueueAttempt job={job} /><Button variant="outline" onClick={() => setSelectedID(job.id)}>Inspect</Button></div>
              </article>
            )) : null}
          </div>

          {jobs.data ? <DataTablePagination pagination={jobs.data.pagination} pageHref={table.pageHref} onPerPageChange={table.setPerPage} /> : null}
        </CardContent>
      </Card>

      <JobInspector
        job={selected}
        artifacts={artifacts.data}
        artifactsPending={artifacts.isPending}
        artifactsError={artifacts.isError}
        failedStep={failedStep}
        retrying={retry.isPending}
        onClose={() => setSelectedID(null)}
        onRetry={() => retry.mutate()}
        onRetryArtifacts={() => void artifacts.refetch()}
      />
    </section>
  )
}

type MonitorConnectionStatus = 'connecting' | 'live' | 'reconnecting'

function useQueueMonitorStream({
  page,
  perPage,
  search,
  status,
  queue,
  kind,
  window,
  queryClient,
}: {
  page: number
  perPage: number
  search: string
  status: string
  queue: string
  kind: string
  window: string
  queryClient: QueryClient
}) {
  const [connection, setConnection] = useState<MonitorConnectionStatus>('connecting')

  useEffect(() => {
    setConnection('connecting')
    const queryKey = ['queue-jobs', page, perPage, search, status, queue, kind, window]
    const events = new EventSource(client.queueMonitorStreamURL({
      page,
      per_page: perPage,
      search,
      status,
      queue,
      kind,
      window,
    }), { withCredentials: true })
    const receiveMonitor = (event: MessageEvent<string>) => {
      try {
        const snapshot = JSON.parse(event.data) as QueueMonitorSnapshot
        if (!snapshot.queue || !snapshot.cron) {
          setConnection('reconnecting')
          return
        }
        startTransition(() => {
          queryClient.setQueryData(queryKey, snapshot.queue)
          queryClient.setQueryData(['cron-jobs'], snapshot.cron)
          setConnection('live')
        })
      } catch {
        setConnection('reconnecting')
      }
    }
    events.onopen = () => setConnection('live')
    events.onerror = () => setConnection('reconnecting')
    events.addEventListener('monitor', receiveMonitor as EventListener)
    events.addEventListener('monitor-error', () => setConnection('reconnecting'))

    return () => {
      events.close()
    }
  }, [kind, page, perPage, queryClient, queue, search, status, window])

  return connection
}

function LiveConnectionStatus({ status }: { status: MonitorConnectionStatus }) {
  const live = status === 'live'
  return (
    <span className="hidden min-w-24 items-center justify-end gap-2 text-xs text-muted-foreground sm:inline-flex" aria-live="polite">
      <span className={cn('size-1.5 rounded-full', live ? 'bg-emerald-500' : 'bg-amber-500')} />
      {live ? 'Live updates' : status === 'connecting' ? 'Connecting' : 'Reconnecting'}
    </span>
  )
}

function CronMonitorPanel({ monitor, pending, error }: { monitor?: CronMonitor; pending: boolean; error: boolean }) {
  const [expanded, setExpanded] = useState(false)
  const detailsID = useId()
  const worker = monitor?.worker
  const workerState = worker?.status ?? 'offline'
  return (
    <Card className="@container/cron gap-0 overflow-hidden py-0 shadow-sm">
      <CardHeader className="p-0">
        <button
          type="button"
          aria-expanded={expanded}
          aria-controls={detailsID}
          className="grid min-h-28 w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-4 px-5 py-4 text-left transition-colors hover:bg-muted/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset @4xl/cron:min-h-24 @4xl/cron:grid-cols-[minmax(0,1fr)_auto_auto]"
          onClick={() => setExpanded((current) => !current)}
        >
          <div className="min-w-0">
            <CardTitle className="flex items-center gap-2"><CalendarClock className="size-4 shrink-0 text-primary" />Cron workers</CardTitle>
            <CardDescription className="mt-1 line-clamp-2">Recurring schedules, scheduler health, and execution reliability over the last 24 hours.</CardDescription>
          </div>
          <ChevronDown className={cn('size-5 shrink-0 text-muted-foreground transition-transform @4xl/cron:col-start-3 @4xl/cron:row-start-1', expanded && 'rotate-180')} aria-hidden />
          <div className="col-span-full flex min-w-0 flex-wrap items-center gap-x-3 gap-y-2 @4xl/cron:col-span-1 @4xl/cron:col-start-2 @4xl/cron:row-start-1 @4xl/cron:justify-end">
            <WorkerStatus status={pending ? 'checking' : workerState} />
            <SummaryBadge icon={Activity} value={monitor?.summary.running} label="running" />
            <SummaryBadge icon={CheckCircle2} value={monitor?.summary.healthy} label="healthy" />
            <SummaryBadge icon={TriangleAlert} value={monitor?.summary.attention} label="attention" attention={Boolean(monitor?.summary.attention)} />
          </div>
        </button>
      </CardHeader>
      {expanded ? (
        <CardContent id={detailsID} className="border-t px-0">
          {worker ? (
            <div className="flex min-h-11 flex-wrap items-center gap-x-4 gap-y-1 border-b bg-muted/15 px-5 py-3 text-xs text-muted-foreground">
              <span className="inline-flex min-w-0 items-center gap-1.5" title={worker.leader_id ?? undefined}><ServerCog className="size-3.5 shrink-0" /><span className="truncate">Leader {shortWorkerID(worker.leader_id)}</span></span>
              <span className="whitespace-nowrap">{worker.max_concurrency} max concurrent workers</span>
              {worker.queues.map((queue) => <span key={queue.name} className={cn('whitespace-nowrap', queue.paused && 'text-amber-600')}>{queue.name}: {queue.max_workers}{queue.paused ? ' · paused' : ''}</span>)}
            </div>
          ) : null}
          {pending ? <CronLoading /> : null}
          {error ? <p className="min-h-40 p-8 text-center text-sm text-muted-foreground">Cron telemetry is temporarily unavailable.</p> : null}
          {!pending && !error ? (
            <>
              <div className="hidden @5xl/main:block">
                <Table className="table-fixed min-w-[1020px]">
                <TableHeader className="bg-muted/30">
                  <TableRow>
                    <TableHead className="w-[25%]">Scheduled job</TableHead>
                    <TableHead className="w-[11%]">Health</TableHead>
                    <TableHead className="w-[13%]">Frequency</TableHead>
                    <TableHead className="w-[14%]">Last run</TableHead>
                    <TableHead className="w-[14%]">Next expected</TableHead>
                    <TableHead className="w-[14%]">24h reliability</TableHead>
                    <TableHead className="w-[9%]">Runtime</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {(monitor?.items ?? []).map((job) => <CronJobRow key={job.kind} job={job} />)}
                </TableBody>
                </Table>
              </div>
              <div className="grid @3xl/main:grid-cols-2 @5xl/main:hidden">
                {(monitor?.items ?? []).map((job) => <CronJobCard key={job.kind} job={job} />)}
              </div>
            </>
          ) : null}
        </CardContent>
      ) : null}
    </Card>
  )
}

function CronJobRow({ job }: { job: CronJobStatistic }) {
  return (
    <TableRow>
      <TableCell className="whitespace-normal">
        <p className="font-medium">{job.name}</p>
        <p className="mt-1 line-clamp-1 text-xs text-muted-foreground">{job.description}</p>
        <p className="mt-1 font-mono text-[10px] text-muted-foreground/80">{job.kind} · {job.queue}</p>
      </TableCell>
      <TableCell><CronHealthBadge job={job} /><p className="mt-1 text-xs capitalize text-muted-foreground">{job.currently_running ? `${job.currently_running} active` : job.state.replaceAll('_', ' ')}</p></TableCell>
      <TableCell><p className="text-xs font-medium">{formatInterval(job.interval_seconds)}</p><p className="mt-1 text-xs text-muted-foreground">{job.run_on_start ? 'Runs on worker start' : 'Scheduled only'}</p></TableCell>
      <TableCell><DateMetric value={job.last_run_at} /></TableCell>
      <TableCell><NextRunMetric value={job.next_run_at} /></TableCell>
      <TableCell><ReliabilityMetric job={job} /></TableCell>
      <TableCell><p className="text-xs font-medium tabular-nums">{formatDuration(job.last_duration_ms)}</p><p className="mt-1 text-xs tabular-nums text-muted-foreground">Avg {formatDuration(job.average_duration_ms)}</p></TableCell>
    </TableRow>
  )
}

function CronJobCard({ job }: { job: CronJobStatistic }) {
  return (
    <article className="min-w-0 space-y-4 border-b p-5 @3xl/main:[&:nth-child(odd)]:border-r">
      <div className="flex items-start justify-between gap-3"><div className="min-w-0"><p className="font-medium">{job.name}</p><p className="mt-1 text-xs text-muted-foreground">{job.description}</p></div><span className="shrink-0"><CronHealthBadge job={job} /></span></div>
      <dl className="grid grid-cols-2 gap-4 text-xs">
        <CronDetail label="Frequency" value={formatInterval(job.interval_seconds)} />
        <CronDetail label="Current state" value={job.currently_running ? `${job.currently_running} running` : job.state.replaceAll('_', ' ')} />
        <CronDetail label="Last run" value={relativeTime(job.last_run_at)} />
        <CronDetail label="Next expected" value={relativeTime(job.next_run_at, true)} />
        <CronDetail label="24h reliability" value={formatReliability(job)} />
        <CronDetail label="Last / average" value={`${formatDuration(job.last_duration_ms)} / ${formatDuration(job.average_duration_ms)}`} />
      </dl>
      <p className="font-mono text-[10px] text-muted-foreground">{job.kind} · {job.queue}{job.worker_id ? ` · ${shortWorkerID(job.worker_id)}` : ''}</p>
    </article>
  )
}

function CronHealthBadge({ job }: { job: CronJobStatistic }) {
  const meta = cronHealth[job.health]
  const Icon = meta.icon
  const badge = <Badge variant="outline" className={meta.className}><Icon className={cn(job.health === 'running' && 'animate-spin')} />{meta.label}</Badge>
  if (!job.last_error) return badge
  return <Tooltip><TooltipTrigger render={<span className="inline-flex" />}>{badge}</TooltipTrigger><TooltipContent className="max-w-sm">{job.last_error}</TooltipContent></Tooltip>
}

function WorkerStatus({ status }: { status: 'online' | 'stale' | 'offline' | 'checking' }) {
  const online = status === 'online'
  const warning = status === 'checking'
  return (
    <span className={cn('inline-flex min-w-28 items-center gap-2 whitespace-nowrap text-xs font-medium', online ? 'text-emerald-700 dark:text-emerald-400' : warning ? 'text-muted-foreground' : 'text-red-700 dark:text-red-400')}>
      <span className="relative flex size-2 shrink-0" aria-hidden>
        {online ? <span className="absolute inline-flex size-full animate-ping rounded-full bg-emerald-400 opacity-70 motion-reduce:hidden" /> : null}
        <span className={cn('relative inline-flex size-2 rounded-full', online ? 'bg-emerald-500' : warning ? 'bg-muted-foreground/60' : 'bg-red-500')} />
      </span>
      Worker {status}
    </span>
  )
}

function SummaryBadge({ icon: Icon, value, label, attention = false }: { icon: typeof Activity; value?: number; label: string; attention?: boolean }) {
  return <Badge variant="outline" className={cn('min-w-[5.75rem] justify-center bg-muted/30 tabular-nums', attention && 'border-amber-400/60 text-amber-700 dark:text-amber-400')}><Icon />{value ?? '—'} {label}</Badge>
}

function DateMetric({ value }: { value: string | null }) {
  if (!value) return <span className="text-xs text-muted-foreground">Never</span>
  return <div className="text-xs"><p className="whitespace-nowrap font-medium">{date.format(new Date(value))}</p><p className="mt-1 text-muted-foreground">{relativeTime(value)}</p></div>
}

function NextRunMetric({ value }: { value: string | null }) {
  if (!value) return <span className="text-xs text-muted-foreground">Not scheduled yet</span>
  return <div className="text-xs"><p className="font-medium">{relativeTime(value, true)}</p><p className="mt-1 whitespace-nowrap text-muted-foreground">{date.format(new Date(value))}</p></div>
}

function ReliabilityMetric({ job }: { job: CronJobStatistic }) {
  return <div className="text-xs"><p className="font-medium tabular-nums">{job.success_rate_24h == null ? '—' : `${Math.round(job.success_rate_24h * 100)}%`}</p><p className="mt-1 tabular-nums text-muted-foreground">{job.runs_24h} runs · {job.failed_runs_24h} failed</p></div>
}

function CronDetail({ label, value }: { label: string; value: string }) {
  return <div className="min-w-0"><dt className="text-muted-foreground">{label}</dt><dd className="mt-1 font-medium tabular-nums">{value}</dd></div>
}

function CronLoading() {
  return <div className="space-y-3 p-5">{Array.from({ length: 4 }, (_, index) => <div key={index} className="grid grid-cols-3 gap-4"><Skeleton className="h-8" /><Skeleton className="h-8" /><Skeleton className="h-8" /></div>)}</div>
}

function TelemetryStrip({ summary, selected, onSelect }: { summary?: Record<string, number>; selected: string; onSelect: (status: string) => void }) {
  return <div className="flex snap-x overflow-x-auto rounded-xl border bg-card shadow-xs">{(Object.keys(statuses) as QueueJobStatus[]).map((status, index) => {
    const meta = statuses[status]
    const Icon = meta.icon
    return <button key={status} className={cn('flex min-w-44 flex-1 snap-start items-center gap-3 border-r px-4 py-4 text-left last:border-r-0 hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset', selected === status && 'bg-muted/50')} onClick={() => onSelect(status)}><span className={cn('flex size-8 items-center justify-center rounded-full border', meta.className)}><Icon className={cn('size-4', status === 'processing' && 'animate-spin')} /></span><span><span className="block text-xs text-muted-foreground">{meta.label}</span><span className="block text-lg font-semibold tabular-nums">{summary?.[status] ?? 0}</span></span>{index === 0 && summary ? <span className="sr-only">of {summary.total} total jobs</span> : null}</button>
  })}</div>
}

function QueueSelect({ label, value, options, includeAll = true, onChange }: { label: string; value: string; options: string[][]; includeAll?: boolean; onChange: (value: string) => void }) {
  const allLabel = label === 'status' ? 'All statuses' : label === 'time' ? 'All time' : `All ${label}s`
  return <Select value={value || 'all'} onValueChange={(next) => next && onChange(next === 'all' ? '' : next)}><SelectTrigger size="sm" aria-label={`Filter by ${label}`}><SelectValue>{() => value ? options.find(([key]) => key === value)?.[1] : allLabel}</SelectValue></SelectTrigger><SelectContent align="end">{includeAll ? <SelectItem value="all">{allLabel}</SelectItem> : null}{options.map(([key, text]) => <SelectItem key={key} value={key}>{text}</SelectItem>)}</SelectContent></Select>
}

function JobTitle({ job }: { job: QueueJob }) {
  return <div className="min-w-0"><p className="line-clamp-2 font-medium leading-snug">{job.title}</p><p className="mt-1 truncate text-xs text-muted-foreground">{job.source ?? kindLabels[job.kind] ?? job.kind} · {date.format(new Date(job.created_at))}</p></div>
}

function StatusBadge({ status }: { status: QueueJobStatus }) {
  const meta = statuses[status]
  const Icon = meta.icon
  return <Badge variant="outline" className={meta.className}><Icon className={cn(status === 'processing' && 'animate-spin')} />{meta.label}</Badge>
}

function QueueAttempt({ job }: { job: QueueJob }) {
  return <div className="text-xs"><p className="font-medium">{job.queue}</p><p className="mt-1 text-muted-foreground">Attempt {job.attempt}/{job.max_attempts} · {job.river_state}</p></div>
}

function JobStarted({ job }: { job: QueueJob }) {
  return <span className="whitespace-nowrap text-xs">{date.format(new Date(job.started_at ?? job.created_at))}</span>
}

function StepRail({ job }: { job: QueueJob }) {
  if (!job.steps.length) return <div className="flex h-8 items-center gap-2 text-xs text-muted-foreground"><span className="flex size-8 shrink-0 items-center justify-center"><StatusDot status={job.status} /></span><span>Single job · {kindLabels[job.kind] ?? job.kind}</span></div>
  return <ol className="relative flex h-8 items-center justify-between" aria-label="Pipeline steps">{job.steps.length > 1 ? <span aria-hidden className="absolute left-4 right-4 top-1/2 h-px -translate-y-1/2 bg-border" /> : null}{job.steps.map((step) => {
    const label = stepLabels[step.name] ?? step.name
    const status = step.status === 'succeeded' || step.status === 'skipped' ? 'completed' : step.status === 'running' ? 'processing' : step.status
    return <li key={step.id} className="relative z-10 flex size-8 shrink-0 items-center justify-center"><Tooltip><TooltipTrigger render={<button type="button" className="flex size-8 items-center justify-center rounded-full bg-card focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label={`${label}: ${step.status}`} />}><StatusDot status={status} /></TooltipTrigger><TooltipContent>{label} · <span className="capitalize">{step.status}</span></TooltipContent></Tooltip></li>
  })}</ol>
}

function StatusDot({ status }: { status: string }) {
  if (status === 'completed') return <span className="flex size-5 items-center justify-center rounded-full bg-emerald-600 text-white"><CheckCircle2 className="size-3.5" /></span>
  if (status === 'failed') return <span className="flex size-5 items-center justify-center rounded-full bg-destructive text-white"><OctagonX className="size-3.5" /></span>
  if (status === 'processing') return <span className="flex size-5 items-center justify-center rounded-full border-2 border-primary bg-background text-primary"><LoaderCircle className="size-3 animate-spin" /></span>
  return <span className="block size-5 rounded-full border-2 border-muted-foreground/50 bg-background" />
}

function JobInspector({
  job,
  artifacts,
  artifactsPending,
  artifactsError,
  failedStep,
  retrying,
  onClose,
  onRetry,
  onRetryArtifacts,
}: {
  job?: QueueJob
  artifacts?: QueueJobArtifacts
  artifactsPending: boolean
  artifactsError: boolean
  failedStep?: PipelineStep
  retrying: boolean
  onClose: () => void
  onRetry: () => void
  onRetryArtifacts: () => void
}) {
  const rootCause = failedStep?.error_detail ?? job?.error_detail
  return (
    <Sheet open={Boolean(job)} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="overflow-hidden data-[side=right]:w-full data-[side=right]:sm:max-w-2xl data-[side=right]:xl:max-w-3xl">
        <SheetHeader className="border-b">
          <SheetTitle>Workflow details</SheetTitle>
          <SheetDescription>Execution telemetry, inputs, and persisted output artifacts.</SheetDescription>
        </SheetHeader>
        {job ? (
          <div className="min-w-0 flex-1 space-y-6 overflow-y-auto overscroll-contain p-4 sm:p-6">
            <div className="flex min-w-0 flex-wrap items-start justify-between gap-3">
              <div className="min-w-0 flex-1 basis-64">
                <p className="break-words font-heading text-lg font-semibold leading-snug">{job.title}</p>
                {job.source ? <div className="mt-2 flex items-center gap-2"><SourceAvatar name={job.source} iconUrl={job.source_icon} className="size-6" /><span className="truncate text-xs text-muted-foreground">{job.source}</span></div> : null}
              </div>
              <StatusBadge status={job.status} />
            </div>

            <dl className="grid grid-cols-2 gap-x-5 gap-y-4 border-y py-5 text-xs sm:grid-cols-3">
              <Detail label="Job ID" value={String(job.job_id ?? job.run_id ?? 'Pending dispatch')} />
              <Detail label="Trigger" value={job.trigger ?? kindLabels[job.kind] ?? job.kind} />
              <Detail label="Queue" value={job.queue} />
              <Detail label="Attempt" value={`${job.attempt} of ${job.max_attempts}`} />
              <Detail label="Started" value={date.format(new Date(job.started_at ?? job.created_at))} />
              <Detail label="Duration" value={formatDuration(job.duration_ms)} />
            </dl>

            {job.steps.length ? <StepTimeline steps={job.steps} /> : null}

            <ArtifactCollection
              artifacts={artifacts}
              pending={artifactsPending}
              error={artifactsError}
              onRetry={onRetryArtifacts}
            />

            {rootCause ? <div><p className="text-sm font-semibold">Root cause</p><pre className="mt-2 whitespace-pre-wrap break-words rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-xs leading-5 text-destructive">{rootCause}</pre></div> : null}
            {job.error_trace ? <details className="group rounded-lg border bg-muted/20"><summary className="flex cursor-pointer list-none items-center gap-2 p-3 text-sm font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"><ChevronRight className="size-4 transition-transform group-open:rotate-90" />Error trace</summary><pre className="max-h-72 overflow-auto whitespace-pre-wrap break-words border-t p-3 text-[11px] leading-5 text-muted-foreground">{job.error_trace}</pre></details> : null}
            {job.article_id ? <Button variant="outline" nativeButton={false} render={<Link to={`/articles/${job.article_id}`} />}><ExternalLink />Open full pipeline logs</Button> : null}
          </div>
        ) : null}
        {failedStep && job?.article_id ? <SheetFooter className="border-t"><Button disabled={retrying} onClick={onRetry}><RotateCcw className={cn(retrying && 'animate-spin')} />Retry failed step</Button></SheetFooter> : null}
      </SheetContent>
    </Sheet>
  )
}

function StepTimeline({ steps }: { steps: PipelineStep[] }) {
  return <div><p className="text-sm font-semibold">Step timeline</p><ol className="mt-4 space-y-0">{steps.map((step, index) => <li key={step.id} className="relative grid min-w-0 grid-cols-[1.25rem_minmax(0,1fr)] gap-3 pb-5 last:pb-0"><span className={cn('relative z-10 bg-popover', index < steps.length - 1 && 'after:absolute after:left-1/2 after:top-5 after:h-[calc(100%+1px)] after:w-px after:-translate-x-1/2 after:bg-border')}><StatusDot status={step.status === 'succeeded' || step.status === 'skipped' ? 'completed' : step.status === 'running' ? 'processing' : step.status} /></span><div className="min-w-0"><p className="break-words text-sm font-medium">{stepLabels[step.name] ?? step.name}</p><p className="mt-1 break-words text-xs capitalize text-muted-foreground">{step.status} · attempt {step.attempt}/{step.max_attempts}{step.duration_ms == null ? '' : ` · ${formatDuration(step.duration_ms)}`}</p></div></li>)}</ol></div>
}

function ArtifactCollection({ artifacts, pending, error, onRetry }: { artifacts?: QueueJobArtifacts; pending: boolean; error: boolean; onRetry: () => void }) {
  if (pending) return <div className="space-y-3" aria-live="polite"><div className="flex items-center gap-2"><Skeleton className="size-4 rounded" /><Skeleton className="h-4 w-28" /></div><Skeleton className="h-20 w-full rounded-lg" /><Skeleton className="h-20 w-full rounded-lg" /></div>
  if (error) return <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4"><p className="text-sm font-medium text-destructive">Artifacts could not be loaded.</p><p className="mt-1 text-xs text-muted-foreground">The queue record is still available; only its detailed payload failed.</p><Button className="mt-3" size="sm" variant="outline" onClick={onRetry}><RefreshCw />Try again</Button></div>
  if (!artifacts) return null
  return <div className="space-y-6"><ArtifactGroup role="input" items={artifacts.inputs} /><ArtifactGroup role="output" items={artifacts.outputs} /></div>
}

function ArtifactGroup({ role, items }: { role: 'input' | 'output'; items: QueueJobArtifact[] }) {
  const Icon = role === 'input' ? FileInput : FileOutput
  const label = role === 'input' ? 'Input artifacts' : 'Output artifacts'
  return <section className="min-w-0" aria-labelledby={`${role}-artifacts-heading`}><div className="mb-3 flex items-center justify-between gap-3"><h3 id={`${role}-artifacts-heading`} className="flex min-w-0 items-center gap-2 text-sm font-semibold"><Icon className="size-4 shrink-0 text-primary" />{label}</h3><Badge variant="secondary" className="tabular-nums">{items.length}</Badge></div>{items.length ? <div className="space-y-2">{items.map((artifact, index) => <ArtifactDisclosure key={`${artifact.id}:${index}`} artifact={artifact} defaultOpen={role === 'output' && artifact.kind === 'article_content'} />)}</div> : <p className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">No {role} artifacts were persisted for this job.</p>}</section>
}

function ArtifactDisclosure({ artifact, defaultOpen }: { artifact: QueueJobArtifact; defaultOpen?: boolean }) {
  const Icon = artifactIcon(artifact)
  return <details className="group min-w-0 overflow-hidden rounded-xl border bg-card" open={defaultOpen}><summary className="flex min-w-0 cursor-pointer list-none items-center gap-3 p-3.5 transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset"><span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground"><Icon className="size-4" /></span><span className="min-w-0 flex-1"><span className="block break-words text-sm font-medium">{artifact.title}</span><span className="mt-0.5 block line-clamp-2 text-xs text-muted-foreground">{artifact.description}</span></span><ChevronRight className="size-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-90" /></summary><div className="min-w-0 border-t bg-muted/10 p-3.5 sm:p-4"><ArtifactBody artifact={artifact} /></div></details>
}

function artifactIcon(artifact: QueueJobArtifact) {
  if (artifact.kind === 'article') return Newspaper
  if (artifact.kind === 'article_content' || artifact.kind === 'daily_brief') return FileText
  if (artifact.kind === 'ingestion_summary' || artifact.kind === 'crawl_attempt') return Database
  if (artifact.kind === 'pipeline_step' || artifact.kind.includes('summary')) return ListTree
  if (artifact.kind === 'job_request') return FileInput
  if (artifact.kind === 'execution') return Activity
  return Braces
}

function ArtifactBody({ artifact }: { artifact: QueueJobArtifact }) {
  if (artifact.kind === 'article') return <ArticleArtifact data={artifact.data} />
  if (artifact.kind === 'article_content') return <ArticleContentArtifact data={artifact.data} />
  if (artifact.kind === 'crawl_attempt') return <CrawlAttemptArtifact data={artifact.data} />
  if (artifact.kind === 'daily_brief') return <DocumentArtifact data={artifact.data} />
  if (artifact.kind === 'ingestion_summary') return <IngestionArtifact data={artifact.data} />
  return <StructuredArtifact data={artifact.data} />
}

function ArticleArtifact({ data }: { data: Record<string, unknown> }) {
  const source = textValue(data.source)
  const sourceIcon = textValue(data.source_icon)
  const description = textValue(data.description)
  const url = textValue(data.original_url)
  return <article className="min-w-0"><div className="flex min-w-0 items-center gap-2"><SourceAvatar name={source || 'Article source'} iconUrl={sourceIcon} className="size-7" /><span className="truncate text-xs font-medium">{source || 'Unknown source'}</span>{data.status ? <Badge variant="outline" className="ml-auto capitalize">{textValue(data.status)}</Badge> : null}</div>{description ? <RichArtifactContent value={description} className="mt-4" /> : null}<dl className="mt-4 grid grid-cols-1 gap-3 text-xs sm:grid-cols-2"><ArtifactDetail label="Author" value={textValue(data.author) || 'Not supplied'} /><ArtifactDetail label="Published" value={formatArtifactDate(data.published_at)} />{data.category ? <ArtifactDetail label="Category" value={textValue(data.category)} /> : null}</dl>{url ? <Button className="mt-4 max-w-full" size="sm" variant="outline" nativeButton={false} render={<a href={url} target="_blank" rel="noreferrer" />}><ExternalLink />View original article</Button> : null}</article>
}

function ArticleContentArtifact({ data }: { data: Record<string, unknown> }) {
  const body = textValue(data.body_text)
  const url = textValue(data.source_url)
  return <div className="min-w-0"><dl className="grid grid-cols-2 gap-3 text-xs sm:grid-cols-4"><ArtifactDetail label="Characters" value={numberValue(data.characters).toLocaleString()} /><ArtifactDetail label="Method" value={humanizeKey(textValue(data.acquisition_method))} /><ArtifactDetail label="Extractor" value={textValue(data.extractor_version) || '—'} /><ArtifactDetail label="Fetched" value={formatArtifactDate(data.fetched_at)} /></dl>{url ? <CompactURLLink className="mt-4" url={url} /> : null}<ScrollArea className="mt-4 h-[min(32rem,60vh)] rounded-lg border bg-background"><div className="p-4 pr-5"><RichArtifactContent value={body} /></div></ScrollArea></div>
}

function DocumentArtifact({ data }: { data: Record<string, unknown> }) {
  return <article className="min-w-0"><div className="flex flex-wrap gap-2 text-xs text-muted-foreground"><span>{formatArtifactDate(data.date, true)}</span>{data.model ? <><span aria-hidden>·</span><span>{textValue(data.model)}</span></> : null}</div><ScrollArea className="mt-4 h-[min(32rem,60vh)] rounded-lg border bg-background"><div className="p-4 pr-5"><RichArtifactContent value={textValue(data.body)} /></div></ScrollArea></article>
}

function RichArtifactContent({ value, className }: { value: string; className?: string }) {
  return <Suspense fallback={<div className={cn('space-y-3', className)} aria-label="Formatting article content"><Skeleton className="h-4 w-full" /><Skeleton className="h-4 w-11/12" /><Skeleton className="h-4 w-4/5" /></div>}><LazyRichArticleContent value={value} className={className} /></Suspense>
}

function CrawlAttemptArtifact({ data }: { data: Record<string, unknown> }) {
  const requestedURL = textValue(data.requested_url)
  const finalURL = textValue(data.final_url)
  const telemetry = Object.fromEntries(Object.entries(data).filter(([key]) => key !== 'requested_url' && key !== 'final_url'))
  return <div className="min-w-0"><StructuredArtifact data={telemetry} /><dl className="mt-3 grid grid-cols-1 gap-3 text-xs sm:grid-cols-2"><ArtifactURLDetail label="Requested URL" url={requestedURL} /><ArtifactURLDetail label="Final URL" url={finalURL} /></dl></div>
}

function IngestionArtifact({ data }: { data: Record<string, unknown> }) {
  const runs = Array.isArray(data.runs) ? data.runs.filter(isRecord) : []
  return <div className="min-w-0"><dl className="grid grid-cols-2 gap-3 text-xs sm:grid-cols-4"><ArtifactDetail label="Endpoints" value={textValue(data.endpoints_checked)} /><ArtifactDetail label="Items seen" value={textValue(data.items_seen)} /><ArtifactDetail label="New articles" value={textValue(data.new_items)} /><ArtifactDetail label="Failed" value={textValue(data.failed_endpoints)} /></dl>{runs.length ? <div className="mt-4 space-y-2">{runs.map((run, index) => <details key={textValue(run.id) || index} className="group/run overflow-hidden rounded-lg border bg-background"><summary className="flex cursor-pointer list-none items-center gap-2 p-3 text-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"><StatusDot status={textValue(run.status) === 'ok' ? 'completed' : textValue(run.status) === 'running' ? 'processing' : 'failed'} /><span className="min-w-0 flex-1 truncate font-medium">{textValue(run.source)}</span><span className="text-muted-foreground">{textValue(run.new_items)} new</span><ChevronRight className="size-3.5 transition-transform group-open/run:rotate-90" /></summary><div className="border-t p-3"><StructuredArtifact data={run} omit={['id', 'source']} /></div></details>)}</div> : <p className="mt-4 rounded-lg border border-dashed p-3 text-xs text-muted-foreground">No endpoint runs were recorded in this job’s execution window.</p>}</div>
}

function StructuredArtifact({ data, omit = [] }: { data: Record<string, unknown>; omit?: string[] }) {
  const entries = Object.entries(data).filter(([key, value]) => !omit.includes(key) && value !== null && value !== undefined && key !== 'output' && key !== 'arguments')
  const structured = entries.filter(([, value]) => isPrimitive(value))
  const nested = entries.filter(([, value]) => !isPrimitive(value))
  const explicit = [['arguments', data.arguments], ['output', data.output]].filter(([, value]) => value !== null && value !== undefined) as [string, unknown][]
  return <div className="min-w-0 space-y-4">{structured.length ? <dl className="grid grid-cols-1 gap-3 text-xs sm:grid-cols-2">{structured.map(([key, value]) => <ArtifactDetail key={key} label={humanizeKey(key)} value={formatArtifactValue(key, value)} />)}</dl> : null}{[...nested, ...explicit].map(([key, value]) => <div key={key} className="min-w-0"><p className="mb-2 text-xs font-medium text-muted-foreground">{humanizeKey(key)}</p><pre className="max-h-72 max-w-full overflow-auto whitespace-pre-wrap break-words rounded-lg border bg-background p-3 font-mono text-[11px] leading-5">{JSON.stringify(value, null, 2)}</pre></div>)}</div>
}

function ArtifactDetail({ label, value }: { label: string; value: string }) {
  return <div className="min-w-0 rounded-lg bg-muted/50 p-2.5"><dt className="text-muted-foreground">{label}</dt><dd className="mt-1 break-words font-medium tabular-nums">{value || '—'}</dd></div>
}

function ArtifactURLDetail({ label, url }: { label: string; url: string }) {
  return <div className="min-w-0 rounded-lg bg-muted/50 p-2.5"><dt className="text-muted-foreground">{label}</dt><dd className="mt-1 min-w-0">{url ? <CompactURLLink url={url} /> : <span className="font-medium">—</span>}</dd></div>
}

function CompactURLLink({ url, className }: { url: string; className?: string }) {
  const label = readableURL(url)
  return <a className={cn('flex min-w-0 max-w-full items-center gap-1.5 text-xs font-medium text-primary underline-offset-4 hover:underline', className)} href={url} target="_blank" rel="noreferrer" title={label}><ExternalLink className="size-3.5 shrink-0" /><span className="truncate">{label}</span></a>
}

function readableURL(value: string) {
  try {
    const parsed = new URL(value)
    let path = parsed.pathname
    try {
      path = decodeURIComponent(path)
    } catch {
      // Keep a malformed path usable instead of failing the whole artifact.
    }
    return `${parsed.host}${path}${parsed.search}${parsed.hash}`
  } catch {
    try {
      return decodeURIComponent(value)
    } catch {
      return value
    }
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function isPrimitive(value: unknown) {
  return typeof value !== 'object'
}

function textValue(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return ''
}

function numberValue(value: unknown) {
  if (typeof value === 'number') return value
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function humanizeKey(value: string) {
  if (!value) return 'Not supplied'
  return value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase())
}

function formatArtifactDate(value: unknown, dateOnly = false) {
  const raw = textValue(value)
  if (!raw) return '—'
  const parsed = new Date(raw)
  if (Number.isNaN(parsed.getTime())) return raw
  return dateOnly ? parsed.toLocaleDateString('en', { dateStyle: 'medium' }) : date.format(parsed)
}

function formatArtifactValue(key: string, value: unknown) {
  if (key.endsWith('_at') || key === 'date') return formatArtifactDate(value, key === 'date')
  if (key === 'duration_ms') return formatDuration(typeof value === 'number' ? value : null)
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  return textValue(value) || '—'
}

function Detail({ label, value }: { label: string; value: string }) {
  return <div className="min-w-0"><dt className="text-muted-foreground">{label}</dt><dd className="mt-1 truncate font-medium" title={value}>{value}</dd></div>
}

function LoadingRows() {
  return Array.from({ length: 6 }, (_, index) => <TableRow key={index}>{Array.from({ length: 7 }, (_, cell) => <TableCell key={cell}><Skeleton className="h-5 w-full max-w-32" /></TableCell>)}</TableRow>)
}

function MessageRow({ message, empty = false }: { message: string; empty?: boolean }) {
  return <TableRow><TableCell colSpan={7} className="h-48 text-center text-sm text-muted-foreground">{empty ? <span className="inline-flex flex-col items-center gap-2"><Inbox className="size-5" />{message}</span> : message}</TableCell></TableRow>
}

function formatDuration(value: number | null) {
  if (value == null) return '—'
  if (value < 1_000) return `${value} ms`
  const seconds = Math.floor(value / 1_000)
  if (seconds < 60) return `${seconds} sec`
  const minutes = Math.floor(seconds / 60)
  return `${minutes}m ${seconds % 60}s`
}

function formatInterval(seconds: number) {
  if (seconds === 60) return 'Every minute'
  if (seconds === 3_600) return 'Every hour'
  if (seconds === 86_400) return 'Daily'
  if (seconds % 86_400 === 0) return `Every ${seconds / 86_400} days`
  if (seconds % 3_600 === 0) return `Every ${seconds / 3_600} hours`
  if (seconds % 60 === 0) return `Every ${seconds / 60} min`
  return `Every ${seconds} sec`
}

function relativeTime(value: string | null, schedule = false) {
  if (!value) return schedule ? 'Not scheduled' : 'Never'
  const difference = new Date(value).getTime() - Date.now()
  const future = difference > 0
  const elapsed = Math.abs(difference)
  if (elapsed < 60_000) return future ? 'In less than a minute' : schedule ? 'Due now' : 'Just now'
  const units = elapsed < 3_600_000
    ? { amount: Math.floor(elapsed / 60_000), label: 'min' }
    : elapsed < 86_400_000
      ? { amount: Math.floor(elapsed / 3_600_000), label: 'hr' }
      : { amount: Math.floor(elapsed / 86_400_000), label: 'day' }
  const label = `${units.amount} ${units.label}${units.amount === 1 || units.label === 'min' ? '' : 's'}`
  if (future) return `In ${label}`
  return schedule ? `${label} overdue` : `${label} ago`
}

function formatReliability(job: CronJobStatistic) {
  const rate = job.success_rate_24h == null ? '—' : `${Math.round(job.success_rate_24h * 100)}%`
  return `${rate} · ${job.runs_24h} runs`
}

function shortWorkerID(value: string | null) {
  if (!value) return 'not elected'
  return value.length > 24 ? `${value.slice(0, 21)}…` : value
}
