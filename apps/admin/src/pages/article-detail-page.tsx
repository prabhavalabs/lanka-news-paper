import {
  createClient, type AdminArticleDetail, type EventNarrativeAnalysis,
  type LLMCall, type PipelineRun, type PipelineStep,
} from '@snap/api-client'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft, BrainCircuit, Check, CircleDashed, Clock3, Eraser, ExternalLink,
  FileText, GitBranch, GripVertical, Layers3, LoaderCircle, Network, Play,
  RotateCcw, Rss, Scale, ServerCog, Tags, TriangleAlert, X,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import { Link, useParams } from 'react-router'
import { toast } from 'sonner'

import { SourceAvatar } from '@/components/source-avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea, ScrollBar } from '@/components/ui/scroll-area'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import {
  PIPELINE_NODE_HEIGHT, PIPELINE_NODE_WIDTH, buildPipelineEdges, layoutPipelineGraph,
  type PipelineGraphEdge, type PipelineGraphPoint,
} from '@/lib/pipeline-graph'
import { cn } from '@/lib/utils'

const client = createClient()
const date = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' })
const stepLabels: Record<string, string> = {
  content_cleaning: 'Content cleaning',
  summarization: 'Article summarization',
  categorization: 'Categorization',
  event_clustering: 'Event clustering',
  stance_evaluation: 'Stance evaluation',
  event_synthesis: 'Event synthesis',
  narration_analysis: 'Legacy narration analysis',
}
const stepIcons = {
  content_cleaning: Eraser,
  summarization: FileText,
  categorization: Tags,
  event_clustering: Network,
  stance_evaluation: Scale,
  event_synthesis: Layers3,
  narration_analysis: BrainCircuit,
}

type PipelineGraphNode = {
  key: string
  label: string
  detail: string
  status: string
  icon: typeof Rss
  id: string
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
        <Metric label="Stance" value={political?.relevant ? political.label.replaceAll('_', ' ') : political ? 'Unrated' : 'Pending'} detail={political ? stanceDetail(political) : 'Waiting for evaluation'} icon={Scale} />
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

      {item.event_analysis ? <EventSpectrumCard analysis={item.event_analysis} /> : null}

      <ContentVersionsCard content={item.content} analysis={item.analysis_document} />

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
  const [selected, setSelected] = useState<string | null>(null)
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
  const edges = run ? buildPipelineEdges(nodes.map((node) => node.key), run.steps) : []

  function chooseRun(id: string) {
    const next = runs.find((item) => item.id === id)
    const currentStepName = selected === 'source' ? 'source' : run?.steps.find((step) => step.id === selected)?.name
    setSelectedRun(id)
    if (currentStepName === 'source') {
      setSelected('source')
      return
    }
    setSelected(next?.steps.find((step) => step.name === currentStepName)?.id ?? next?.steps[0]?.id ?? 'source')
  }

  function runStep(step: string) {
    setSelectedRun('')
    setSelected(null)
    onRun(step)
  }

  return (
    <>
      <Card className="overflow-hidden gap-0 py-0 shadow-sm">
        <CardHeader className="gap-4 border-b py-4 sm:flex sm:flex-row sm:items-center sm:justify-between sm:py-5">
          <div>
            <CardTitle>Processing pipeline</CardTitle>
            <CardDescription>Inspect the progress and output of each step in the pipeline.</CardDescription>
          </div>
          <Button className="w-full sm:w-auto" disabled={busy} onClick={() => runStep('')}><Play /> Run full pipeline</Button>
        </CardHeader>
        {!run ? (
          <CardContent className="p-5 sm:p-6">
            <p className="rounded-2xl border border-dashed p-6 text-sm text-muted-foreground">This historical article has no run yet. Start the full pipeline to process its stored source snapshot.</p>
          </CardContent>
        ) : (
          <CardContent className="bg-muted/10 p-4 sm:p-5 lg:p-6">
            <PipelineGraph nodes={nodes} edges={edges} selected={selected} onSelect={setSelected} />
          </CardContent>
        )}
      </Card>

      {run ? (
        <Sheet open={selected !== null} onOpenChange={(open) => { if (!open) setSelected(null) }}>
          <SheetContent
            side="right"
            style={{ width: '100%', maxWidth: '42rem' }}
            className="pipeline-inspector-sheet overflow-hidden"
          >
            <SheetHeader className="sr-only">
              <SheetTitle>{selectedStep ? stepLabels[selectedStep.name] ?? selectedStep.name : 'Source intake'}</SheetTitle>
              <SheetDescription>Pipeline step execution details and controls.</SheetDescription>
            </SheetHeader>
            <ScrollArea className="min-h-0 flex-1">
              <div className="space-y-7 p-5 pt-16 sm:p-7 sm:pt-16">
                {selectedStep ? (
                  <StepInspector step={selectedStep} calls={selectedCalls} running={busy} onRun={() => runStep(selectedStep.name)} />
                ) : (
                  <SourceInspector source={source} running={busy} onRun={() => runStep('source')} />
                )}
                <RunContext run={run} runs={runs} onChangeRun={chooseRun} />
              </div>
            </ScrollArea>
          </SheetContent>
        </Sheet>
      ) : null}
    </>
  )
}

