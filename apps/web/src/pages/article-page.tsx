import { createClient, type PublicArticle } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Link, useParams } from 'react-router'

import { HalftoneFigure } from '@/components/newsprint/figure'
import { ExternalMark, OutletName } from '@/components/newsprint/outlet'
import { ReportProblem } from '@/components/newsprint/report-problem'
import { SpectrumBar } from '@/components/newsprint/spectrum'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { absoluteTitle, paragraphs, sinhalaDateTime, sourceTypeLabel } from '@/lib/format'

const client = createClient()

function setMeta(property: string, content: string) {
  let element = document.querySelector(`meta[property="${property}"]`)
  if (!element) {
    element = document.createElement('meta')
    element.setAttribute('property', property)
    document.head.appendChild(element)
  }
  element.setAttribute('content', content)
}

/** The prominent box that sends readers to the publisher. */
export function OriginalBox({ item }: { item: PublicArticle }) {
  return (
    <div className="border-[3px] border-double border-ink p-4">
      <p className="np-kicker text-ink">මුල් ලිපිය</p>
      <p className="np-body mt-1 text-[1rem] text-ink-secondary">
        මෙම වාර්තාව <OutletName name={item.source.name} /> ආයතනයට අයත් ය. සම්පූර්ණ ලිපිය ඔවුන්ගේ වෙබ් අඩවියේ කියවන්න.
      </p>
      <a href={item.original_url} target="_blank" rel="noopener noreferrer" className="np-btn mt-3 w-full">
        මුල් ලිපිය — {item.source.name}
        <ExternalMark />
      </a>
    </div>
  )
}

export function ArticlePage() {
  const { id = '' } = useParams()
  const article = useQuery({ queryKey: ['article', id], queryFn: () => client.getNews(id), enabled: Boolean(id) })

  useEffect(() => {
    if (!article.data) return
    document.title = `${article.data.headline} | තුලාව`
    setMeta('og:title', `${article.data.headline} — තුලාව`)
    setMeta('og:description', `${article.data.source.name} වාර්තාව. සම්පූර්ණ ලිපිය මුල් ප්‍රකාශකයා සතුය.`)
    setMeta('og:type', 'article')
  }, [article.data])

  if (article.isPending) return <GalleyPlaceholder lines={10} className="max-w-3xl" />
  if (article.isError || !article.data) {
    return <Notice title="මෙම වාර්තාව සොයාගත නොහැක" retry={() => article.refetch()} />
  }

  const item = article.data
  const summary = paragraphs(item.analysis?.summary ?? item.excerpt)

  return (
    <article aria-labelledby="article-title" className="grid grid-cols-1 gap-8 lg:grid-cols-12">
      <div className="lg:col-span-8 lg:border-e lg:border-rule lg:pe-8">
        <p className="np-kicker flex flex-wrap gap-x-3">
          {item.category ? (
            <Link to={`/c/${item.category.slug}`} className="np-headline-link">
              {item.category.name_si}
            </Link>
          ) : null}
          <span className="text-ink-tertiary">මාධ්‍ය වාර්තාව</span>
        </p>
        <h1 id="article-title" className="np-headline mt-1 text-[2rem] leading-[1.35] md:text-[2.75rem] md:leading-[1.28]">
          {item.headline}
        </h1>
        <div className="mt-3 border-y border-ink py-2">
          <p className="font-headline text-[1.0625rem] leading-[1.6]">
            වාර්තා කළේ: <OutletName name={item.source.name} className="text-[1.0625rem]" />
            <span className="np-meta"> · {sourceTypeLabel(item.source.type)}</span>
          </p>
          <p className="np-meta">
            පළ කළේ{' '}
            <time dateTime={item.published_at} title={absoluteTitle(item.published_at)}>
              {sinhalaDateTime(item.published_at)}
            </time>
            {' · '}ශීර්ෂපාඨය මුල් ප්‍රකාශකයාගේ ය
          </p>
        </div>

        <div className="mt-5 lg:hidden">
          <OriginalBox item={item} />
        </div>

        <HalftoneFigure src={item.media} alt="" credit={item.source.name} className="mt-6" priority />

        {item.editorial_note ? (
          <p className="mt-5 border-s-2 border-ink ps-3 font-headline text-[1rem] leading-[1.7] font-medium text-ink-secondary">
            කතු සටහන: {item.editorial_note}
          </p>
        ) : null}

        {summary.length ? (
          <section aria-labelledby="summary-title" className="mt-6">
            <h2 id="summary-title" className="np-section-head">
              සාරාංශය
            </h2>
            <p className="np-meta text-[0.875rem]">{item.source.name} වාර්තාව ඇසුරින් ස්වයංක්‍රීයව සකස් කළකි. සම්පූර්ණ විස්තර සඳහා මුල් ලිපිය කියවන්න.</p>
            <div className="np-body mt-3 text-[1.125rem]">
              {summary.map((text, index) => (
                <p key={index} className={index > 0 ? 'mt-4' : undefined}>
                  {text}
                </p>
              ))}
            </div>
          </section>
        ) : (
          <p className="np-body mt-6 text-ink-secondary">මෙම වාර්තාවේ සාරාංශයක් නැත. සම්පූර්ණ ලිපිය මුල් ප්‍රකාශකයාගේ වෙබ් අඩවියේ ඇත.</p>
        )}

        <div className="mt-8 flex flex-wrap gap-3">
          {item.event_id ? (
            <Link to={`/e/${item.event_id}`} className="np-btn np-btn-ghost">
              මෙම පුවත වාර්තා කළ අනෙක් මාධ්‍ය →
            </Link>
          ) : null}
          <ReportProblem entityType="article" entityId={item.id} />
        </div>
      </div>

      <aside className="flex flex-col gap-6 lg:col-span-4" aria-label="මූලාශ්‍රය සහ විශ්ලේෂණය">
        <div className="hidden lg:block">
          <OriginalBox item={item} />
        </div>
        {item.analysis ? (
          <section aria-labelledby="frame-title" className="border-t-2 border-ink pt-2">
            <h2 id="frame-title" className="np-kicker text-ink">
              වාර්තාකරණ රාමුව
            </h2>
            {item.analysis.relevant ? (
              <SpectrumBar
                className="mt-3"
                left={item.analysis.left_probability * 100}
                center={item.analysis.center_probability * 100}
                right={item.analysis.right_probability * 100}
              />
            ) : (
              <p className="np-body mt-2 text-[1rem] text-ink-secondary">මෙම වාර්තාවේ කැපී පෙනෙන දේශපාලන රාමුවක් හමු නොවීය.</p>
            )}
            <p className="np-meta mt-3 text-[0.8125rem]">
              ස්වයංක්‍රීය විශ්ලේෂණයකි. මෙය මෙම ලිපියේ වාර්තාකරණ රාමුව පමණක් විස්තර කරයි; ප්‍රකාශකයාට දෙන ස්ථිර ලේබලයක් නොවේ.
            </p>
          </section>
        ) : null}
        <section className="border-t-2 border-ink pt-2">
          <h2 className="np-kicker text-ink">මූලාශ්‍රය ගැන</h2>
          <p className="mt-1">
            <Link to={`/sources/${item.source.id}`} className="np-link font-headline inline-flex min-h-11 items-center font-bold">
              {item.source.name} වෙතින් තවත් වාර්තා →
            </Link>
          </p>
        </section>
      </aside>
    </article>
  )
}
