import { createClient, type KnowledgeEvent, type KnowledgeGraph } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import {
  CalendarRangeIcon,
  ExternalLinkIcon,
  GitMergeIcon,
  MinusIcon,
  MoveIcon,
  NetworkIcon,
  NewspaperIcon,
  PlusIcon,
  RadioTowerIcon,
  RotateCcwIcon,
  ScaleIcon,
  ShieldCheckIcon,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router'
import { Label, Pie, PieChart } from 'recharts'

import { SourceAvatar } from '@/components/source-avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from '@/components/ui/empty'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ScrollArea, ScrollBar } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import {
  initialGraphViewport,
  layoutKnowledgeGraph,
  zoomGraphViewport,
} from '@/lib/knowledge-graph'

const client = createClient()
const dayOptions = [1, 7, 30] as const
const dateFormatter = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' })
const rangeDateFormatter = new Intl.DateTimeFormat('en', { month: 'short', day: 'numeric' })
type PoliticalParty = KnowledgeGraph['political']['parties'][number]

export function KnowledgeGraphPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedDays = Number(searchParams.get('days') || 1)
  const days = dayOptions.includes(requestedDays as 1 | 7 | 30) ? (requestedDays as 1 | 7 | 30) : 1
  const from = searchParams.get('from') || ''
  const to = searchParams.get('to') || ''
  const customRange = isDateInputValue(from) && isDateInputValue(to) && from <= to ? { from, to } : undefined
  const category = searchParams.get('category') || ''
  const [selectedID, setSelectedID] = useState('')
  const graph = useQuery({
    queryKey: ['knowledge-graph', days, customRange?.from, customRange?.to, category],
    queryFn: () => client.knowledgeGraph(customRange
      ? { ...customRange, category }
      : { days, category }),
  })
  const selected = graph.data?.events.find((event) => event.id === selectedID)
    ?? graph.data?.events.find((event) => event.articles.length > 1)
    ?? graph.data?.events[0]

  function setFilter(key: string, value: string) {
    const next = new URLSearchParams(searchParams)
    if (value) next.set(key, value)
    else next.delete(key)
    setSearchParams(next, { replace: true })
  }

  function setDays(value: 1 | 7 | 30) {
    const next = new URLSearchParams(searchParams)
    next.set('days', String(value))
    next.delete('from')
    next.delete('to')
    setSearchParams(next, { replace: true })
  }

  function setCustomRange(fromDate: string, toDate: string) {
    const next = new URLSearchParams(searchParams)
    next.delete('days')
    next.set('from', fromDate)
    next.set('to', toDate)
    setSearchParams(next, { replace: true })
  }

  return (
    <section className="flex flex-col gap-6">
      <div className="flex flex-col justify-between gap-4 xl:flex-row xl:items-end">
        <div className="space-y-2">
          <Badge variant="outline" className="gap-1.5">
            <NetworkIcon /> Semantic intelligence
          </Badge>
          <div>
            <h1 className="font-heading text-2xl font-semibold tracking-tight">News knowledge graph</h1>
            <p className="mt-1 max-w-3xl text-sm text-muted-foreground">
              See categories, shared events, and reporting sources as one live newsroom timeline.
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex rounded-4xl bg-muted/60 p-1" aria-label="Graph time range">
            {dayOptions.map((option) => (
              <Button
                key={option}
                size="sm"
                variant={!customRange && days === option ? 'default' : 'ghost'}
                onClick={() => setDays(option)}
              >
                {option === 1 ? '24 hours' : `${option} days`}
              </Button>
            ))}
            <CustomDateRange
              from={customRange?.from}
              to={customRange?.to}
              onApply={setCustomRange}
            />
          </div>
          <Select
            value={category || 'all'}
            onValueChange={(value) => {
              if (value !== null) setFilter('category', value === 'all' ? '' : value)
            }}
          >
            <SelectTrigger className="min-w-44" aria-label="Filter graph by category">
              <SelectValue>
                {() => graph.data?.categories.find((item) => item.slug === category)?.name_en ?? 'All categories'}
              </SelectValue>
            </SelectTrigger>
            <SelectContent align="end">
              <SelectItem value="all">All categories</SelectItem>
              {graph.data?.categories.map((item) => (
                <SelectItem key={item.slug} value={item.slug}>{item.name_en}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <SummaryCards data={graph.data} loading={graph.isPending} />

      <Card className="gap-0 py-0 shadow-sm">
        <CardHeader className="grid-cols-1! border-b py-6 sm:grid-cols-[1fr_auto]!">
          <CardTitle>Live event map</CardTitle>
          <CardDescription>
            Categories feed events; events connect to every publisher reporting the story.
          </CardDescription>
          <CardAction className="col-start-1 row-span-1 row-start-3 mt-2 flex items-center gap-3 justify-self-start text-xs text-muted-foreground sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0 sm:justify-self-end">
            <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-primary" />Multi-source</span>
            <span className="flex items-center gap-1.5"><span className="size-2 rounded-full bg-muted-foreground/50" />Single report</span>
          </CardAction>
        </CardHeader>
        <CardContent className="px-0">
          <div className="min-h-[560px] overflow-hidden bg-[radial-gradient(circle_at_center,var(--color-border)_1px,transparent_1px)] bg-[size:22px_22px]">
            {graph.isPending ? <Skeleton className="m-6 h-[512px]" /> : null}
            {graph.isError ? (
              <div className="flex h-[560px] items-center justify-center text-sm text-muted-foreground">
                The knowledge graph is temporarily unavailable.
              </div>
            ) : null}
            {graph.data && graph.data.events.length === 0 ? (
              <div className="flex h-[560px] items-center justify-center text-sm text-muted-foreground">
                No published events were found in this window.
              </div>
            ) : null}
            {graph.data?.events.length ? (
              <EventGraph data={graph.data} selectedID={selected?.id ?? ''} onSelect={setSelectedID} />
            ) : null}
          </div>
          <EventArticleRail event={selected} parties={graph.data?.political.parties ?? []} />
        </CardContent>
      </Card>

      <PoliticalSpectrum data={graph.data} />
      <CategoryBreakdown data={graph.data} />
    </section>
  )
}

function CustomDateRange({
  from: initialFrom,
  to: initialTo,
  onApply,
}: {
  from?: string
  to?: string
  onApply: (from: string, to: string) => void
}) {
  const today = toDateInputValue(new Date())
  const defaultStart = new Date()
  defaultStart.setDate(defaultStart.getDate() - 6)
  const [open, setOpen] = useState(false)
  const [from, setFrom] = useState(initialFrom || toDateInputValue(defaultStart))
  const [to, setTo] = useState(initialTo || today)
  const rangeError = from && to && from > to ? 'The end date must be on or after the start date.' : ''

  function handleOpenChange(nextOpen: boolean) {
    if (nextOpen) {
      setFrom(initialFrom || toDateInputValue(defaultStart))
      setTo(initialTo || today)
    }
    setOpen(nextOpen)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger render={<Button size="sm" variant={initialFrom && initialTo ? 'default' : 'ghost'} />}>
        <CalendarRangeIcon data-icon="inline-start" />
        {initialFrom && initialTo
          ? `${formatRangeDate(initialFrom)} – ${formatRangeDate(initialTo)}`
          : 'Custom'}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Custom date range</DialogTitle>
          <DialogDescription>
            Show events published on any day within this range, including both selected dates.
          </DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-6"
          onSubmit={(event) => {
            event.preventDefault()
            if (rangeError) return
            onApply(from, to)
            setOpen(false)
          }}
        >
          <FieldGroup className="gap-4 sm:grid sm:grid-cols-2">
            <Field data-invalid={rangeError ? true : undefined}>
              <FieldLabel htmlFor="knowledge-from">From</FieldLabel>
              <Input
                id="knowledge-from"
                type="date"
                value={from}
                max={to || today}
                onChange={(event) => setFrom(event.target.value)}
                aria-invalid={rangeError ? true : undefined}
                required
              />
            </Field>
            <Field data-invalid={rangeError ? true : undefined}>
              <FieldLabel htmlFor="knowledge-to">To</FieldLabel>
              <Input
                id="knowledge-to"
                type="date"
                value={to}
                min={from}
                max={today}
                onChange={(event) => setTo(event.target.value)}
                aria-invalid={rangeError ? true : undefined}
                required
              />
            </Field>
          </FieldGroup>
          {rangeError ? <FieldError>{rangeError}</FieldError> : null}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>Cancel</Button>
            <Button type="submit" disabled={Boolean(rangeError)}>Apply range</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function SummaryCards({ data, loading }: { data?: KnowledgeGraph; loading: boolean }) {
  const items = [
    { label: 'Published reports', value: data?.summary.articles, icon: NewspaperIcon },
    { label: 'Distinct events', value: data?.summary.events, icon: NetworkIcon },
    { label: 'Cross-source events', value: data?.summary.multi_source_events, icon: GitMergeIcon },
    { label: 'Reporting sources', value: data?.summary.sources, icon: RadioTowerIcon },
  ]
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      {items.map((item) => (
        <Card key={item.label} className="bg-linear-to-b from-card to-primary/[0.035] shadow-sm">
          <CardHeader>
            <CardDescription>{item.label}</CardDescription>
            <CardTitle className="text-3xl font-semibold tabular-nums">
              {loading ? <Skeleton className="h-9 w-20" /> : (item.value ?? 0).toLocaleString()}
            </CardTitle>
            <CardAction><item.icon className="size-5 text-muted-foreground" /></CardAction>
          </CardHeader>
        </Card>
      ))}
    </div>
  )
}

function EventGraph({
  data,
  selectedID,
  onSelect,
}: {
  data: KnowledgeGraph
  selectedID: string
  onSelect: (id: string) => void
}) {
  const width = 1240
  const height = 560
  const graph = useMemo(() => layoutKnowledgeGraph(data, width, height), [data])
  const positions = useMemo(() => new Map(graph.nodes.map((node) => [node.id, node])), [graph.nodes])
  const events = useMemo(() => new Map(data.events.map((event) => [event.id, event])), [data.events])
  const [viewport, setViewport] = useState(initialGraphViewport)
  const drag = useRef<{ pointerID: number; x: number; y: number } | null>(null)
  const svg = useRef<SVGSVGElement>(null)

  useEffect(() => {
    const element = svg.current
    if (!element) return
    function handleWheel(event: WheelEvent) {
      event.preventDefault()
      const bounds = (event.currentTarget as SVGSVGElement).getBoundingClientRect()
      const point = {
        x: ((event.clientX - bounds.left) / bounds.width) * width,
        y: ((event.clientY - bounds.top) / bounds.height) * height,
      }
      setViewport((current) => zoomGraphViewport(
        current,
        current.scale * (event.deltaY < 0 ? 1.12 : 0.89),
        point,
      ))
    }
    element.addEventListener('wheel', handleWheel, { passive: false })
    return () => element.removeEventListener('wheel', handleWheel)
  }, [])

  function zoom(nextScale: number, point = { x: width / 2, y: height / 2 }) {
    setViewport((current) => zoomGraphViewport(current, nextScale, point))
  }

  return (
    <div className="relative">
      <div className="absolute top-4 right-4 z-10 flex items-center gap-1 rounded-2xl border bg-background/90 p-1 shadow-sm backdrop-blur">
        <Button size="icon" variant="ghost" aria-label="Zoom out" onClick={() => zoom(viewport.scale / 1.25)}>
          <MinusIcon />
        </Button>
        <span className="w-12 text-center text-xs tabular-nums text-muted-foreground" aria-live="polite">
          {Math.round(viewport.scale * 100)}%
        </span>
        <Button size="icon" variant="ghost" aria-label="Zoom in" onClick={() => zoom(viewport.scale * 1.25)}>
          <PlusIcon />
        </Button>
        <Button size="icon" variant="ghost" aria-label="Reset graph view" onClick={() => setViewport(initialGraphViewport)}>
          <RotateCcwIcon />
        </Button>
      </div>
      <div className="pointer-events-none absolute bottom-4 left-4 z-10 hidden items-center gap-2 rounded-xl border bg-background/85 px-3 py-2 text-xs text-muted-foreground backdrop-blur sm:flex">
        <MoveIcon className="size-3.5" /> Drag to move · Scroll to zoom
      </div>
      <svg
        ref={svg}
        viewBox={`0 0 ${width} ${height}`}
        className="h-[560px] w-full cursor-grab touch-none overscroll-contain select-none active:cursor-grabbing"
        style={{ touchAction: 'none' }}
        role="img"
        aria-label="Interactive knowledge graph of news categories, events, and sources. Drag to pan and scroll to zoom."
        onPointerDown={(event) => {
          if ((event.target as SVGElement).closest('[data-graph-node]')) return
          event.currentTarget.setPointerCapture(event.pointerId)
          drag.current = { pointerID: event.pointerId, x: event.clientX, y: event.clientY }
        }}
        onPointerMove={(event) => {
          if (drag.current?.pointerID !== event.pointerId) return
          const bounds = event.currentTarget.getBoundingClientRect()
          const factor = width / bounds.width
          const deltaX = (event.clientX - drag.current.x) * factor
          const deltaY = (event.clientY - drag.current.y) * factor
          drag.current = { pointerID: event.pointerId, x: event.clientX, y: event.clientY }
          setViewport((current) => ({ ...current, x: current.x + deltaX, y: current.y + deltaY }))
        }}
        onPointerUp={(event) => {
          if (drag.current?.pointerID !== event.pointerId) return
          drag.current = null
          if (event.currentTarget.hasPointerCapture(event.pointerId)) {
            event.currentTarget.releasePointerCapture(event.pointerId)
          }
        }}
        onPointerCancel={() => { drag.current = null }}
      >
        <title>News categories connected to clustered events and their reporting sources</title>
        <g transform={`translate(${viewport.x} ${viewport.y}) scale(${viewport.scale})`}>
          <g aria-hidden="true" className="pointer-events-none">
            {graph.edges.map((edge, index) => {
              const source = positions.get(edge.source)
              const target = positions.get(edge.target)
              if (!source || !target) return null
              return (
                <line
                  key={`${edge.source}-${edge.target}-${index}`}
                  x1={source.x}
                  y1={source.y}
                  x2={target.x}
                  y2={target.y}
                  className={edge.kind === 'category' ? 'stroke-primary/15' : 'stroke-foreground/8'}
                  strokeWidth={edge.kind === 'category' ? 1.25 : 1}
                />
              )
            })}
          </g>
          {graph.nodes.map((node) => {
        if (node.kind === 'category') {
          return (
            <g key={node.id} aria-hidden="true" data-graph-node>
              <circle cx={node.x} cy={node.y} r={node.radius} className="fill-primary/15 stroke-primary" />
              <text x={node.x + node.radius + 8} y={node.y + 4} className="fill-foreground text-[11px] font-medium">{truncate(node.label, 18)}</text>
            </g>
          )
        }
        if (node.kind === 'source') {
          return (
            <g key={node.id} aria-hidden="true" data-graph-node>
              <circle cx={node.x} cy={node.y} r={node.radius} className="fill-background stroke-muted-foreground/60" />
              <text x={node.x + 10} y={node.y + 4} className="fill-muted-foreground text-[10px]">{truncate(node.label, 20)}</text>
            </g>
          )
        }
        const event = node.eventId ? events.get(node.eventId) : undefined
        const selected = node.eventId === selectedID
        const multiSource = new Set(event?.articles.map((article) => article.source_id)).size > 1
        return (
          <g
            key={node.id}
            data-graph-node
            role="button"
            tabIndex={0}
            aria-label={`${event?.title ?? 'Event'}, ${event?.articles.length ?? 0} reports`}
            className="cursor-pointer outline-none"
            onClick={() => node.eventId && onSelect(node.eventId)}
            onKeyDown={(keyboardEvent) => {
              if ((keyboardEvent.key === 'Enter' || keyboardEvent.key === ' ') && node.eventId) onSelect(node.eventId)
            }}
          >
            <circle
              cx={node.x}
              cy={node.y}
              r={node.radius + (selected ? 4 : 0)}
              className={selected ? 'fill-primary/10 stroke-primary' : 'fill-transparent stroke-transparent'}
              strokeWidth={2}
            />
            <circle
              cx={node.x}
              cy={node.y}
              r={node.radius}
              className={multiSource ? 'fill-primary stroke-primary-foreground/40' : 'fill-muted-foreground/45 stroke-background'}
              strokeWidth={2}
            />
            <title>{event?.title}</title>
            {selected ? (
              <text x={node.x + node.radius + 8} y={node.y - node.radius - 3} className="fill-foreground text-[11px] font-medium">
                {truncate(event?.title ?? '', 46)}
              </text>
            ) : null}
          </g>
        )
          })}
        </g>
      </svg>
    </div>
  )
}

function EventArticleRail({ event, parties }: { event?: KnowledgeEvent; parties: PoliticalParty[] }) {
  const drag = useRef<{ pointerId: number; startX: number; scrollLeft: number; moved: boolean } | null>(null)
  const suppressClick = useRef(false)
  const sourceCount = new Set(event?.articles.map((article) => article.source_id)).size
  const partyNames = new Map(parties.map((party) => [party.slug, party.short_name]))
  return (
    <section className="border-t px-6 py-5" aria-label="Articles reporting the selected event">
      {!event ? <p className="text-sm text-muted-foreground">Select an event node to inspect it.</p> : (
        <div className="space-y-4">
          <div className="flex flex-col justify-between gap-3 lg:flex-row lg:items-end">
            <div className="min-w-0 space-y-1.5">
              <div className="flex flex-wrap gap-2">
                <Badge variant="secondary">{event.category.replaceAll('_', ' ')}</Badge>
                {sourceCount > 1 ? <Badge>{sourceCount} sources</Badge> : <Badge variant="outline">Single report</Badge>}
                {event.is_breaking ? <Badge variant="destructive">Breaking</Badge> : null}
              </div>
              <h2 className="font-heading text-lg font-semibold leading-snug">{event.title}</h2>
              <p className="text-xs text-muted-foreground">
                Updated {formatDate(event.last_update_at)} · {Math.round(event.confidence * 100)}% cluster confidence
              </p>
            </div>
            <p className="shrink-0 text-xs text-muted-foreground">{event.articles.length} relevant reports · {event.algorithm_version}</p>
          </div>
          <ScrollArea
            className="h-[190px] w-full cursor-grab select-none active:cursor-grabbing"
            aria-label="Drag or scroll through relevant article cards"
            onDragStart={(pointerEvent) => pointerEvent.preventDefault()}
            onPointerDown={(pointerEvent) => {
              if (pointerEvent.pointerType !== 'mouse' || pointerEvent.button !== 0) return
              if ((pointerEvent.target as HTMLElement).closest('[data-slot="scroll-area-scrollbar"]')) return
              const viewport = pointerEvent.currentTarget.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]')
              if (!viewport) return
              suppressClick.current = false
              drag.current = {
                pointerId: pointerEvent.pointerId,
                startX: pointerEvent.clientX,
                scrollLeft: viewport.scrollLeft,
                moved: false,
              }
              pointerEvent.currentTarget.setPointerCapture(pointerEvent.pointerId)
            }}
            onPointerMove={(pointerEvent) => {
              if (!drag.current || drag.current.pointerId !== pointerEvent.pointerId) return
              const viewport = pointerEvent.currentTarget.querySelector<HTMLElement>('[data-slot="scroll-area-viewport"]')
              if (!viewport) return
              const distance = pointerEvent.clientX - drag.current.startX
              drag.current.moved ||= Math.abs(distance) > 4
              if (!drag.current.moved) return
              pointerEvent.preventDefault()
              viewport.scrollLeft = drag.current.scrollLeft - distance
            }}
            onPointerUp={(pointerEvent) => {
              if (!drag.current || drag.current.pointerId !== pointerEvent.pointerId) return
              suppressClick.current = drag.current.moved
              drag.current = null
              pointerEvent.currentTarget.releasePointerCapture(pointerEvent.pointerId)
            }}
            onPointerCancel={() => {
              drag.current = null
              suppressClick.current = false
            }}
            onClickCapture={(clickEvent) => {
              if (!suppressClick.current) return
              clickEvent.preventDefault()
              clickEvent.stopPropagation()
              suppressClick.current = false
            }}
          >
            <div className="flex w-max snap-x snap-mandatory gap-3 pb-3">
              {event.articles.map((article) => (
                <a
                  key={article.id}
                  href={article.original_url}
                  target="_blank"
                  rel="noreferrer"
                  draggable={false}
                  className="group flex min-h-36 w-[285px] shrink-0 snap-start flex-col justify-between rounded-2xl border bg-card p-4 transition-colors hover:bg-muted/50 sm:w-[340px]"
                >
                <span className="flex items-start gap-3">
                  <SourceAvatar name={article.source} iconUrl={article.source_icon} className="size-9 shrink-0" />
                  <span className="min-w-0 flex-1">
                    <span className="block text-xs text-muted-foreground">{article.source}</span>
                    <span className="mt-1 line-clamp-3 block text-sm font-medium leading-snug">{article.headline}</span>
                  </span>
                </span>
                {article.political ? (
                  <span className="mt-3 flex flex-wrap items-center gap-1.5">
                    {article.political.mentions.map((mention) => (
                      <Badge
                        key={mention.party_slug}
                        variant="outline"
                        title={`${partyNames.get(mention.party_slug) ?? mention.party_slug}: ${stanceLabel(mention.stance)}`}
                      >
                        {partyNames.get(mention.party_slug) ?? mention.party_slug.toUpperCase()}
                      </Badge>
                    ))}
                    <span className="text-[11px] text-muted-foreground">
                      {article.political.confidence >= 0.45
                        ? frameLabel(article.political.economic_frame)
                        : 'Framing unclear'}
                    </span>
                  </span>
                ) : null}
                <span className="mt-4 flex items-center justify-between text-xs text-muted-foreground">
                  <span>{formatDate(article.published_at)}</span>
                  <ExternalLinkIcon className="size-3.5 group-hover:text-foreground" />
                </span>
                </a>
              ))}
            </div>
            <ScrollBar orientation="horizontal" />
          </ScrollArea>
        </div>
      )}
    </section>
  )
}

function PoliticalSpectrum({ data }: { data?: KnowledgeGraph }) {
  const political = data?.political
  const relatedArticles = data?.events.reduce(
    (total, event) => total + event.articles.filter((article) => article.political).length,
    0,
  ) ?? 0
  return (
    <Card className="gap-0 py-0 shadow-sm">
      <CardHeader className="grid-cols-1! border-b py-6 sm:grid-cols-[1fr_auto]!">
        <CardTitle>Political framing monitor</CardTitle>
        <CardDescription>
          Compare an evidence-backed party baseline with how current reporting frames those parties.
        </CardDescription>
        <CardAction className="col-start-1 row-span-1 row-start-3 mt-2 flex items-center gap-2 justify-self-start sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0 sm:justify-self-end">
          <Badge variant="outline" className="gap-1.5"><ScaleIcon />Economic axis</Badge>
          <Badge variant="secondary">{relatedArticles} related reports</Badge>
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-6 py-6">
        <div className="grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(320px,0.65fr)]">
          <section className="rounded-2xl border bg-muted/20 p-5" aria-labelledby="party-spectrum-title">
            <div className="space-y-1">
              <h2 id="party-spectrum-title" className="font-heading font-semibold">Sri Lankan party baseline</h2>
              <p className="text-xs text-muted-foreground">Provisional economic-policy position · hover or focus a party for its rationale</p>
            </div>
            <div className="relative mt-5 h-44" aria-label="Political parties arranged from far left to right">
              <div className="absolute top-[78px] right-[5%] left-[5%] h-px bg-border" />
              <div className="absolute top-[73px] left-1/2 h-3 w-px bg-foreground/50" />
              {political?.parties.map((party, index) => (
                <button
                  key={party.slug}
                  type="button"
                  className="group absolute -translate-x-1/2 text-center outline-none"
                  style={{
                    left: spectrumPosition(party.economic_position),
                    top: index % 2 === 0 ? 30 : 82,
                  }}
                  title={`${party.name_en}: ${party.rationale}`}
                  aria-label={`${party.name_en}, ${spectrumPositionLabel(party.economic_position)}. ${party.rationale}`}
                >
                  <span className={index % 2 === 0 ? 'mb-2 block' : 'mt-2 block'}>
                    <span className="inline-flex rounded-full border bg-background px-2 py-1 text-xs font-semibold shadow-xs group-focus-visible:ring-2 group-focus-visible:ring-ring">
                      {party.short_name}
                    </span>
                    <span className="mt-1 block text-[10px] tabular-nums text-muted-foreground">{party.economic_position.toFixed(2)}</span>
                  </span>
                  <span className={index % 2 === 0 ? 'absolute top-[45px] left-1/2 h-3 w-px bg-primary' : 'absolute -top-[9px] left-1/2 h-3 w-px bg-primary'} />
                  <span className={index % 2 === 0 ? 'absolute top-[54px] left-1/2 size-2 -translate-x-1/2 rounded-full bg-primary' : 'absolute -top-[14px] left-1/2 size-2 -translate-x-1/2 rounded-full bg-primary'} />
                </button>
              ))}
              <div className="absolute right-[3%] bottom-0 left-[3%] flex justify-between text-[11px] text-muted-foreground">
                <span>Far left · state-led</span><span>Center</span><span>Right · market-led</span>
              </div>
            </div>
          </section>

          <section className="rounded-2xl border p-5" aria-labelledby="source-framing-title">
            <div className="flex items-start justify-between gap-4">
              <div className="space-y-1">
                <h2 id="source-framing-title" className="font-heading font-semibold">Outlet framing tendency</h2>
                <p className="text-xs text-muted-foreground">Only shown after {political?.minimum_sample ?? 5} scored reports</p>
              </div>
              <Badge variant="outline">Shrunk average</Badge>
            </div>
            <div className="mt-5 space-y-4">
              {political?.sources.length ? political.sources.slice(0, 8).map((source) => (
                <div key={source.source_id} className="space-y-2">
                  <div className="flex items-center gap-2">
                    <SourceAvatar name={source.source} iconUrl={source.source_icon} className="size-7" />
                    <span className="min-w-0 flex-1 truncate text-sm font-medium">{source.source}</span>
                    <span className="text-[11px] tabular-nums text-muted-foreground">
                      {source.qualified ? frameLabel(source.economic_frame) : `${source.scored_articles}/${political.minimum_sample} scored`}
                    </span>
                  </div>
                  <div className="relative h-1.5 rounded-full bg-muted">
                    <span className="absolute top-[-2px] left-1/2 h-2.5 w-px bg-border" />
                    {source.qualified ? (
                      <span
                        className="absolute top-1/2 size-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-background bg-primary shadow-sm"
                        style={{ left: spectrumPosition(source.economic_frame) }}
                      />
                    ) : (
                      <span
                        className="block h-full rounded-full bg-muted-foreground/25"
                        style={{ width: `${Math.min(100, (source.scored_articles / political.minimum_sample) * 100)}%` }}
                      />
                    )}
                  </div>
                  {!source.qualified ? (
                    <p className="text-[10px] text-muted-foreground">{source.mentioned_articles} party-related reports; insufficient directional evidence</p>
                  ) : null}
                </div>
              )) : (
                <p className="rounded-xl bg-muted/50 p-4 text-sm text-muted-foreground">No party-related reporting in this window yet.</p>
              )}
            </div>
          </section>
        </div>

        <div className="grid gap-3 rounded-2xl border bg-primary/[0.035] p-4 text-xs text-muted-foreground md:grid-cols-3">
          <p><strong className="text-foreground">Party position</strong><br />A curated economic-policy baseline with evidence and confidence—not a universal truth score.</p>
          <p><strong className="text-foreground">Article framing</strong><br />Favorable or critical language near a party mention; mentioning a party alone does not imply bias.</p>
          <p className="flex gap-2"><ShieldCheckIcon className="mt-0.5 size-4 shrink-0 text-primary" /><span><strong className="text-foreground">Outlet safeguard</strong><br />Low samples remain “insufficient” and the aggregate is pulled toward neutral to avoid premature labels.</span></p>
        </div>
      </CardContent>
    </Card>
  )
}

function CategoryBreakdown({ data }: { data?: KnowledgeGraph }) {
  const categories = data?.categories ?? []
  const total = categories.reduce((sum, category) => sum + category.articles, 0)
  const colors = ['var(--primary)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)']
  const chartConfig: ChartConfig = Object.fromEntries(categories.map((category, index) => [
    category.slug,
    { label: category.name_en, color: colors[index % colors.length] },
  ]))
  const chartData = categories.map((category, index) => ({
    ...category,
    fill: `var(--color-${category.slug})`,
    share: total ? Math.round((category.articles / total) * 100) : 0,
    color: colors[index % colors.length],
  }))
  return (
    <Card className="gap-0 py-0 shadow-sm">
      <CardHeader className="grid-cols-1! border-b py-6 sm:grid-cols-[1fr_auto]!">
        <CardTitle>Category distribution</CardTitle>
        <CardDescription>See which newsroom topics dominate the selected reporting window.</CardDescription>
        <CardAction className="col-start-1 row-span-1 row-start-3 mt-2 justify-self-start sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0 sm:justify-self-end">
          <Badge variant="secondary">{total.toLocaleString()} categorized reports</Badge>
        </CardAction>
      </CardHeader>
      <CardContent className="py-6">
        {!data ? <Skeleton className="h-[320px] w-full" /> : null}
        {data && categories.length === 0 ? (
          <Empty className="min-h-72 border">
            <EmptyHeader>
              <EmptyMedia variant="icon"><NetworkIcon /></EmptyMedia>
              <EmptyTitle>No category distribution yet</EmptyTitle>
              <EmptyDescription>No categorized reports were published in this window.</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : null}
        {categories.length ? (
          <div className="grid items-center gap-8 lg:grid-cols-[minmax(280px,0.8fr)_minmax(0,1.2fr)]">
            <ChartContainer
              config={chartConfig}
              className="mx-auto h-[310px] w-full max-w-[390px]"
              role="img"
              aria-label="Donut chart showing the share of published reports in each category"
            >
              <PieChart accessibilityLayer>
                <ChartTooltip
                  cursor={false}
                  content={<ChartTooltipContent hideLabel nameKey="slug" />}
                />
                <Pie
                  data={chartData}
                  dataKey="articles"
                  nameKey="slug"
                  innerRadius={82}
                  outerRadius={124}
                  paddingAngle={2}
                  strokeWidth={3}
                >
                  <Label
                    content={({ viewBox }) => {
                      if (!viewBox || !('cx' in viewBox) || !('cy' in viewBox)) return null
                      return (
                        <text x={viewBox.cx} y={viewBox.cy} textAnchor="middle" dominantBaseline="middle">
                          <tspan x={viewBox.cx} y={viewBox.cy} className="fill-foreground text-3xl font-semibold tabular-nums">
                            {total.toLocaleString()}
                          </tspan>
                          <tspan x={viewBox.cx} y={(viewBox.cy ?? 0) + 24} className="fill-muted-foreground text-xs">
                            reports
                          </tspan>
                        </text>
                      )
                    }}
                  />
                </Pie>
              </PieChart>
            </ChartContainer>
            <div className="grid gap-3 sm:grid-cols-2">
              {chartData.map((category) => (
                <div key={category.slug} className="flex items-center gap-3 rounded-2xl border bg-muted/20 p-3">
                  <span className="size-2.5 shrink-0 rounded-full" style={{ backgroundColor: category.color }} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">{category.name_en}</span>
                    <span className="text-xs text-muted-foreground">{category.events} events</span>
                  </span>
                  <span className="text-right">
                    <span className="block text-sm font-semibold tabular-nums">{category.share}%</span>
                    <span className="text-[11px] tabular-nums text-muted-foreground">{category.articles} reports</span>
                  </span>
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

function formatDate(value: string) {
  return dateFormatter.format(new Date(value))
}

function formatRangeDate(value: string) {
  return rangeDateFormatter.format(new Date(`${value}T00:00:00`))
}

function isDateInputValue(value: string) {
  return /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(Date.parse(`${value}T00:00:00Z`))
}

function toDateInputValue(value: Date) {
  const year = value.getFullYear()
  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function truncate(value: string, length: number) {
  return value.length > length ? `${value.slice(0, length - 1)}…` : value
}

function spectrumPosition(value: number) {
  return `${5 + ((value + 1) / 2) * 90}%`
}

function spectrumPositionLabel(value: number) {
  if (value <= -0.65) return 'far left'
  if (value < -0.15) return 'center-left'
  if (value <= 0.15) return 'center'
  if (value < 0.65) return 'center-right'
  return 'right'
}

function stanceLabel(value: number) {
  if (value > 0.2) return 'favorable framing'
  if (value < -0.2) return 'critical framing'
  return 'neutral or unclear framing'
}

function frameLabel(value: number) {
  if (value < -0.15) return 'Left-framed'
  if (value > 0.15) return 'Right-framed'
  return 'Neutral / mixed'
}