function PipelineGraph({ nodes, edges, selected, onSelect }: {
  nodes: PipelineGraphNode[]
  edges: PipelineGraphEdge[]
  selected: string | null
  onSelect: (id: string) => void
}) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const drag = useRef<{
    key: string
    pointerId: number
    startClientX: number
    startClientY: number
    startPoint: PipelineGraphPoint
    moved: boolean
  } | null>(null)
  const lastDragEnd = useRef(0)
  const [viewportWidth, setViewportWidth] = useState(0)
  const nodeKey = nodes.map((node) => node.key).join('|')
  const edgeKey = edges.map((edge) => `${edge.from}>${edge.to}`).join('|')
  const layout = useMemo(
    () => layoutPipelineGraph(nodes.map((node) => node.key), edges, viewportWidth),
    // Primitive topology keys keep drag interactions from rebuilding the layout.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [nodeKey, edgeKey, viewportWidth],
  )
  const [positions, setPositions] = useState<Record<string, PipelineGraphPoint>>({})

  useEffect(() => {
    const viewport = scrollRef.current?.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]')
    if (!viewport) return
    const updateWidth = () => setViewportWidth(viewport.clientWidth)
    updateWidth()
    const observer = new ResizeObserver(updateWidth)
    observer.observe(viewport)
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    setPositions(layout.positions)
  }, [nodeKey, edgeKey, viewportWidth, layout.positions])

  function pointFor(key: string) {
    return positions[key] ?? layout.positions[key] ?? { x: 24, y: 24 }
  }

  function moveNode(key: string, x: number, y: number) {
    setPositions((current) => ({
      ...current,
      [key]: {
        x: Math.max(16, Math.min(layout.width - PIPELINE_NODE_WIDTH - 16, x)),
        y: Math.max(16, Math.min(layout.height - PIPELINE_NODE_HEIGHT - 16, y)),
      },
    }))
  }

  function startDrag(event: PointerEvent<HTMLButtonElement>, key: string) {
    if (event.pointerType === 'touch' || event.button !== 0) return
    event.preventDefault()
    event.currentTarget.focus()
    event.currentTarget.setPointerCapture(event.pointerId)
    drag.current = {
      key,
      pointerId: event.pointerId,
      startClientX: event.clientX,
      startClientY: event.clientY,
      startPoint: pointFor(key),
      moved: false,
    }
  }

  function dragNode(event: PointerEvent<HTMLButtonElement>) {
    const active = drag.current
    if (!active || active.pointerId !== event.pointerId) return
    const deltaX = event.clientX - active.startClientX
    const deltaY = event.clientY - active.startClientY
    if (!active.moved && Math.hypot(deltaX, deltaY) < 4) return
    active.moved = true
    event.preventDefault()
    moveNode(active.key, active.startPoint.x + deltaX, active.startPoint.y + deltaY)
  }

  function endDrag(event: PointerEvent<HTMLButtonElement>) {
    if (drag.current?.pointerId !== event.pointerId) return
    if (drag.current.moved) lastDragEnd.current = performance.now()
    drag.current = null
  }

  function moveWithKeyboard(event: KeyboardEvent<HTMLButtonElement>, key: string) {
    const amount = event.shiftKey ? 24 : 8
    const movement = ({
      ArrowLeft: [-amount, 0], ArrowRight: [amount, 0],
      ArrowUp: [0, -amount], ArrowDown: [0, amount],
    } as Record<string, [number, number]>)[event.key]
    if (!movement) return
    event.preventDefault()
    const point = pointFor(key)
    moveNode(key, point.x + movement[0], point.y + movement[1])
  }

  function resetLayout() {
    setPositions(layout.positions)
    scrollRef.current?.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]')?.scrollTo({ left: 0, top: 0 })
  }

  function selectNode(id: string) {
    if (performance.now() - lastDragEnd.current < 250) return
    onSelect(id)
  }

  const statusByKey = new Map(nodes.map((node) => [node.key, node.status]))

  return (
    <div>
      <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <p id="pipeline-graph-instructions" className="flex items-center gap-2 text-xs text-muted-foreground">
          <GripVertical className="size-3.5" />
          <span className="hidden sm:inline">Drag steps to explore the flow · Select one for details</span>
          <span className="sm:hidden">Swipe through the flow · Select a step for details</span>
        </p>
        <Button variant="ghost" size="sm" className="h-8 w-fit self-end text-xs sm:self-auto" onClick={resetLayout}>
          <RotateCcw /> Center steps
        </Button>
      </div>
      <ScrollArea
        ref={scrollRef}
        className="w-full rounded-2xl border bg-background/55 shadow-inner"
        style={{ height: layout.height + 10 }}
        aria-describedby="pipeline-graph-instructions"
      >
        <div
          className="relative bg-[radial-gradient(circle_at_center,var(--border)_1px,transparent_1px)] bg-[size:18px_18px]"
          style={{ width: layout.width, height: layout.height }}
        >
          <svg className="pointer-events-none absolute inset-0 size-full overflow-visible" aria-hidden="true">
            <defs>
              <marker id="pipeline-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                <path d="M 0 0 L 8 4 L 0 8 z" fill="var(--border)" />
              </marker>
              <marker id="pipeline-arrow-active" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                <path d="M 0 0 L 8 4 L 0 8 z" fill="var(--primary)" />
              </marker>
            </defs>
            {edges.map((edge) => {
              const from = pointFor(edge.from)
              const to = pointFor(edge.to)
              const startX = from.x + PIPELINE_NODE_WIDTH
              const startY = from.y + PIPELINE_NODE_HEIGHT / 2
              const endX = to.x
              const endY = to.y + PIPELINE_NODE_HEIGHT / 2
              const curve = Math.max(12, Math.abs(endX - startX) / 2)
              const active = ['running', 'succeeded'].includes(statusByKey.get(edge.to) ?? '')
              return (
                <path
                  key={`${edge.from}-${edge.to}`}
                  d={`M ${startX} ${startY} C ${startX + curve} ${startY}, ${endX - curve} ${endY}, ${endX} ${endY}`}
                  className={cn('fill-none stroke-border transition-colors', active && 'stroke-primary/80')}
                  strokeWidth="2"
                  markerEnd={active ? 'url(#pipeline-arrow-active)' : 'url(#pipeline-arrow)'}
                  vectorEffect="non-scaling-stroke"
                />
              )
            })}
          </svg>
          <ol className="absolute inset-0 list-none" aria-label="Article processing workflow">
            {nodes.map((node, index) => (
              <li key={node.key} className="contents">
                <PipelineNode
                  label={node.label}
                  detail={node.detail}
                  status={node.status}
                  icon={node.icon}
                  position={index + 1}
                  point={pointFor(node.key)}
                  selected={selected === node.id}
                  onClick={() => selectNode(node.id)}
                  onPointerDown={(event) => startDrag(event, node.key)}
                  onPointerMove={dragNode}
                  onPointerUp={endDrag}
                  onPointerCancel={endDrag}
                  onKeyDown={(event) => moveWithKeyboard(event, node.key)}
                />
              </li>
            ))}
          </ol>
        </div>
        <ScrollBar orientation="horizontal" />
      </ScrollArea>
    </div>
  )
}

