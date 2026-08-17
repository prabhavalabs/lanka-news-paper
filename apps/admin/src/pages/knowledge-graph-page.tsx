import { createClient, type KnowledgeEvent, type KnowledgeGraph } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import {
  ExternalLinkIcon,
  GitMergeIcon,
  MinusIcon,
  MoveIcon,
  NetworkIcon,
  NewspaperIcon,
  PlusIcon,
  RadioTowerIcon,
  RotateCcwIcon,
} from 'lucide-react'
import { useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router'

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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  initialGraphViewport,
  layoutKnowledgeGraph,
  zoomGraphViewport,
} from '@/lib/knowledge-graph'

const client = createClient()
const dayOptions = [1, 7, 30] as const
const dateFormatter = new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' })

export function KnowledgeGraphPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedDays = Number(searchParams.get('days') || 1)
  const days = dayOptions.includes(requestedDays as 1 | 7 | 30) ? (requestedDays as 1 | 7 | 30) : 1
  const category = searchParams.get('category') || ''
  const [selectedID, setSelectedID] = useState('')
  const graph = useQuery({
    queryKey: ['knowledge-graph', days, category],
    queryFn: () => client.knowledgeGraph(days, category),
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
                variant={days === option ? 'default' : 'ghost'}
                onClick={() => setFilter('days', String(option))}
              >
                {option === 1 ? '24 hours' : `${option} days`}
              </Button>
            ))}
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
        <CardHeader className="border-b py-6">
          <CardTitle>Live event map</CardTitle>
          <CardDescription>
            Categories feed events; events connect to every publisher reporting the story.
          </CardDescription>
          <CardAction className="flex items-center gap-3 text-xs text-muted-foreground">
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
          <EventArticleRail event={selected} />
        </CardContent>
      </Card>

      <CategoryBreakdown data={graph.data} />
    </section>
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

  function graphPoint(element: SVGSVGElement, clientX: number, clientY: number) {
    const bounds = element.getBoundingClientRect()
    return {
      x: ((clientX - bounds.left) / bounds.width) * width,
      y: ((clientY - bounds.top) / bounds.height) * height,
    }
  }

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
        viewBox={`0 0 ${width} ${height}`}
        className="h-[560px] w-full cursor-grab select-none active:cursor-grabbing"
        style={{ touchAction: 'none' }}
        role="img"
        aria-label="Interactive knowledge graph of news categories, events, and sources. Drag to pan and scroll to zoom."
        onWheel={(event) => {
          event.preventDefault()
          const point = graphPoint(event.currentTarget, event.clientX, event.clientY)
          zoom(viewport.scale * (event.deltaY < 0 ? 1.12 : 0.89), point)
        }}
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

function EventArticleRail({ event }: { event?: KnowledgeEvent }) {
  const sourceCount = new Set(event?.articles.map((article) => article.source_id)).size
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
          <div className="flex snap-x snap-mandatory gap-3 overflow-x-auto pb-3" tabIndex={0} aria-label="Scrollable relevant article cards">
            {event.articles.map((article) => (
              <a
                key={article.id}
                href={article.original_url}
                target="_blank"
                rel="noreferrer"
                className="group flex min-h-36 w-[285px] shrink-0 snap-start flex-col justify-between rounded-2xl border bg-card p-4 transition-colors hover:bg-muted/50 sm:w-[340px]"
              >
                <span className="flex items-start gap-3">
                  <SourceAvatar name={article.source} iconUrl={article.source_icon} className="size-9 shrink-0" />
                  <span className="min-w-0 flex-1">
                    <span className="block text-xs text-muted-foreground">{article.source}</span>
                    <span className="mt-1 line-clamp-3 block text-sm font-medium leading-snug">{article.headline}</span>
                  </span>
                </span>
                <span className="mt-4 flex items-center justify-between text-xs text-muted-foreground">
                  <span>{formatDate(article.published_at)}</span>
                  <ExternalLinkIcon className="size-3.5 group-hover:text-foreground" />
                </span>
              </a>
            ))}
          </div>
        </div>
      )}
    </section>
  )
}

function CategoryBreakdown({ data }: { data?: KnowledgeGraph }) {
  const maximum = Math.max(1, ...(data?.categories.map((category) => category.articles) ?? [1]))
  return (
    <Card className="gap-0 py-0 shadow-sm">
      <CardHeader className="border-b py-6">
        <CardTitle>Category distribution</CardTitle>
        <CardDescription>Semantic coverage for this window.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 py-6">
        {data?.categories.map((category) => (
          <div key={category.slug} className="space-y-1.5">
            <div className="flex justify-between gap-4 text-sm">
              <span className="font-medium">{category.name_en}</span>
              <span className="tabular-nums text-muted-foreground">{category.articles} reports</span>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full bg-muted">
              <div className="h-full rounded-full bg-primary" style={{ width: `${Math.max(3, (category.articles / maximum) * 100)}%` }} />
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

function formatDate(value: string) {
  return dateFormatter.format(new Date(value))
}

function truncate(value: string, length: number) {
  return value.length > length ? `${value.slice(0, length - 1)}…` : value
}
