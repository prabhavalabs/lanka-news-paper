import { createClient, type PublicKnowledgeGraph } from '@snap/api-client'
import { layoutKnowledgeGraph } from '@snap/ui/knowledge-graph'
import { useQuery } from '@tanstack/react-query'
import { lazy, Suspense, useMemo } from 'react'
import { Link, useSearchParams } from 'react-router'

const client = createClient()
const KnowledgeGraphView = lazy(() => import('@snap/ui/knowledge-graph-view')
  .then((module) => ({ default: module.KnowledgeGraphView })))
const dayOptions = [1, 7, 30] as const
const dateFormatter = new Intl.DateTimeFormat('si-LK', { dateStyle: 'medium', timeStyle: 'short' })

export function KnowledgeAnalysisPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedDays = Number(searchParams.get('days') || 1)
  const days = dayOptions.includes(requestedDays as 1 | 7 | 30) ? requestedDays as 1 | 7 | 30 : 1
  const from = validDate(searchParams.get('from'))
  const to = validDate(searchParams.get('to'))
  const customRange = from && to && from <= to ? { from, to } : undefined
  const category = safeFilter(searchParams.get('category'))
  const source = validUUID(searchParams.get('source'))
  const requestedNode = safeNode(searchParams.get('node'))
  const graph = useQuery({
    queryKey: ['public-knowledge-graph', days, customRange?.from, customRange?.to, category, source],
    queryFn: () => client.publicKnowledgeGraph(customRange
      ? { ...customRange, category, source }
      : { days, category, source }),
  })
  const nodes = useMemo(() => graph.data ? layoutKnowledgeGraph(graph.data) : { nodes: [], edges: [] }, [graph.data])
  const selectedNode = nodes.nodes.find((node) => node.id === requestedNode)
  const selectedID = selectedNode?.id ?? ''
  const related = useMemo(() => relatedArticles(graph.data, selectedID), [graph.data, selectedID])

  function selectNode(nodeID: string) {
    const next = new URLSearchParams(searchParams)
    next.set('node', nodeID)
    if (nodeID.startsWith('category:')) {
      next.set('category', nodeID.slice(9))
      next.delete('source')
    } else if (nodeID.startsWith('source:')) {
      next.set('source', nodeID.slice(7))
      next.delete('category')
    }
    setSearchParams(next, { replace: true })
  }

  function reset() {
    const next = new URLSearchParams(searchParams)
    next.delete('category')
    next.delete('source')
    next.delete('node')
    setSearchParams(next, { replace: true })
  }

  return (
    <article>
      <div className="border-y-4 border-double border-ink py-5">
        <p className="text-[0.75rem] font-bold uppercase tracking-[0.16em] text-muted-foreground">Public analysis</p>
        <div className="mt-2 flex flex-col justify-between gap-4 md:flex-row md:items-end">
          <div>
            <h1 className="font-headline text-[length:var(--text-h1)] font-bold leading-tight">News knowledge graph</h1>
            <p className="mt-2 max-w-3xl text-sm text-muted-foreground">
              Explore how published stories connect across topics, events, and reporting sources.
            </p>
          </div>
          <button type="button" onClick={reset} className="w-fit border border-ink px-4 py-2 text-sm font-bold">
            Reset complete view
          </button>
        </div>
      </div>

      {graph.isPending ? <p className="py-16 text-center">පූරණය වේ…</p> : null}
      {graph.isError ? <p className="py-16 text-center">මෙම විශ්ලේෂණය දැනට ලබාගත නොහැක.</p> : null}
      {graph.data ? (
        <>
          <dl className="grid grid-cols-2 border-b border-rule md:grid-cols-4">
            {[
              ['Published reports', graph.data.summary.articles],
              ['Events', graph.data.summary.events],
              ['Cross-source events', graph.data.summary.multi_source_events],
              ['Sources', graph.data.summary.sources],
            ].map(([label, value]) => (
              <div key={label} className="border-e border-rule px-4 py-5 last:border-e-0">
                <dt className="text-xs text-muted-foreground">{label}</dt>
                <dd className="font-headline text-3xl font-bold tabular-nums">{value}</dd>
              </div>
            ))}
          </dl>
          <section className="mt-8 border-y border-rule" aria-labelledby="graph-title">
            <div className="flex flex-col justify-between gap-2 border-b border-rule py-4 md:flex-row md:items-end">
              <div>
                <h2 id="graph-title" className="font-headline text-2xl font-bold">Selected network</h2>
                <p className="text-sm text-muted-foreground">Select any topic, story, or source to isolate its direct connections.</p>
              </div>
              <p className="text-xs text-muted-foreground">{rangeLabel(graph.data, customRange)}</p>
            </div>
            <Suspense fallback={<div className="min-h-[calc(100svh-2rem)] animate-pulse bg-tint" />}>
              <KnowledgeGraphView
                data={graph.data}
                selectedID={selectedID}
                onSelect={selectNode}
                onReset={reset}
                variant="public"
              />
            </Suspense>
          </section>
          <AnalysisDetails data={graph.data} selectedID={selectedID} articles={related} />
        </>
      ) : null}
    </article>
  )
}