function PipelineNode({ label, detail, status, icon: Icon, selected, onClick, position, point, ...events }: {
  label: string
  detail: string
  status: string
  icon: typeof Rss
  selected: boolean
  onClick: () => void
  position: number
  point: PipelineGraphPoint
  onPointerDown: (event: PointerEvent<HTMLButtonElement>) => void
  onPointerMove: (event: PointerEvent<HTMLButtonElement>) => void
  onPointerUp: (event: PointerEvent<HTMLButtonElement>) => void
  onPointerCancel: (event: PointerEvent<HTMLButtonElement>) => void
  onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => void
}) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      aria-label={`Open details for ${label}, ${status}`}
      onClick={onClick}
      style={{
        left: point.x,
        top: point.y,
        width: PIPELINE_NODE_WIDTH,
        height: PIPELINE_NODE_HEIGHT,
      }}
      className={cn(
        'group absolute z-10 flex cursor-grab touch-manipulation select-none flex-col rounded-2xl border bg-card p-3 text-left shadow-sm transition-[border-color,box-shadow,transform] hover:-translate-y-0.5 hover:border-primary/50 hover:shadow-lg active:cursor-grabbing focus-visible:ring-2 focus-visible:ring-primary/30',
        selected && 'border-primary shadow-md ring-2 ring-primary/20',
      )}
      {...events}
    >
      <span className="flex items-start justify-between gap-3">
        <span className={cn('flex size-10 shrink-0 items-center justify-center rounded-xl bg-muted text-muted-foreground transition-colors group-hover:bg-primary/10 group-hover:text-primary', selected && 'bg-primary text-primary-foreground group-hover:bg-primary group-hover:text-primary-foreground')}>
          <Icon className="size-4" />
        </span>
        <span className="flex items-center gap-1">
          <GripVertical className="size-4 text-muted-foreground/60 opacity-50 transition-opacity group-hover:opacity-100" aria-hidden="true" />
          <PipelineStatusIcon status={status} />
        </span>
      </span>
      <span className="mt-auto min-w-0">
        <span className="block text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Step {position}</span>
        <span className="mt-1 line-clamp-2 block min-h-10 font-heading font-semibold leading-5">{label}</span>
        <span className="mt-0.5 block truncate text-xs capitalize text-muted-foreground">{detail}</span>
      </span>
    </button>
  )
}

