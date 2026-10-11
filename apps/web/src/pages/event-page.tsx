import { createClient, type EventNarrativeAnalysis } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Link, useParams } from 'react-router'

import { HalftoneFigure } from '@/components/newsprint/figure'
import { OutletName } from '@/components/newsprint/outlet'
import { ReportProblem } from '@/components/newsprint/report-problem'
import { SourceLedger, fromArticles } from '@/components/newsprint/sources'
import { SpectrumBar, Swatch, leanLabels } from '@/components/newsprint/spectrum'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { absoluteTitle, outletsReported, paragraphs, relativeTime } from '@/lib/format'

const client = createClient()

export function EventPage() {
  const { id = '' } = useParams()
  const event = useQuery({ queryKey: ['event', id], queryFn: () => client.getEvent(id), enabled: Boolean(id) })

  useEffect(() => {
    if (event.data) document.title = `${event.data.title} | තුලාව`
  }, [event.data])

  if (event.isPending) return <GalleyPlaceholder lines={10} className="max-w-3xl" />
  if (event.isError || !event.data) return <Notice title="මෙම පුවත සොයාගත නොහැක" retry={() => event.refetch()} />

  const data = event.data
  const sources = fromArticles(data.articles)
  const outletCount = new Set(data.articles.map((article) => article.source.id)).size
  const latest = data.articles.reduce<string>((max, article) => (article.published_at > max ? article.published_at : max), '')
  const category = data.articles.find((article) => article.category)?.category
  const media = data.articles.find((article) => article.media)
  const summary = paragraphs(data.analysis?.summary)

  return (
    <article aria-labelledby="event-title" className="grid grid-cols-1 gap-8 lg:grid-cols-12">
      <div className="lg:col-span-8 lg:border-e lg:border-rule lg:pe-8">
        <p className="np-kicker flex flex-wrap items-center gap-x-3">
          {data.is_breaking ? <span className="bg-ink px-1.5 text-paper">විශේෂ පුවත</span> : null}
          {category ? (
            <Link to={`/c/${category.slug}`} className="np-headline-link">
              {category.name_si}
            </Link>
          ) : null}
          <span className="text-ink-tertiary">මාධ්‍ය කිහිපයක වාර්තා එක තැනක</span>
        </p>
        <h1 id="event-title" className="np-headline mt-1 text-[2rem] leading-[1.35] md:text-[2.75rem] md:leading-[1.28]">
          {data.title}
        </h1>
        <p className="np-meta mt-3 border-y border-ink py-2 text-[0.9375rem]">
          <span className="font-bold text-ink">{outletsReported(outletCount)}</span>
          {latest ? (
            <>
              {' · අවසන් වාර්තාව '}
              <time dateTime={latest} title={absoluteTitle(latest)}>
                {relativeTime(latest)}
              </time>
            </>
          ) : null}
        </p>

        <HalftoneFigure src={media?.media} alt="" credit={media?.source.name} className="mt-6" priority />

        {summary.length ? (
          <section aria-labelledby="event-summary" className="mt-6 border border-ink bg-paper-raised p-4 md:p-5">
            <h2 id="event-summary" className="np-kicker text-ink">
              ස්වයංක්‍රීය සාරාංශය
            </h2>
            <p className="np-meta text-[0.8125rem]">පහත මාධ්‍ය වාර්තා {data.articles.length} ඇසුරින් ස්වයංක්‍රීයව සකස් කළකි. වාර්තා අතර වෙනස්කම් ඇත්නම් මුල් ලිපි සසඳන්න.</p>
            <div className="np-body mt-3 text-[1.0625rem] md:text-[1.125rem]">
              {summary.map((text, index) => (
                <p key={index} className={index > 0 ? 'mt-4' : undefined}>
                  {text}
                </p>
              ))}
            </div>
          </section>
        ) : null}

        <SourceLedger sources={sources} className="mt-8" />

        <div className="mt-8">
          <ReportProblem entityType="event" entityId={data.id} />
        </div>
      </div>

      <aside className="lg:col-span-4" aria-label="ආවරණ විස්තර">
        <div className="lg:sticky lg:top-16">
          {data.analysis ? (
            <CoverageSpectrum analysis={data.analysis} />
          ) : (
            <section className="border-t-2 border-ink pt-2">
              <h2 className="np-kicker text-ink">වාර්තාකරණ රාමු</h2>
              <p className="np-body mt-2 text-[1rem] text-ink-secondary">මෙම පුවතේ විශ්ලේෂණය තවම සකස් වෙමින් පවතී.</p>
            </section>
          )}
        </div>
      </aside>
    </article>
  )
}

function CoverageSpectrum({ analysis }: { analysis: EventNarrativeAnalysis }) {
  const groups = (['left', 'center', 'right'] as const).map((lean) => ({
    lean,
    items: analysis.source_spectrum.filter((item) => item.label === lean),
  }))
  const unrated = analysis.source_spectrum.filter((item) => item.label === 'unrated')
  return (
    <section aria-labelledby="coverage-title" className="border-t-2 border-ink pt-2">
      <div className="flex items-baseline justify-between gap-3">
        <h2 id="coverage-title" className="np-kicker text-ink">
          වාර්තාකරණ රාමු
        </h2>
        <span className="np-meta">
          මාධ්‍ය <span className="np-latin text-ink">{analysis.source_count}</span>
        </span>
      </div>
      {analysis.rated_source_count > 0 ? (
        <SpectrumBar
          className="mt-3"
          left={analysis.left_percentage}
          center={analysis.center_percentage}
          right={analysis.right_percentage}
        />
      ) : (
        <p className="np-body mt-2 text-[1rem] text-ink-secondary">දේශපාලන රාමුවක් මැනිය හැකි වාර්තා නැත.</p>
      )}
      <div className="mt-4 grid grid-cols-3 border-t border-rule">
        {groups.map(({ lean, items }) => (
          <div key={lean} className="min-w-0 border-e border-rule px-2 py-2 last:border-e-0">
            <p className="np-meta flex items-center gap-1.5 text-[0.875rem]">
              <Swatch lean={lean} />
              {leanLabels[lean]}
            </p>
            <ul className="mt-1 flex flex-col gap-1">
              {items.map((item) => (
                <li key={item.article_id} className="text-[0.875rem] leading-[1.4] break-words">
                  <a href={item.original_url} target="_blank" rel="noopener noreferrer" className="np-link">
                    <OutletName name={item.source} className="font-bold" />
                    <span className="sr-only"> — මුල් ලිපිය (නව පටිත්තක)</span>
                  </a>
                </li>
              ))}
              {!items.length ? <li className="np-meta text-[0.8125rem]">—</li> : null}
            </ul>
          </div>
        ))}
      </div>
      {unrated.length ? (
        <p className="np-meta mt-3 border-t border-rule pt-2 text-[0.875rem]">
          {leanLabels.unrated}: {unrated.map((item) => item.source).join(', ')}
        </p>
      ) : null}
      <p className="np-meta mt-3 text-[0.8125rem]">
        ස්වයංක්‍රීය විශ්ලේෂණයකි. එක් එක් ලිපියේ වාර්තාකරණ රාමුව පමණක් විස්තර කරයි; ප්‍රකාශකයන්ට දෙන ස්ථිර ලේබල නොවේ.
      </p>
    </section>
  )
}
