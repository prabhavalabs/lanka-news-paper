import { createClient, type LLMCall, type PipelineRun, type PipelineStep } from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft, BrainCircuit, Check, CircleDashed, Clock3, ExternalLink, GitBranch,
  LoaderCircle, Network, Play, RotateCcw, Rss, ServerCog, Sparkles, Tags, TriangleAlert, X,
} from 'lucide-react'
import { useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
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
const stepIcons = { categorization: Tags, event_clustering: Network, narration_analysis: BrainCircuit }
const canvas = { width: 1080, height: 300, nodeWidth: 220, nodeHeight: 132 }
const initialPositions: Record<string, { x: number; y: number }> = {
  source: { x: 30, y: 84 }, categorization: { x: 300, y: 84 },
  event_clustering: { x: 570, y: 84 }, narration_analysis: { x: 840, y: 84 },
}

function safeExternalURL(value: string) {
  try {
    const parsed = new URL(value)
    return parsed.protocol === 'https:' ? parsed.toString() : undefined
  } catch {
    return undefined
  }
}

export function ArticleDetailPage() {
  const { id = '' } = useParams()
  const queryClient = useQueryClient()
  const article = useQuery({
    queryKey: ['article', id], queryFn: () => client.adminArticle(id),
    refetchInterval: (query) => ['queued', 'running'].includes(query.state.data?.pipeline_runs[0]?.status ?? '') ? 5_000 : false,
  })
  const runPipeline = useMutation({
    mutationFn: async (step: string) => {
      if (step === 'source') return client.runEndpoint(article.data!.endpoint_id)
      if (step === '') await client.runEndpoint(article.data!.endpoint_id)
      return client.runArticlePipeline(id, step)
    },
    onSuccess: (_data, step) => { toast.success(step === 'source' ? 'Source capture completed' : step ? 'Pipeline step queued' : 'Full pipeline queued'); void queryClient.invalidateQueries({ queryKey: ['article', id] }) },
    onError: (error) => toast.error(error.message || 'Could not run pipeline'),
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
        {safeExternalURL(item.original_url) ? <Button variant="outline" nativeButton={false} render={<a href={safeExternalURL(item.original_url)} target="_blank" rel="noreferrer" />}><ExternalLink /> Open original</Button> : null}
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Metric label="Pipeline" value={run?.status ?? 'Not started'} detail={run?.current_step?.replaceAll('_', ' ') ?? `${run?.steps.length ?? 0} recorded steps`} icon={ServerCog} />
        <Metric label="Classification" value={item.category?.replaceAll('-', ' ') ?? 'Unassigned'} detail={item.classification_confidence == null ? 'No confidence recorded' : `${Math.round(item.classification_confidence * 100)}% confidence`} icon={GitBranch} />
        <Metric label="Event" value={item.event ? 'Clustered' : 'Unclustered'} detail={item.event?.algorithm_version ?? 'No matching event yet'} icon={CircleDashed} />
        <Metric label="Narration" value={political?.relevant ? political.label.replaceAll('_', ' ') : political ? 'Not relevant' : 'Pending'} detail={political ? `${Math.round(political.confidence * 100)}% confidence` : 'Waiting for analysis'} icon={Sparkles} />
      </div>

      <PipelineInspector
        runs={item.pipeline_runs}
        source={{ name: item.source, icon: item.source_icon, endpoint: item.endpoint_url, receivedAt: item.received_at, rightsMode: item.rights_mode }}
        calls={item.llm_calls}
        running={runPipeline.isPending}
        onRun={(step) => runPipeline.mutate(step)}
      />

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

      <Card>
        <CardHeader>
          <CardTitle>Restricted full content</CardTitle>
          <CardDescription>{item.content ? `${item.content.characters.toLocaleString()} characters captured through ${item.content.acquisition_method.replaceAll('_', ' ')}.` : 'No approved full article body has been stored.'}</CardDescription>
        </CardHeader>
        <CardContent>
          {item.content ? (
            <div className="space-y-5">
              <p className="max-h-[34rem] overflow-y-auto whitespace-pre-line rounded-2xl border bg-muted/10 p-4 text-sm leading-7">{item.content.body_text}</p>
              <dl className="grid gap-4 border-t pt-5 text-sm sm:grid-cols-2 lg:grid-cols-4">
                <Detail label="Acquired" value={date.format(new Date(item.content.fetched_at))} />
                <Detail label="Extractor" value={item.content.extractor_version} />
                <Detail label="Retention" value={item.content.retention_until ? date.format(new Date(item.content.retention_until)) : 'No automatic expiry'} />
                <Detail label="Visibility" value="Admin only" />
              </dl>
            </div>
          ) : <p className="rounded-2xl border border-dashed p-6 text-sm text-muted-foreground">Full text remains unavailable until the collection recipe and compliance review both authorize it.</p>}
        </CardContent>
      </Card>

      <Card className="gap-0 py-0">
        <CardHeader className="border-b py-6"><CardTitle>LLM telemetry</CardTitle><CardDescription>Model calls linked to this article and pipeline run.</CardDescription></CardHeader>
        <CardContent className="px-0">
          <Table>
            <TableHeader className="bg-muted/30"><TableRow><TableHead>Task</TableHead><TableHead>Provider</TableHead><TableHead>Model</TableHead><TableHead>Result</TableHead><TableHead>Latency</TableHead><TableHead>Time</TableHead></TableRow></TableHeader>
            <TableBody>
              {item.llm_calls.length ? item.llm_calls.map((call) => (
                <TableRow key={call.id}>
                  <TableCell className="font-medium">{call.task.replaceAll('_', ' ')}</TableCell>
                  <TableCell>{call.provider_id}</TableCell><TableCell>{call.model}<p className="mt-1 text-xs text-muted-foreground">{formatTokens(call)}</p></TableCell>
                  <TableCell><Badge variant={call.outcome === 'ok' ? 'outline' : call.outcome === 'running' ? 'secondary' : 'destructive'}>{call.outcome}</Badge>{call.error_detail ? <p className="mt-1 max-w-md text-xs text-destructive">{call.error_detail}</p> : null}</TableCell>
                  <TableCell className="tabular-nums">{call.latency_ms == null ? 'In progress' : formatDuration(call.latency_ms)}{call.first_token_ms != null ? <p className="mt-1 text-xs text-muted-foreground">first token {formatDuration(call.first_token_ms)}</p> : null}</TableCell>
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

function PipelineInspector({ runs, source, calls, running, onRun }: {
  runs: PipelineRun[]
  source: { name: string; icon: string; endpoint: string; receivedAt: string; rightsMode: string }
  calls: LLMCall[]
  running: boolean
  onRun: (step: string) => void
}) {
  const [selectedRun, setSelectedRun] = useState('')
  const run = runs.find((item) => item.id === selectedRun) ?? runs[0]
  const initialStep = run?.steps.find((step) => step.name === run.current_step) ?? run?.steps[0]
  const [selected, setSelected] = useState(initialStep?.id ?? 'source')
  const [positions, setPositions] = useState(() => structuredClone(initialPositions))
  const canvasRef = useRef<HTMLDivElement>(null)
  const drag = useRef<{ key: string; pointer: number; offsetX: number; offsetY: number } | null>(null)
  const selectedStep = run?.steps.find((step) => step.id === selected)
  const selectedCalls = selectedStep ? calls.filter((call) => call.pipeline_step_id === selectedStep.id) : []
  const busy = running || ['queued', 'running'].includes(runs[0]?.status ?? '')
  const nodes = run ? [
    { key: 'source', label: 'Source intake', detail: source.name, status: 'succeeded', icon: Rss, id: 'source' },
    ...run.steps.map((step) => ({
      key: step.name, label: stepLabels[step.name] ?? step.name.replaceAll('_', ' '),
      detail: stepDetail(step), status: step.status, icon: stepIcons[step.name as keyof typeof stepIcons] ?? GitBranch, id: step.id,
    })),
  ] : []

  function chooseRun(id: string) {
    const next = runs.find((item) => item.id === id)
    setSelectedRun(id)
    setSelected(next?.steps.find((step) => step.name === next.current_step)?.id ?? next?.steps[0]?.id ?? 'source')
  }

  function runStep(step: string) {
    setSelectedRun('')
    setSelected('source')
    onRun(step)
  }

  function moveNode(key: string, x: number, y: number) {
    setPositions((current) => ({ ...current, [key]: {
      x: Math.max(16, Math.min(canvas.width - canvas.nodeWidth - 16, x)),
      y: Math.max(48, Math.min(canvas.height - canvas.nodeHeight - 16, y)),
    } }))
  }

  function pointFor(key: string, index = 0) {
    return positions[key] ?? { x: 30 + index * 270, y: 84 }
  }

  function startDrag(event: PointerEvent<HTMLButtonElement>, key: string) {
    const bounds = canvasRef.current?.getBoundingClientRect()
    const point = pointFor(key)
    if (!bounds || !point) return
    event.currentTarget.setPointerCapture(event.pointerId)
    drag.current = { key, pointer: event.pointerId, offsetX: event.clientX - bounds.left - point.x, offsetY: event.clientY - bounds.top - point.y }
  }

  function dragNode(event: PointerEvent<HTMLButtonElement>) {
    const active = drag.current
    const bounds = canvasRef.current?.getBoundingClientRect()
    if (!active || active.pointer !== event.pointerId || !bounds) return
    event.preventDefault()
    moveNode(active.key, event.clientX - bounds.left - active.offsetX, event.clientY - bounds.top - active.offsetY)
  }

  function endDrag(event: PointerEvent<HTMLButtonElement>) {
    if (drag.current?.pointer === event.pointerId) drag.current = null
  }

  function moveWithKeyboard(event: KeyboardEvent<HTMLButtonElement>, key: string) {
    const offset = event.shiftKey ? 24 : 8
    const movement = ({ ArrowLeft: [-offset, 0], ArrowRight: [offset, 0], ArrowUp: [0, -offset], ArrowDown: [0, offset] } as Record<string, [number, number]>)[event.key]
    if (!movement) return
    event.preventDefault()
    const point = pointFor(key)
    moveNode(key, point.x + movement[0], point.y + movement[1])
  }

  return (
    <Card className="overflow-hidden">
      <CardHeader className="border-b lg:flex lg:flex-row lg:items-center lg:justify-between">
        <div><CardTitle>Processing pipeline</CardTitle><CardDescription>Drag nodes to explore the flow. Select one to inspect its complete execution log.</CardDescription></div>
        <div className="flex flex-wrap items-center gap-2">
          {runs.length > 1 ? <select aria-label="Pipeline run" value={run?.id ?? ''} onChange={(event) => chooseRun(event.target.value)} className="h-9 max-w-52 rounded-md border bg-background px-3 text-sm">
            {runs.map((item, index) => <option key={item.id} value={item.id}>{index === 0 ? 'Latest · ' : ''}{item.trigger} · {date.format(new Date(item.created_at))}</option>)}
          </select> : null}
          <Button disabled={busy} onClick={() => runStep('')}><Play /> Run full pipeline</Button>
        </div>
      </CardHeader>
      {!run ? <CardContent className="space-y-4"><p className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">This historical article has no run yet. Start the full pipeline to process its stored source snapshot.</p></CardContent> : (
        <div>
          <div className="border-b bg-muted/25">
            <div className="flex items-center justify-between gap-3 px-4 py-3">
              <p className="truncate text-xs text-muted-foreground">Drag to rearrange · Arrow keys move a focused node · Click to inspect</p>
              <Button variant="outline" size="sm" className="shrink-0 bg-background/90" onClick={() => setPositions(structuredClone(initialPositions))}><RotateCcw /> Reset layout</Button>
            </div>
            <div className="overflow-x-auto">
            <div ref={canvasRef} className="relative mx-auto h-[300px] w-[1080px]" aria-label="Article processing workflow">
              <svg className="pointer-events-none absolute inset-0" width={canvas.width} height={canvas.height} aria-hidden="true">
                {nodes.slice(1).map((node, index) => {
                  const previous = nodes[index]!
                  const from = pointFor(previous.key, index)
                  const to = pointFor(node.key, index + 1)
                  const startX = from.x + canvas.nodeWidth
                  const startY = from.y + canvas.nodeHeight / 2
                  const endX = to.x
                  const endY = to.y + canvas.nodeHeight / 2
                  const middle = (startX + endX) / 2
                  return <path key={`${previous.key}-${node.key}`} className="pipeline-edge" data-active={node.status === 'running' || node.status === 'succeeded'} d={`M ${startX} ${startY} C ${middle} ${startY}, ${middle} ${endY}, ${endX} ${endY}`} />
                })}
              </svg>
              {nodes.map((node, index) => <PipelineNode
                key={node.key} label={node.label} detail={node.detail} status={node.status} icon={node.icon}
                position={index + 1} point={pointFor(node.key, index)}
                selected={selected === node.id} onClick={() => setSelected(node.id)}
                onPointerDown={(event) => startDrag(event, node.key)} onPointerMove={dragNode}
                onPointerUp={endDrag} onPointerCancel={endDrag} onKeyDown={(event) => moveWithKeyboard(event, node.key)}
                hasInput={index > 0} hasOutput={index < nodes.length - 1}
              />)}
            </div>
            </div>
          </div>
          <div className="grid xl:grid-cols-[minmax(0,1fr)_minmax(24rem,0.9fr)]">
            <div className="min-w-0 p-5 lg:p-6">
              <dl className="mt-6 grid gap-3 border-t pt-5 text-sm sm:grid-cols-3">
                <Detail label="Run ID" value={run.id} />
                <Detail label="Trigger" value={run.trigger} />
                <Detail label="Started" value={run.started_at ? date.format(new Date(run.started_at)) : 'Waiting'} />
              </dl>
              {run.last_error ? <div className="mt-5 flex gap-3 rounded-xl border border-destructive/30 bg-destructive/5 p-4 text-sm"><TriangleAlert className="mt-0.5 size-4 shrink-0 text-destructive" /><div><p className="font-medium">Last failure</p><p className="mt-1 break-words text-muted-foreground">{run.last_error}</p></div></div> : null}
            </div>
          <aside className="border-t bg-muted/15 p-5 xl:border-l xl:border-t-0 lg:p-6" aria-live="polite">
            {selectedStep ? <StepInspector step={selectedStep} calls={selectedCalls} running={busy} onRun={() => runStep(selectedStep.name)} /> : <SourceInspector source={source} running={busy} onRun={() => runStep('source')} />}
          </aside>
        </div>
        </div>
      )}
    </Card>
  )
}

function PipelineNode({ label, detail, status, icon: Icon, selected, onClick, position, point, hasInput, hasOutput, ...events }: {
  label: string; detail: string; status: string; icon: typeof Rss; selected: boolean; onClick: () => void; position: number
  point: { x: number; y: number }; hasInput: boolean; hasOutput: boolean
  onPointerDown: (event: PointerEvent<HTMLButtonElement>) => void; onPointerMove: (event: PointerEvent<HTMLButtonElement>) => void
  onPointerUp: (event: PointerEvent<HTMLButtonElement>) => void; onPointerCancel: (event: PointerEvent<HTMLButtonElement>) => void
  onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => void
}) {
  const StatusIcon = status === 'succeeded' ? Check : status === 'failed' ? X : status === 'running' ? LoaderCircle : status === 'skipped' ? CircleDashed : Clock3
  return (
    <button type="button" aria-pressed={selected} aria-label={`${label}, ${status}. Drag to move or select to inspect.`} onClick={onClick} {...events}
      className={`group absolute z-10 cursor-grab touch-none rounded-lg border p-4 text-left shadow-sm transition-[border-color,box-shadow] active:cursor-grabbing ${selected ? 'border-primary bg-card ring-2 ring-primary/20' : 'bg-card hover:border-primary/40 hover:shadow-md'}`}
      style={{ left: point.x, top: point.y, width: canvas.nodeWidth, height: canvas.nodeHeight }}>
      {hasInput ? <span className="absolute -left-2 top-1/2 size-3 -translate-y-1/2 rounded-full border-2 border-background bg-muted-foreground" aria-hidden="true" /> : null}
      {hasOutput ? <span className="absolute -right-2 top-1/2 size-3 -translate-y-1/2 rounded-full border-2 border-background bg-muted-foreground" aria-hidden="true" /> : null}
      <div className="flex items-center justify-between gap-3">
        <span className={`flex size-9 items-center justify-center rounded-lg ${selected ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'}`}><Icon className="size-4" /></span>
        <span className={`flex size-6 items-center justify-center rounded-full ${status === 'succeeded' ? 'bg-primary text-primary-foreground' : status === 'failed' ? 'bg-destructive text-white' : status === 'running' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'}`}><StatusIcon className={`size-3.5 ${status === 'running' ? 'animate-spin' : ''}`} /></span>
      </div>
      <p className="mt-2 text-xs text-muted-foreground">Step {position}</p>
      <p className="mt-0.5 font-medium">{label}</p>
      <p className="mt-0.5 truncate text-xs capitalize text-muted-foreground">{detail}</p>
    </button>
  )
}

function StepInspector({ step, calls, running, onRun }: { step: PipelineStep; calls: LLMCall[]; running: boolean; onRun: () => void }) {
  const skippedReason = typeof step.output.reason === 'string' ? step.output.reason : 'This step was not required for this run.'
  return (
    <div className="space-y-5">
      <div className="flex items-start justify-between gap-3">
        <div><p className="text-lg font-semibold">{stepLabels[step.name] ?? step.name}</p><p className="mt-1 text-xs capitalize text-muted-foreground">{step.status} · attempt {step.attempt}/{step.max_attempts}</p></div>
        <Button variant="outline" size="sm" disabled={running} onClick={onRun}><Play /> Run step</Button>
      </div>
      {step.status === 'queued' ? <p className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-3 text-xs text-muted-foreground"><strong className="text-foreground">Waiting for the analysis worker.</strong> This is queued, not broken; VPS inference runs one article at a time.</p> : null}
      {step.status === 'skipped' ? <p className="rounded-lg border bg-muted/40 p-3 text-xs text-muted-foreground"><strong className="text-foreground">Skipped for this run.</strong> {skippedReason}</p> : null}
      <dl className="grid grid-cols-2 gap-x-4 gap-y-3 border-y py-4 text-sm">
        <Detail label="Started" value={step.started_at ? date.format(new Date(step.started_at)) : 'Waiting'} />
        <Detail label="Duration" value={step.duration_ms == null ? step.status === 'running' ? 'In progress' : '—' : formatDuration(step.duration_ms)} />
      </dl>
      <div>
        <p className="text-sm font-medium">Execution log</p>
        {step.logs.length ? <ol className="mt-3 space-y-0">{step.logs.map((log) => (
          <li key={log.id} className="group relative grid grid-cols-[1rem_1fr] gap-3 pb-4 last:pb-0">
            <span className={`relative z-10 mt-1 size-2.5 rounded-full ${log.level === 'error' ? 'bg-destructive' : 'bg-primary'}`} />
            <span className="absolute bottom-0 left-[4px] top-3 w-px bg-border group-last:hidden" />
            <details className="group/log min-w-0"><summary className="flex cursor-pointer list-none items-start justify-between gap-3 [&::-webkit-details-marker]:hidden"><span className="text-sm font-medium group-hover/log:text-primary">{log.message}</span><time className="shrink-0 text-[11px] text-muted-foreground">{date.format(new Date(log.created_at))}</time></summary>{Object.keys(log.details).length ? <pre className="mt-2 max-h-40 overflow-auto rounded-lg bg-muted/60 p-2 text-[11px] leading-5 text-muted-foreground">{JSON.stringify(log.details, null, 2)}</pre> : null}</details>
          </li>
        ))}</ol> : <p className="mt-3 rounded-lg border border-dashed p-3 text-xs text-muted-foreground">No append-only events were captured for this older run. Its current state is still shown above.</p>}
      </div>
      {calls.map((call) => <LLMCallInspector key={call.id} call={call} />)}
      {step.error_detail ? <LogBlock title="Error detail" value={step.error_detail} error /> : null}
      {Object.keys(step.output).length ? <LogBlock title="Step output" value={JSON.stringify(step.output, null, 2)} /> : null}
    </div>
  )
}

function SourceInspector({ source, running, onRun }: { source: { name: string; icon: string; endpoint: string; receivedAt: string; rightsMode: string }; running: boolean; onRun: () => void }) {
  return <div className="space-y-5"><div className="flex items-start justify-between gap-3"><div className="flex items-center gap-3"><SourceAvatar name={source.name} iconUrl={source.icon} className="size-10" /><div><p className="text-lg font-semibold">Source intake</p><p className="text-xs text-muted-foreground">Captured from {source.name}</p></div></div><Button variant="outline" size="sm" disabled={running} onClick={onRun}><Play /> Run source</Button></div><p className="rounded-lg border bg-muted/40 p-3 text-xs text-muted-foreground">Running this step polls the source endpoint. Processing steps use the latest article snapshot stored at intake.</p><dl className="grid gap-4 border-y py-4 text-sm"><Detail label="Received" value={date.format(new Date(source.receivedAt))} /><Detail label="Endpoint" value={source.endpoint} /><Detail label="Rights mode" value={source.rightsMode.replaceAll('_', ' ')} /></dl><div><p className="text-sm font-medium">Origin log</p><ol className="mt-3 space-y-4 text-sm"><li className="border-l-2 border-primary pl-3"><p className="font-medium">Endpoint payload received</p><p className="mt-1 text-xs text-muted-foreground">{date.format(new Date(source.receivedAt))}</p></li><li className="border-l-2 border-primary pl-3"><p className="font-medium">Article normalized and stored</p><p className="mt-1 text-xs text-muted-foreground">Ready for categorization</p></li></ol></div></div>
}

function stepDetail(step: PipelineStep) {
  if (step.duration_ms != null) return formatDuration(step.duration_ms)
  if (step.status === 'queued') return 'Waiting for worker'
  if (step.status === 'skipped' && typeof step.output.reason === 'string') return step.output.reason
  return step.status
}

function LLMCallInspector({ call }: { call: LLMCall }) {
  return <div className="space-y-3 border-t pt-5"><div className="flex items-center justify-between gap-3"><p className="text-sm font-medium">LLM request</p><Badge variant={call.outcome === 'ok' ? 'outline' : call.outcome === 'running' ? 'secondary' : 'destructive'}>{call.outcome}</Badge></div><dl className="grid grid-cols-2 gap-x-4 gap-y-3 text-sm"><Detail label="Provider / model" value={`${call.provider_id} / ${call.model}`} /><Detail label="Transport" value={call.streamed ? 'Streaming' : 'Buffered'} /><Detail label="First token" value={call.first_token_ms == null ? '—' : formatDuration(call.first_token_ms)} /><Detail label="Total latency" value={call.latency_ms == null ? 'In progress' : formatDuration(call.latency_ms)} /><Detail label="Tokens" value={formatTokens(call)} /><Detail label="Finish reason" value={call.finish_reason || '—'} /></dl>{call.error_detail ? <LogBlock title="Provider error" value={call.error_detail} error /> : null}{call.response_text ? <LogBlock title="Raw response" value={call.response_text} /> : null}</div>
}

function LogBlock({ title, value, error = false }: { title: string; value: string; error?: boolean }) {
  return <div><p className={`text-xs font-medium ${error ? 'text-destructive' : ''}`}>{title}</p><pre className={`mt-2 max-h-64 overflow-auto rounded-lg border p-3 text-[11px] leading-5 ${error ? 'border-destructive/30 bg-destructive/5 text-destructive' : 'border-zinc-700 bg-zinc-950 text-zinc-200'}`}>{value}</pre></div>
}

function formatTokens(call: Pick<LLMCall, 'input_tokens' | 'output_tokens'>) {
  if (call.input_tokens == null && call.output_tokens == null) return 'Tokens unavailable'
  return `${call.input_tokens ?? 0} in · ${call.output_tokens ?? 0} out`
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
        {political.evidence?.length ? <div><p className="text-sm font-medium">Evidence</p><ul className="mt-2 space-y-2 text-sm text-muted-foreground">{political.evidence.map((evidence) => <li key={evidence} className="border-l-2 pl-3">“{evidence}”</li>)}</ul></div> : null}
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
