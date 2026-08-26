import { createClient, type EventNarrativeAnalysis } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router'

import { ArticleCard } from './article-card'

const client = createClient()

export function EventPage() {
  const { id } = useParams()
  const event = useQuery({
    queryKey: ['event', id],
    queryFn: () => client.getEvent(id ?? ''),
    enabled: Boolean(id),
  })
  if (!event.data) {
    return <p>පූරණය වේ…</p>
  }
  return (
    <section className="grid gap-8 xl:grid-cols-[minmax(0,1fr)_22rem] xl:items-start">
      <div>
        <p className="text-[0.8125rem] text-muted-foreground">සිදුවීම</p>
        <h1 className="font-headline text-[length:var(--text-h1)] font-bold">{event.data.title}</h1>
        {event.data.analysis?.summary ? <div className="mt-6 border-y border-rule py-5"><p className="text-xs font-bold uppercase tracking-[0.14em] text-muted-foreground">Cross-source summary</p><p className="mt-3 max-w-4xl whitespace-pre-line text-lg leading-8">{event.data.analysis.summary}</p></div> : null}
      {event.data.is_breaking ? (
        <p className="mt-4 border border-rule bg-tint px-3 py-2 text-sm">විශේෂ පුවත් — මූලාශ්‍ර කිහිපයක් වාර්තා කර ඇත.</p>
      ) : null}
        <h2 className="font-headline mt-8 text-2xl font-bold">Source reporting</h2>
        <ol className="mt-4">
        {event.data.articles.map((item) => (
          <li key={item.id}>
            <ArticleCard item={item} />
          </li>
        ))}
        </ol>
      </div>
      {event.data.analysis ? <CoverageSpectrum analysis={event.data.analysis} /> : <aside className="border border-rule bg-tint p-5"><h2 className="font-headline text-xl font-bold">Coverage spectrum</h2><p className="mt-2 text-sm text-muted-foreground">Analysis is still processing for this event.</p></aside>}
    </section>
  )
}

function CoverageSpectrum({ analysis }: { analysis: EventNarrativeAnalysis }) {
  const groups = {
    left: analysis.source_spectrum.filter((item) => item.label === 'left'),
    center: analysis.source_spectrum.filter((item) => item.label === 'center'),
    right: analysis.source_spectrum.filter((item) => item.label === 'right'),
    unrated: analysis.source_spectrum.filter((item) => item.label === 'unrated'),
  }
  return (
    <aside className="border border-rule bg-tint p-5 xl:sticky xl:top-24">
      <div className="flex items-start justify-between gap-4"><div><p className="text-xs font-bold uppercase tracking-[0.14em] text-muted-foreground">Coverage details</p><h2 className="font-headline mt-1 text-2xl font-bold">Bias distribution</h2></div><span className="font-headline text-3xl font-bold">{analysis.source_count}</span></div>
      <p className="mt-2 text-sm text-muted-foreground">{analysis.rated_source_count} rated · {analysis.source_count - analysis.rated_source_count} untracked</p>
      {analysis.rated_source_count > 0 ? <div className="mt-5 flex h-10 overflow-hidden border border-rule text-xs font-bold" aria-label={`Left ${analysis.left_percentage}%, center ${analysis.center_percentage}%, right ${analysis.right_percentage}%`}>
        {analysis.left_percentage > 0 ? <span className="grid place-items-center bg-[#b96d70] text-white" style={{ width: `${analysis.left_percentage}%` }}>L {Math.round(analysis.left_percentage)}%</span> : null}
        {analysis.center_percentage > 0 ? <span className="grid place-items-center bg-white text-black" style={{ width: `${analysis.center_percentage}%` }}>C {Math.round(analysis.center_percentage)}%</span> : null}
        {analysis.right_percentage > 0 ? <span className="grid place-items-center bg-[#587ead] text-white" style={{ width: `${analysis.right_percentage}%` }}>R {Math.round(analysis.right_percentage)}%</span> : null}
      </div> : <div className="mt-5 border border-rule bg-background/60 px-3 py-3 text-center text-xs font-bold text-muted-foreground">No politically rateable sources</div>}
      <div className="mt-6 grid grid-cols-3 gap-2">
        {(['left', 'center', 'right'] as const).map((label) => <SourceColumn key={label} label={label} items={groups[label]} />)}
      </div>
      {groups.unrated.length ? <div className="mt-6 border-t border-rule pt-4"><p className="text-xs font-bold uppercase tracking-wide text-muted-foreground">Untracked stance</p><div className="mt-3 flex flex-wrap gap-2">{groups.unrated.map((item) => <SourceMark key={item.article_id} item={item} />)}</div></div> : null}
      <p className="mt-6 text-xs leading-5 text-muted-foreground">Scores describe each article’s reporting frame. They are not permanent labels for a publisher.</p>
    </aside>
  )
}

function SourceColumn({ label, items }: { label: 'left' | 'center' | 'right'; items: EventNarrativeAnalysis['source_spectrum'] }) {
  return <div className="min-w-0 rounded-full bg-background/70 px-2 py-3 text-center"><p className="text-[0.65rem] font-bold uppercase tracking-wide text-muted-foreground">{label}</p><div className="mt-3 flex flex-col items-center gap-2">{items.map((item) => <SourceMark key={item.article_id} item={item} />)}</div></div>
}

function SourceMark({ item }: { item: EventNarrativeAnalysis['source_spectrum'][number] }) {
  return item.source_icon ? <img src={item.source_icon} alt={item.source} title={item.source} className="size-10 rounded-full border border-rule bg-white object-contain p-1" loading="lazy" /> : <span title={item.source} className="grid size-10 place-items-center rounded-full border border-rule bg-paper text-xs font-bold">{item.source.slice(0, 2).toUpperCase()}</span>
}