function PipelineStatusIcon({ status }: { status: string }) {
  const StatusIcon = status === 'succeeded' ? Check : status === 'failed' ? X : status === 'running' ? LoaderCircle : status === 'skipped' ? CircleDashed : Clock3
  return (
    <span className={cn(
      'flex size-7 items-center justify-center rounded-full bg-muted text-muted-foreground',
      status === 'succeeded' && 'bg-primary text-primary-foreground',
      status === 'failed' && 'bg-destructive text-white',
      status === 'running' && 'bg-primary/10 text-primary',
    )}>
      <StatusIcon className={cn('size-4', status === 'running' && 'animate-spin')} />
    </span>
  )
}

function RunContext({ run, runs, onChangeRun }: { run: PipelineRun; runs: PipelineRun[]; onChangeRun: (id: string) => void }) {
  return (
    <section className="border-t pt-6" aria-labelledby="pipeline-run-context">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h3 id="pipeline-run-context" className="font-heading text-sm font-semibold">Run context</h3>
          <p className="mt-1 text-xs text-muted-foreground">The pipeline execution that produced these step details.</p>
        </div>
        {runs.length > 1 ? (
          <select
            aria-label="Pipeline run"
            value={run.id}
            onChange={(event) => onChangeRun(event.target.value)}
            className="h-9 max-w-full rounded-2xl border bg-background px-3 text-xs sm:max-w-64"
          >
            {runs.map((item, index) => <option key={item.id} value={item.id}>{index === 0 ? 'Latest · ' : ''}{item.trigger} · {date.format(new Date(item.created_at))}</option>)}
          </select>
        ) : null}
      </div>
      <dl className="mt-5 grid gap-4 rounded-2xl border bg-muted/15 p-4 text-sm sm:grid-cols-3">
        <Detail label="Run ID" value={run.id} />
        <Detail label="Trigger" value={run.trigger} />
        <Detail label="Started" value={run.started_at ? date.format(new Date(run.started_at)) : 'Waiting'} />
      </dl>
      {run.last_error ? <div className="mt-4 flex gap-3 rounded-2xl border border-destructive/30 bg-destructive/5 p-4 text-sm"><TriangleAlert className="mt-0.5 size-4 shrink-0 text-destructive" /><div><p className="font-medium">Last failure</p><p className="mt-1 break-words text-muted-foreground">{run.last_error}</p></div></div> : null}
    </section>
  )
}

