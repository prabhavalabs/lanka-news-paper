import { createClient, type PipelineStep } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft, Check, CircleDashed, Clock3, ExternalLink, GitBranch,
  LoaderCircle, RefreshCw, ServerCog, Sparkles, TriangleAlert, X,
} from 'lucide-react'
import { Link, useParams } from 'react-router'
import { toast } from 'sonner'

import { SourceAvatar } from '@/components/source-avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const client = createClient()
const date = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' })
const stepLabels: Record<string, string> = {
  categorization: 'Categorization', event_clustering: 'Event clustering', narration_analysis: 'Narration analysis',
}

export function ArticleDetailPage() {
  const { id = '' } = useParams()
  const queryClient = useQueryClient()
  const article = useQuery({
    queryKey: ['article', id], queryFn: () => client.adminArticle(id),
    refetchInterval: (query) => ['queued', 'running'].includes(query.state.data?.pipeline_runs[0]?.status ?? '') ? 5_000 : false,
  })
  const retry = useMutation({
    mutationFn: (step: string) => client.retryArticlePipeline(id, step),
    onSuccess: () => { toast.success('Pipeline retry queued'); void queryClient.invalidateQueries({ queryKey: ['article', id] }) },
    onError: (error) => toast.error(error.message || 'Could not retry pipeline'),
  })

  if (article.isPending) return <DetailSkeleton />
  if (article.isError) return <p className="p-8 text-sm text-muted-foreground">This article could not be loaded.</p>

  const item = article.data
  const run = item.pipeline_runs[0]
  const political = item.political
  const description = item.description.replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim()

  return (
    <section className="flex flex-col gap-6">
      <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="max-w-4xl">
          <Button variant="ghost" size="sm" className="-ml-3 mb-3" nativeButton={false} render={<Link to="/articles" />}><ArrowLeft /> Back to articles</Button>
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <Badge variant="outline">{item.category?.replaceAll('-', ' ') ?? 'Unassigned'}</Badge>
            <Badge variant={item.public_status === 'published' ? 'default' : 'outline'}>{item.public_status}</Badge>
            {run ? <PipelineBadge status={run.status} /> : <Badge variant="secondary">Pipeline not started</Badge>}
          </div>
          <h1 className="font-heading text-2xl font-semibold leading-tight tracking-tight lg:text-3xl">{item.headline}</h1>
          <div className="mt-4 flex items-center gap-3 text-sm text-muted-foreground">
            <SourceAvatar name={item.source} iconUrl={item.source_icon} className="size-9" />
            <span><strong className="font-medium text-foreground">{item.source}</strong><br />Published {date.format(new Date(item.published_at))}</span>
          </div>
        </div>
        <Button variant="outline" nativeButton={false} render={<a href={item.original_url} target="_blank" rel="noreferrer" />}><ExternalLink /> Open original</Button>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Metric label="Pipeline" value={run?.status ?? 'Not started'} detail={run?.current_step?.replaceAll('_', ' ') ?? `${run?.steps.length ?? 0} recorded steps`} icon={ServerCog} />
        <Metric label="Classification" value={item.category?.replaceAll('-', ' ') ?? 'Unassigned'} detail={item.classification_confidence == null ? 'No confidence recorded' : `${Math.round(item.classification_confidence * 100)}% confidence`} icon={GitBranch} />
        <Metric label="Event" value={item.event ? 'Clustered' : 'Unclustered'} detail={item.event?.algorithm_version ?? 'No matching event yet'} icon={CircleDashed} />
        <Metric label="Narration" value={political?.relevant ? political.label.replaceAll('_', ' ') : political ? 'Not relevant' : 'Pending'} detail={political ? `${Math.round(political.confidence * 100)}% confidence` : 'Waiting for analysis'} icon={Sparkles} />
      </div>

      <Card>
        <CardHeader><CardTitle>Processing pipeline</CardTitle><CardDescription>Durable execution state. Successful steps are retained when a later step retries.</CardDescription></CardHeader>
        <CardContent>
          {run ? (
            <div className="grid gap-3 lg:grid-cols-3">
              {run.steps.map((step) => <PipelineStepCard key={step.id} step={step} retrying={retry.isPending} onRetry={() => retry.mutate(step.name)} />)}
            </div>
          ) : <p className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">The dispatcher has not created a run for this historical article yet.</p>}
          {run?.last_error ? <div className="mt-4 flex gap-3 rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm"><TriangleAlert className="mt-0.5 size-4 shrink-0 text-destructive" /><div><p className="font-medium">Last failure</p><p className="mt-1 break-words text-muted-foreground">{run.last_error}</p></div></div> : null}
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-[1.35fr_1fr]">
        <Card>
          <CardHeader><CardTitle>Captured article</CardTitle><CardDescription>Content stored from the source endpoint.</CardDescription></CardHeader>
          <CardContent className="space-y-5">
            <p className="whitespace-pre-line text-sm leading-7 text-muted-foreground">{description || 'No article excerpt was supplied by this endpoint.'}</p>
            <dl className="grid gap-4 border-t pt-5 text-sm sm:grid-cols-2">
              <Detail label="Author" value={item.author || 'Not supplied'} />
              <Detail label="Received" value={date.format(new Date(item.received_at))} />
              <Detail label="Publisher category" value={item.publisher_category || 'Not supplied'} />
              <Detail label="Rights mode" value={item.rights_mode.replaceAll('_', ' ')} />
              <Detail label="Classification model" value={item.classification_model ?? 'Pending'} />
              <Detail label="Endpoint" value={item.endpoint_url} />
            </dl>
          </CardContent>
        </Card>
        <PoliticalCard political={political} />
      </div>

      <Card className="gap-0 py-0">
        <CardHeader className="border-b py-6"><CardTitle>LLM telemetry</CardTitle><CardDescription>Model calls linked to this article and pipeline run.</CardDescription></CardHeader>
        <CardContent className="px-0">
          <Table>
            <TableHeader className="bg-muted/30"><TableRow><TableHead>Task</TableHead><TableHead>Provider</TableHead><TableHead>Model</TableHead><TableHead>Result</TableHead><TableHead>Latency</TableHead><TableHead>Time</TableHead></TableRow></TableHeader>
            <TableBody>
              {item.llm_calls.length ? item.llm_calls.map((call) => (
                <TableRow key={call.id}>
                  <TableCell className="font-medium">{call.task.replaceAll('_', ' ')}</TableCell>
                  <TableCell>{call.provider_id}</TableCell><TableCell>{call.model}</TableCell>
                  <TableCell><Badge variant={call.outcome === 'ok' ? 'outline' : 'destructive'}>{call.outcome}</Badge>{call.error_detail ? <p className="mt-1 max-w-md text-xs text-destructive">{call.error_detail}</p> : null}</TableCell>
                  <TableCell className="tabular-nums">{call.latency_ms == null ? '—' : formatDuration(call.latency_ms)}</TableCell>
                  <TableCell className="text-muted-foreground">{date.format(new Date(call.created_at))}</TableCell>
                </TableRow>
              )) : <TableRow><TableCell colSpan={6} className="h-32 text-center text-muted-foreground">No LLM calls recorded for this article yet.</TableCell></TableRow>}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </section>
  )
}

function PipelineStepCard({ step, retrying, onRetry }: { step: PipelineStep; retrying: boolean; onRetry: () => void }) {
  const Icon = step.status === 'succeeded' ? Check : step.status === 'failed' ? X : step.status === 'running' ? LoaderCircle : step.status === 'skipped' ? CircleDashed : Clock3
  return (
    <div className="relative rounded-xl border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="flex gap-3"><span className={`flex size-8 shrink-0 items-center justify-center rounded-full ${step.status === 'succeeded' ? 'bg-primary text-primary-foreground' : step.status === 'failed' ? 'bg-destructive text-white' : 'bg-muted'}`}><Icon className={`size-4 ${step.status === 'running' ? 'animate-spin' : ''}`} /></span><div><p className="font-medium">{stepLabels[step.name] ?? step.name}</p><p className="mt-0.5 text-xs capitalize text-muted-foreground">{step.status} · attempt {step.attempt}/{step.max_attempts}</p></div></div>
        {step.status === 'failed' ? <Button variant="outline" size="icon-sm" aria-label={`Retry ${stepLabels[step.name]}`} disabled={retrying} onClick={onRetry}><RefreshCw /></Button> : null}
      </div>
      <p className="mt-4 text-xs text-muted-foreground">{step.duration_ms == null ? 'Waiting for execution' : `Completed in ${formatDuration(step.duration_ms)}`}</p>
      {step.error_detail ? <p className="mt-2 line-clamp-3 text-xs text-destructive" title={step.error_detail}>{step.error_detail}</p> : null}
      {Object.keys(step.output).length ? <pre className="mt-3 max-h-28 overflow-auto rounded-lg bg-muted/50 p-2 text-[11px] text-muted-foreground">{JSON.stringify(step.output, null, 2)}</pre> : null}
    </div>
  )
}

function PoliticalCard({ political }: { political: Awaited<ReturnType<typeof client.adminArticle>>['political'] }) {
  if (!political) return <Card><CardHeader><CardTitle>Political narration</CardTitle><CardDescription>Waiting for the narration step.</CardDescription></CardHeader><CardContent><Skeleton className="h-24 w-full" /></CardContent></Card>
  const position = (political.economic_frame + 1) * 50
  return (
    <Card>
      <CardHeader><CardTitle>Political narration</CardTitle><CardDescription>State-led (-1) to market-led (+1), based on the journalist’s framing.</CardDescription></CardHeader>
      <CardContent className="space-y-5">
        {political.relevant ? <><div className="flex items-end justify-between"><span className="text-2xl font-semibold capitalize">{political.label.replaceAll('_', ' ')}</span><span className="font-mono text-xl">{political.economic_frame.toFixed(2)}</span></div><div><div className="relative h-3 rounded-full bg-gradient-to-r from-blue-600 via-muted to-amber-500"><span className="absolute top-1/2 size-5 -translate-x-1/2 -translate-y-1/2 rounded-full border-4 border-background bg-foreground shadow" style={{ left: `${position}%` }} /></div><div className="mt-2 flex justify-between text-xs text-muted-foreground"><span>State-led</span><span>Neutral</span><span>Market-led</span></div></div></> : <div className="rounded-xl bg-muted/50 p-4 text-sm">This article is not meaningfully about economic policy, so no directional score was assigned.</div>}
        <div><p className="text-sm font-medium">Why</p><p className="mt-1 text-sm leading-6 text-muted-foreground">{political.rationale}</p></div>
        {political.evidence.length ? <div><p className="text-sm font-medium">Evidence</p><ul className="mt-2 space-y-2 text-sm text-muted-foreground">{political.evidence.map((evidence) => <li key={evidence} className="border-l-2 pl-3">“{evidence}”</li>)}</ul></div> : null}
        <p className="text-xs text-muted-foreground">{political.provider_id} · {political.provider_model} · {Math.round(political.confidence * 100)}% confidence</p>
      </CardContent>
    </Card>
  )
}

function Metric({ label, value, detail, icon: Icon }: { label: string; value: string; detail: string; icon: typeof ServerCog }) { return <Card><CardContent className="flex items-start justify-between p-5"><div><p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</p><p className="mt-2 text-xl font-semibold capitalize">{value}</p><p className="mt-1 text-xs text-muted-foreground">{detail}</p></div><span className="rounded-lg bg-primary/10 p-2 text-primary"><Icon className="size-4" /></span></CardContent></Card> }
function Detail({ label, value }: { label: string; value: string }) { return <div className="min-w-0"><dt className="text-xs uppercase tracking-wide text-muted-foreground">{label}</dt><dd className="mt-1 break-words capitalize">{value}</dd></div> }
function PipelineBadge({ status }: { status: string }) { return <Badge variant={status === 'failed' ? 'destructive' : status === 'succeeded' ? 'default' : 'outline'} className="capitalize">{status === 'running' ? <LoaderCircle className="animate-spin" /> : null}{status}</Badge> }
function formatDuration(ms: number) { return ms >= 60_000 ? `${(ms / 60_000).toFixed(1)}m` : ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms` }
function DetailSkeleton() { return <div className="space-y-6"><Skeleton className="h-8 w-32" /><Skeleton className="h-20 w-full max-w-4xl" /><div className="grid gap-4 md:grid-cols-4">{Array.from({ length: 4 }, (_, index) => <Skeleton key={index} className="h-28" />)}</div><Skeleton className="h-72" /></div> }
