import {
  createClient,
  type PipelineStep,
  type QueueJob,
  type QueueJobStatus,
} from '@snap/api-client'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  CheckCircle2,
  Circle,
  Clock3,
  ExternalLink,
  Inbox,
  LoaderCircle,
  OctagonX,
  RefreshCw,
  RotateCcw,
  TriangleAlert,
} from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { DataTablePagination, DataTableToolbar } from '@/components/data-table-controls'
import { SourceAvatar } from '@/components/source-avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
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

const statuses: Record<QueueJobStatus, { label: string; icon: typeof Circle; className: string }> = {
  queued: { label: 'Queued', icon: Clock3, className: 'border-slate-300 bg-slate-50 text-slate-700 dark:bg-slate-900' },
  processing: { label: 'Processing', icon: LoaderCircle, className: 'border-blue-300 bg-blue-50 text-blue-700 dark:bg-blue-950' },
  completed: { label: 'Completed', icon: CheckCircle2, className: 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:bg-emerald-950' },
  partially_completed: { label: 'Partially completed', icon: TriangleAlert, className: 'border-amber-300 bg-amber-50 text-amber-700 dark:bg-amber-950' },
  failed: { label: 'Failed', icon: OctagonX, className: 'border-red-300 bg-red-50 text-red-700 dark:bg-red-950' },
}

const stepLabels: Record<string, string> = {
  categorization: 'Categorization',
  event_clustering: 'Event clustering',
  narration_analysis: 'Narration analysis',
}

const kindLabels: Record<string, string> = {
  'article.pipeline': 'Article workflow',
  'article.pipeline.dispatch': 'Pipeline dispatch',
  'ingest.poll': 'Source polling',
  'brief.daily': 'Daily brief',
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
    refetchInterval: 5_000,
  })
  const selected = jobs.data?.items.find((job) => job.id === selectedID)
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
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="font-heading text-2xl font-semibold tracking-tight">Queue monitor</h1>
          <p className="mt-1 text-sm text-muted-foreground">Pipeline workflows from the last seven days. Include routine jobs with the filters.</p>
        </div>
        <Button variant="outline" size="icon" aria-label="Refresh queue" disabled={jobs.isFetching} onClick={() => void jobs.refetch()}>
          <RefreshCw className={cn(jobs.isFetching && 'animate-spin')} />
        </Button>
      </div>

      <TelemetryStrip
        summary={jobs.data?.summary}
        selected={status}
        onSelect={(next) => table.setFilter('status', next === status ? '' : next)}
      />

      <Card className="gap-0 overflow-hidden py-0 shadow-sm">
        <CardContent className="px-0">
          <DataTableToolbar search={table.search} searchPlaceholder="Search jobs…" onSearch={table.setSearch}>
            <QueueSelect label="status" value={status} options={Object.entries(statuses).map(([value, item]) => [value, item.label])} onChange={(value) => table.setFilter('status', value)} />
            <QueueSelect label="queue" value={queue} options={[["analysis", "Analysis queue"], ["default", "Default queue"]]} onChange={(value) => table.setFilter('queue', value)} />
            <QueueSelect label="job type" value={kind} options={Object.entries(kindLabels)} onChange={(value) => table.setFilter('kind', value || 'all')} />
            <QueueSelect label="time" value={window} options={[["24h", "Last 24 hours"], ["7d", "Last 7 days"]]} includeAll={false} onChange={(value) => table.setFilter('window', value)} />
          </DataTableToolbar>

          <div className="hidden overflow-x-auto md:block">
            <Table className="min-w-[1050px]">
              <TableHeader className="bg-muted/30">
                <TableRow>
                  <TableHead>Workflow</TableHead>
                  <TableHead>Overall status</TableHead>
                  <TableHead className="w-80">Step progress</TableHead>
                  <TableHead>Queue / attempt</TableHead>
                  <TableHead>Started</TableHead>
                  <TableHead>Duration</TableHead>
                  <TableHead className="text-right">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {jobs.isPending ? <LoadingRows /> : null}
                {jobs.isError ? <MessageRow message="Queue telemetry is temporarily unavailable." /> : null}
                {!jobs.isPending && !jobs.isError && items.length === 0 ? <MessageRow message="No jobs match these filters." empty /> : null}
                {!jobs.isPending && !jobs.isError ? items.map((job) => (
                  <TableRow key={job.id} data-state={selectedID === job.id ? 'selected' : undefined}>
                    <TableCell className="max-w-80 whitespace-normal">
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

          <div className="divide-y md:hidden">
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

      <JobInspector job={selected} failedStep={failedStep} retrying={retry.isPending} onClose={() => setSelectedID(null)} onRetry={() => retry.mutate()} />
    </section>
  )
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

function JobInspector({ job, failedStep, retrying, onClose, onRetry }: { job?: QueueJob; failedStep?: PipelineStep; retrying: boolean; onClose: () => void; onRetry: () => void }) {
  const rootCause = failedStep?.error_detail ?? job?.error_detail
  return <Sheet open={Boolean(job)} onOpenChange={(open) => !open && onClose()}><SheetContent className="overflow-hidden data-[side=right]:w-full data-[side=right]:sm:max-w-xl"><SheetHeader className="border-b"><SheetTitle>Workflow details</SheetTitle><SheetDescription>River execution telemetry and pipeline step outcomes.</SheetDescription></SheetHeader>{job ? <div className="flex-1 space-y-6 overflow-y-auto p-6"><div className="flex items-start justify-between gap-3"><div className="min-w-0"><p className="font-heading text-lg font-semibold leading-snug">{job.title}</p>{job.source ? <div className="mt-2 flex items-center gap-2"><SourceAvatar name={job.source} iconUrl={job.source_icon} className="size-6" /><span className="text-xs text-muted-foreground">{job.source}</span></div> : null}</div><StatusBadge status={job.status} /></div><dl className="grid grid-cols-2 gap-x-5 gap-y-4 border-y py-5 text-xs"><Detail label="Job ID" value={String(job.job_id ?? job.run_id ?? 'Pending dispatch')} /><Detail label="Trigger" value={job.trigger ?? kindLabels[job.kind] ?? job.kind} /><Detail label="Queue" value={job.queue} /><Detail label="Attempt" value={`${job.attempt} of ${job.max_attempts}`} /><Detail label="Started" value={date.format(new Date(job.started_at ?? job.created_at))} /><Detail label="Duration" value={formatDuration(job.duration_ms)} /></dl>{job.steps.length ? <div><p className="text-sm font-semibold">Step timeline</p><ol className="mt-4 space-y-0">{job.steps.map((step, index) => <li key={step.id} className="relative grid grid-cols-[1.25rem_1fr_auto] gap-3 pb-5 last:pb-0"><span className={cn('relative z-10 bg-popover', index < job.steps.length - 1 && 'after:absolute after:left-1/2 after:top-5 after:h-[calc(100%+1px)] after:w-px after:-translate-x-1/2 after:bg-border')}><StatusDot status={step.status === 'succeeded' || step.status === 'skipped' ? 'completed' : step.status === 'running' ? 'processing' : step.status} /></span><div><p className="text-sm font-medium">{stepLabels[step.name] ?? step.name}</p><p className="mt-1 text-xs capitalize text-muted-foreground">{step.status} · attempt {step.attempt}/{step.max_attempts}{step.duration_ms == null ? '' : ` · ${formatDuration(step.duration_ms)}`}</p></div></li>)}</ol></div> : <p className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">This is a single River job and has no child workflow steps.</p>}{rootCause ? <div><p className="text-sm font-semibold">Root cause</p><pre className="mt-2 whitespace-pre-wrap break-words rounded-lg border border-destructive/30 bg-destructive/5 p-3 text-xs leading-5 text-destructive">{rootCause}</pre></div> : null}{job.error_trace ? <div><p className="text-sm font-semibold">Error trace</p><pre className="mt-2 max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-lg bg-muted/60 p-3 text-[11px] leading-5 text-muted-foreground">{job.error_trace}</pre></div> : null}{job.article_id ? <Button variant="outline" nativeButton={false} render={<Link to={`/articles/${job.article_id}`} />}><ExternalLink />Open full pipeline logs</Button> : null}</div> : null}{failedStep && job?.article_id ? <SheetFooter className="border-t"><Button disabled={retrying} onClick={onRetry}><RotateCcw className={cn(retrying && 'animate-spin')} />Retry failed step</Button></SheetFooter> : null}</SheetContent></Sheet>
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