function StepInspector({ step, calls, running, onRun }: { step: PipelineStep; calls: LLMCall[]; running: boolean; onRun: () => void }) {
  const skippedReason = typeof step.output.reason === 'string' ? step.output.reason : 'This step was not required for this run.'
  const Icon = stepIcons[step.name as keyof typeof stepIcons] ?? GitBranch
  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 items-center gap-3 pr-10 sm:pr-0">
          <span className="flex size-12 shrink-0 items-center justify-center rounded-2xl bg-primary text-primary-foreground"><Icon className="size-5" /></span>
          <div className="min-w-0">
            <p className="font-heading text-xl font-semibold leading-tight">{stepLabels[step.name] ?? step.name}</p>
            <div className="mt-1 flex items-center gap-2 text-xs capitalize text-muted-foreground"><PipelineStatusIcon status={step.status} /><span>{step.status} · attempt {step.attempt}/{step.max_attempts}</span></div>
          </div>
        </div>
        <Button className="w-full sm:w-auto" variant="outline" size="sm" disabled={running} onClick={onRun}><Play /> Run step</Button>
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
            <details className="group/log min-w-0"><summary className="flex cursor-pointer list-none flex-col items-start gap-1 sm:flex-row sm:justify-between sm:gap-3 [&::-webkit-details-marker]:hidden"><span className="text-sm font-medium group-hover/log:text-primary">{log.message}</span><time className="shrink-0 text-[11px] text-muted-foreground">{date.format(new Date(log.created_at))}</time></summary>{Object.keys(log.details).length ? <pre className="mt-2 max-h-40 overflow-auto rounded-lg bg-muted/60 p-2 text-[11px] leading-5 text-muted-foreground">{JSON.stringify(log.details, null, 2)}</pre> : null}</details>
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
  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 items-center gap-3 pr-10 sm:pr-0">
          <SourceAvatar name={source.name} iconUrl={source.icon} className="size-12" />
          <div className="min-w-0"><p className="font-heading text-xl font-semibold">Source intake</p><p className="mt-1 truncate text-xs text-muted-foreground">Captured from {source.name}</p></div>
        </div>
        <Button className="w-full sm:w-auto" variant="outline" size="sm" disabled={running} onClick={onRun}><Play /> Run source</Button>
      </div>
      <p className="rounded-2xl border bg-muted/40 p-4 text-xs leading-5 text-muted-foreground">Running this step polls the source endpoint. Processing steps use the latest article snapshot stored at intake.</p>
      <dl className="grid gap-4 border-y py-5 text-sm sm:grid-cols-2"><Detail label="Received" value={date.format(new Date(source.receivedAt))} /><Detail label="Endpoint" value={source.endpoint} /><Detail label="Rights mode" value={source.rightsMode.replaceAll('_', ' ')} /></dl>
      <div><p className="text-sm font-medium">Origin log</p><ol className="mt-4 space-y-4 text-sm"><li className="border-l-2 border-primary pl-3"><p className="font-medium">Endpoint payload received</p><p className="mt-1 text-xs text-muted-foreground">{date.format(new Date(source.receivedAt))}</p></li><li className="border-l-2 border-primary pl-3"><p className="font-medium">Article normalized and stored</p><p className="mt-1 text-xs text-muted-foreground">Ready for categorization</p></li></ol></div>
    </div>
  )
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
	if (!political) return <Card><CardHeader><CardTitle>Article stance</CardTitle><CardDescription>Waiting for the stance-evaluation step.</CardDescription></CardHeader><CardContent><Skeleton className="h-24 w-full" /></CardContent></Card>
	const left = political.left_probability * 100
	const center = political.center_probability * 100
	const right = political.right_probability * 100
	return (
		<Card>
			<CardHeader><CardTitle>Article stance</CardTitle><CardDescription>Left, center, and right probabilities for this article’s reporting frame.</CardDescription></CardHeader>
			<CardContent className="space-y-5">
				{political.relevant ? <><div className="flex items-end justify-between gap-3"><span className="text-2xl font-semibold capitalize">{political.label.replaceAll('_', ' ')}</span><span className="text-xs text-muted-foreground">{Math.round(political.confidence * 100)}% confidence</span></div><SpectrumBar left={left} center={center} right={right} /></> : <div className="rounded-xl bg-muted/50 p-4 text-sm">This article does not contain enough political or public-policy framing to place it on the spectrum.</div>}
				<div><p className="text-sm font-medium">Why</p><p className="mt-1 text-sm leading-6 text-muted-foreground">{political.rationale}</p></div>
				{political.evidence?.length ? <div><p className="text-sm font-medium">Evidence</p><ul className="mt-2 space-y-2 text-sm text-muted-foreground">{political.evidence.map((evidence) => <li key={evidence} className="border-l-2 pl-3">“{evidence}”</li>)}</ul></div> : null}
				<p className="text-xs text-muted-foreground">{political.provider_id} · {political.provider_model} · {political.axis_version}</p>
			</CardContent>
		</Card>
	)
}