function AnalysisDetails({ data, selectedID, articles }: { data: PublicKnowledgeGraph; selectedID: string; articles: ReturnType<typeof relatedArticles> }) {
  const nodeLabel = selectedID.startsWith('category:')
    ? data.categories.find((item) => `category:${item.slug}` === selectedID)?.name_en
    : selectedID.startsWith('source:')
      ? articles[0]?.source
      : data.events.find((item) => `event:${item.id}` === selectedID)?.title
  const event = selectedID.startsWith('event:') ? data.events.find((item) => `event:${item.id}` === selectedID) : undefined
  return (
    <section className="py-8" aria-labelledby="related-title">
      <p className="text-xs font-bold uppercase tracking-[0.14em] text-muted-foreground">Selection origin</p>
      <h2 id="related-title" className="font-headline mt-1 text-3xl font-bold">{nodeLabel ?? 'Related published reports'}</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        {event
          ? `${event.category_name_si} → ${new Set(event.articles.map((item) => item.source_id)).size} reporting source(s)`
          : selectedID ? `${articles.length} directly connected public report(s)` : `${articles.length} recent public report(s)`}
      </p>
      {articles.length ? (
        <div className="mt-6 grid gap-px bg-rule md:grid-cols-2">
          {articles.map((article) => (
            <Link key={article.id} to={`/a/${article.id}`} className="flex min-h-48 flex-col justify-between bg-paper p-5 hover:bg-tint">
              <span>
                <span className="text-xs font-bold uppercase tracking-wide text-muted-foreground">{article.source}</span>
                <span className="font-headline mt-2 block text-xl font-bold leading-snug">{article.headline}</span>
              </span>
              <span className="mt-6 flex flex-wrap items-end justify-between gap-3 text-xs text-muted-foreground">
                <span>{dateFormatter.format(new Date(article.published_at))}</span>
                {article.narrative ? <span>{narrativeLabel(article.narrative.economic_frame)} · {Math.round(article.narrative.confidence * 100)}%</span> : null}
              </span>
            </Link>
          ))}
        </div>
      ) : <p className="mt-6 text-sm text-muted-foreground">No public reports are connected to this selection.</p>}
      <p className="mt-6 border-l-4 border-ink pl-4 text-xs text-muted-foreground">
        This public view contains published metadata and high-level narrative scores only. Administrative logs, model/provider details, internal evidence, and source-management data are excluded.
      </p>
    </section>
  )
}

function relatedArticles(data: PublicKnowledgeGraph | undefined, selectedID: string) {
  if (!data) return []
  const all = data.events.flatMap((event) => event.articles)
  if (selectedID.startsWith('event:')) return data.events.find((event) => `event:${event.id}` === selectedID)?.articles ?? []
  if (selectedID.startsWith('source:')) return all.filter((article) => `source:${article.source_id}` === selectedID)
  if (selectedID.startsWith('category:')) {
    return data.events.filter((event) => `category:${event.category}` === selectedID).flatMap((event) => event.articles)
  }
  return all.slice(0, 24)
}

function validDate(value: string | null) {
  return value && /^\d{4}-\d{2}-\d{2}$/.test(value) ? value : ''
}

function safeFilter(value: string | null) {
  return value && /^[a-z0-9_-]{1,50}$/.test(value) ? value : ''
}

function validUUID(value: string | null) {
  return value && /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value) ? value : ''
}

function safeNode(value: string | null) {
  return value && /^(category:[a-z0-9_-]{1,50}|(?:event|source):[0-9a-f-]{36})$/i.test(value) ? value : ''
}

function rangeLabel(data: PublicKnowledgeGraph, range?: { from: string; to: string }) {
  return range ? `${range.from} – ${range.to}` : `Latest ${data.days === 1 ? '24 hours' : `${data.days} days`}`
}

function narrativeLabel(score: number) {
  if (score <= -0.35) return 'State-led narration'
  if (score >= 0.35) return 'Market-led narration'
  return 'Neutral narration'
}
