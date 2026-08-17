import { createClient, type KnowledgeEvent, type KnowledgeGraph } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import {
  Clock3Icon,
  ExternalLinkIcon,
  GitMergeIcon,
  NetworkIcon,
  NewspaperIcon,
  RadioTowerIcon,
} from 'lucide-react'
import { useMemo, useState } from 'react'
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
import { layoutKnowledgeGraph } from '@/lib/knowledge-graph'
import { cn } from '@/lib/utils'

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
        <CardContent className="grid gap-0 px-0 2xl:grid-cols-[minmax(0,1fr)_340px]">
          <div className="min-h-[560px] overflow-x-auto bg-[radial-gradient(circle_at_center,var(--color-border)_1px,transparent_1px)] bg-[size:22px_22px]">
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
          <EventInspector event={selected} />
        </CardContent>
      </Card>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.5fr)_minmax(300px,0.5fr)]">
        <EventTimeline events={graph.data?.events ?? []} selectedID={selected?.id ?? ''} onSelect={setSelectedID} />
        <CategoryBreakdown data={graph.data} />
      </div>
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

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="h-[560px] min-w-[980px] w-full" role="img" aria-label="Knowledge graph of news categories, events, and sources">
      <title>News categories connected to clustered events and their reporting sources</title>
      <g aria-hidden="true">
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
            <g key={node.id} aria-hidden="true">
              <circle cx={node.x} cy={node.y} r={node.radius} className="fill-primary/15 stroke-primary" />
              <text x={node.x + node.radius + 8} y={node.y + 4} className="fill-foreground text-[11px] font-medium">{truncate(node.label, 18)}</text>
            </g>
          )
        }
        if (node.kind === 'source') {
          return (
            <g key={node.id} aria-hidden="true">
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
    </svg>
  )
}

function EventInspector({ event }: { event?: KnowledgeEvent }) {
  const sourceCount = new Set(event?.articles.map((article) => article.source_id)).size
  return (
    <aside className="border-t p-6 2xl:border-t-0 2xl:border-l">
      {!event ? <p className="text-sm text-muted-foreground">Select an event node to inspect it.</p> : (
        <div className="space-y-5">
          <div className="space-y-2">
            <div className="flex flex-wrap gap-2">
              <Badge variant="secondary">{event.category.replaceAll('_', ' ')}</Badge>
              {sourceCount > 1 ? <Badge variant="default">{sourceCount} sources</Badge> : <Badge variant="outline">Single report</Badge>}
              {event.is_breaking ? <Badge variant="destructive">Breaking</Badge> : null}
            </div>
            <h2 className="font-heading text-lg font-semibold leading-snug">{event.title}</h2>
            <p className="text-xs text-muted-foreground">
              Updated {formatDate(event.last_update_at)} · {Math.round(event.confidence * 100)}% cluster confidence
            </p>
          </div>
          <div className="space-y-3">
            <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Reports in this event</p>
            {event.articles.map((article) => (
              <a
                key={article.id}
                href={article.original_url}
                target="_blank"
                rel="noreferrer"
                className="group flex items-start gap-3 rounded-2xl border p-3 transition-colors hover:bg-muted/50"
              >
                <SourceAvatar name={article.source} iconUrl={article.source_icon} className="size-8 shrink-0" />
                <span className="min-w-0 flex-1">
                  <span className="block text-xs text-muted-foreground">{article.source}</span>
                  <span className="mt-0.5 line-clamp-2 block text-sm font-medium leading-snug">{article.headline}</span>
                </span>
                <ExternalLinkIcon className="mt-1 size-3.5 shrink-0 text-muted-foreground group-hover:text-foreground" />
              </a>
            ))}
          </div>
          <p className="text-[11px] text-muted-foreground">Model: {event.algorithm_version}</p>
        </div>
      )}
    </aside>
  )
}

function EventTimeline({ events, selectedID, onSelect }: { events: KnowledgeEvent[]; selectedID: string; onSelect: (id: string) => void }) {
  return (
    <Card className="gap-0 py-0 shadow-sm">
      <CardHeader className="border-b py-6">
        <CardTitle>Event timeline</CardTitle>
        <CardDescription>Newest story clusters in the selected window.</CardDescription>
        <CardAction><Badge variant="secondary">{events.length} shown</Badge></CardAction>
      </CardHeader>
      <CardContent className="divide-y px-0">
        {events.slice(0, 12).map((event) => {
          const sources = new Set(event.articles.map((article) => article.source_id)).size
          return (
            <button
              key={event.id}
              type="button"
              className={cn('flex w-full items-start gap-4 px-6 py-4 text-left transition-colors hover:bg-muted/40', selectedID === event.id && 'bg-primary/[0.05]')}
              onClick={() => onSelect(event.id)}
            >
              <span className="mt-1 flex size-8 shrink-0 items-center justify-center rounded-full bg-muted"><Clock3Icon className="size-4" /></span>
              <span className="min-w-0 flex-1">
                <span className="line-clamp-2 font-medium leading-snug">{event.title}</span>
                <span className="mt-1 block text-xs text-muted-foreground">{formatDate(event.last_update_at)} · {event.articles.length} reports · {sources} sources</span>
              </span>
              <Badge variant="outline">{event.category}</Badge>
            </button>
          )
        })}
      </CardContent>
    </Card>
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