function EventSpectrumCard({ analysis }: { analysis: EventNarrativeAnalysis }) {
  return (
    <Card>
      <CardHeader className="gap-3 border-b sm:flex sm:flex-row sm:items-start sm:justify-between">
        <div><CardTitle>Coverage spectrum</CardTitle><CardDescription>A cross-source synthesis that is refreshed whenever another report joins this event.</CardDescription></div>
        <Badge variant="secondary" className="w-fit">{analysis.source_count} source{analysis.source_count === 1 ? '' : 's'}</Badge>
      </CardHeader>
      <CardContent className="space-y-6 pt-6">
        <p className="max-w-5xl text-sm leading-7 text-muted-foreground">{analysis.summary}</p>
        {analysis.rated_source_count > 0 ? <SpectrumBar left={analysis.left_percentage} center={analysis.center_percentage} right={analysis.right_percentage} /> : <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">No source in this event contains enough political framing to calculate a left/center/right distribution.</div>}
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {analysis.source_spectrum.map((source) => (
            <div key={source.article_id} className="flex min-w-0 items-center gap-3 rounded-2xl border bg-muted/10 p-3">
              <SourceAvatar name={source.source} iconUrl={source.source_icon} className="size-10" />
              <div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{source.source}</p><p className="mt-1 text-xs capitalize text-muted-foreground">{source.label === 'unrated' ? 'Unrated coverage' : `${source.label} · ${Math.round(source.confidence * 100)}% confidence`}</p></div>
            </div>
          ))}
        </div>
        <p className="text-xs text-muted-foreground">{analysis.rated_source_count} of {analysis.source_count} sources had sufficient political framing to rate. Unrated sources remain visible and are not forced into left, center, or right.</p>
      </CardContent>
    </Card>
  )
}

function SpectrumBar({ left, center, right }: { left: number; center: number; right: number }) {
  return (
    <div>
      <div className="flex h-9 overflow-hidden rounded-xl border text-xs font-semibold" role="img" aria-label={`Left ${left.toFixed(1)}%, center ${center.toFixed(1)}%, right ${right.toFixed(1)}%`}>
        {left > 0 ? <span className="flex items-center justify-center bg-rose-500/75 text-white" style={{ width: `${left}%` }}>L {Math.round(left)}%</span> : null}
        {center > 0 ? <span className="flex items-center justify-center bg-zinc-200 text-zinc-900 dark:bg-zinc-100" style={{ width: `${center}%` }}>C {Math.round(center)}%</span> : null}
        {right > 0 ? <span className="flex items-center justify-center bg-blue-500/80 text-white" style={{ width: `${right}%` }}>R {Math.round(right)}%</span> : null}
      </div>
      <div className="mt-2 flex justify-between text-xs text-muted-foreground"><span>Left</span><span>Center</span><span>Right</span></div>
    </div>
  )
}

function ContentVersionsCard({ content, analysis }: {
  content: AdminArticleDetail['content']
  analysis: AdminArticleDetail['analysis_document']
}) {
  const initial = analysis?.summary_text ? 'summary' : analysis?.cleaned_text ? 'cleaned' : 'original'
  const [version, setVersion] = useState<'original' | 'cleaned' | 'summary'>(initial)
  const versions = {
    original: analysis?.original_text || content?.body_text || '',
    cleaned: analysis?.cleaned_text || '',
    summary: analysis?.summary_text || '',
  }
  const selected = versions[version]
  return (
    <Card>
      <CardHeader className="gap-4 border-b sm:flex sm:flex-row sm:items-start sm:justify-between">
        <div><CardTitle>Article content versions</CardTitle><CardDescription>Compare the captured original, deterministic clean copy, and model-generated summary.</CardDescription></div>
        <div className="grid grid-cols-3 gap-1 rounded-xl bg-muted p-1" aria-label="Article content version">
          {(['original', 'cleaned', 'summary'] as const).map((item) => <Button key={item} size="sm" variant={version === item ? 'default' : 'ghost'} disabled={!versions[item]} onClick={() => setVersion(item)} className="capitalize">{item}</Button>)}
        </div>
      </CardHeader>
      <CardContent className="space-y-5 pt-6">
        {selected ? <p className="max-h-[34rem] overflow-y-auto whitespace-pre-line rounded-2xl border bg-muted/10 p-4 text-sm leading-7">{selected}</p> : <p className="rounded-2xl border border-dashed p-6 text-sm text-muted-foreground">This version has not been produced yet.</p>}
        {version === 'summary' && analysis?.summary_points.length ? <ul className="grid gap-2 text-sm text-muted-foreground sm:grid-cols-2">{analysis.summary_points.map((point) => <li key={point} className="rounded-xl border p-3">{point}</li>)}</ul> : null}
        <dl className="grid gap-4 border-t pt-5 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <Detail label="Original acquisition" value={content ? content.acquisition_method.replaceAll('_', ' ') : 'Unavailable'} />
          <Detail label="Cleaner" value={analysis?.cleaner_version ?? 'Pending'} />
          <Detail label="Summary model" value={analysis?.summary_model || 'Pending'} />
          <Detail label="Visibility" value="Admin only" />
        </dl>
      </CardContent>
    </Card>
  )
}

function stanceDetail(political: NonNullable<AdminArticleDetail['political']>) {
  return `L ${Math.round(political.left_probability * 100)} · C ${Math.round(political.center_probability * 100)} · R ${Math.round(political.right_probability * 100)}`
}

function Metric({ label, value, detail, icon: Icon }: { label: string; value: string; detail: string; icon: typeof ServerCog }) {
  return (
    <Card className="gap-0 py-0 shadow-sm">
      <CardContent className="flex min-h-28 items-center justify-between gap-4 p-4 sm:p-5">
        <div className="min-w-0">
          <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</p>
          <p className="mt-1.5 truncate text-xl font-semibold capitalize">{value}</p>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">{detail}</p>
        </div>
        <span className="shrink-0 rounded-lg bg-primary/10 p-2 text-primary"><Icon className="size-4" /></span>
      </CardContent>
    </Card>
  )
}
function Detail({ label, value }: { label: string; value: string }) { return <div className="min-w-0"><dt className="text-xs uppercase tracking-wide text-muted-foreground">{label}</dt><dd className="mt-1 break-words capitalize">{value}</dd></div> }
function PipelineBadge({ status }: { status: string }) { return <Badge variant={status === 'failed' ? 'destructive' : status === 'succeeded' ? 'default' : 'outline'} className="capitalize">{status === 'running' ? <LoaderCircle className="animate-spin" /> : null}{status}</Badge> }
function formatDuration(ms: number) { return ms >= 60_000 ? `${(ms / 60_000).toFixed(1)}m` : ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms` }
function DetailSkeleton() { return <div className="space-y-6"><Skeleton className="h-8 w-32" /><Skeleton className="h-20 w-full max-w-4xl" /><div className="grid gap-4 md:grid-cols-4">{Array.from({ length: 4 }, (_, index) => <Skeleton key={index} className="h-28" />)}</div><Skeleton className="h-72" /></div> }
