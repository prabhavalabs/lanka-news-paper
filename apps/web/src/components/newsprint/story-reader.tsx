import { createClient, type PublicCategory } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Link } from 'react-router'

import { absoluteTitle, dropCapSplit, outletsReported, paragraphs as splitParagraphs, sinhalaDateTime } from '@/lib/format'
import { CoverageSidebar, type CoverageSummary, type OutletCoverage } from './coverage-sidebar'
import { HalftoneFigure, Illustration } from './figure'
import { RelatedStories } from './related-stories'

const client = createClient()

/** The desk's full report for a story, or null when it has not been written yet (404). */
export function useStoryReport(kind: 'event' | 'article', id: string) {
  return useQuery({
    queryKey: ['story-report', kind, id],
    queryFn: async () => {
      try {
        return await client.storyReport(kind, id)
      } catch (error) {
        if (error instanceof Error && /: 404$/.test(error.message)) return null
        throw error
      }
    },
    enabled: Boolean(id),
    retry: false,
    staleTime: 5 * 60_000,
  })
}

/**
 * The reading page for one story, at the header's full width. Desktop: the article in the wide
 * left column, every reporting outlet with its lean and original link in a sidebar on the right,
 * related stories across the foot. Phones: article, then sources, then related stories.
 */
export function StoryReader({
  kind,
  id,
  headline,
  summary,
  category,
  breaking = false,
  outlets,
  coverage,
  photo,
  updatedAt,
}: {
  kind: 'event' | 'article'
  id: string
  headline: string
  summary: string | null | undefined
  category: PublicCategory | null | undefined
  breaking?: boolean
  outlets: OutletCoverage[]
  coverage: CoverageSummary | null
  photo?: { url: string | null; credit: string } | undefined
  updatedAt?: string
}) {
  const report = useStoryReport(kind, id)
  const written = report.data?.report
  const title = written?.headline || headline
  const body = written?.paragraphs.length ? written.paragraphs : splitParagraphs(summary)
  const outletCount = outlets.length
  const stamp = written?.written_at ?? updatedAt

  useEffect(() => {
    document.title = `${title} | තුලාව`
  }, [title])

  const [first = '', ...rest] = body
  const [cap, remainder] = dropCapSplit(first)

  return (
    <>
    <div className="grid grid-cols-1 gap-y-8 lg:grid-cols-12">
    <article aria-labelledby="story-title" className="min-w-0 lg:col-span-8 lg:border-e lg:border-rule lg:pe-8">
      <p className="np-kicker flex flex-wrap items-center gap-x-3 text-[0.875rem]">
        {breaking ? <span className="bg-ink px-1.5 text-paper">විශේෂ පුවත</span> : null}
        {category ? (
          <Link to={`/c/${category.slug}`} className="np-headline-link max-lg:inline-flex max-lg:min-h-11 max-lg:items-center">
            {category.name_si}
          </Link>
        ) : null}
      </p>
      <h1 id="story-title" className="np-headline mt-1 text-[2.125rem] leading-[1.3] md:text-[2.875rem] md:leading-[1.24]">
        {title}
      </h1>
      {written?.standfirst ? (
        <p className="np-standfirst mt-3 max-w-[42rem] md:text-[1.1875rem]">{written.standfirst}</p>
      ) : null}
      <p className="np-meta mt-3 border-y border-ink py-1.5 text-[0.875rem]">
        {outletsReported(outletCount)}
        {stamp ? (
          <>
            {' · '}
            <time dateTime={stamp} title={absoluteTitle(stamp)}>
              {sinhalaDateTime(stamp)}
            </time>
          </>
        ) : null}
      </p>

      {report.data?.image ? (
        <Illustration image={report.data.image} className="mt-5" priority large />
      ) : photo ? (
        <HalftoneFigure src={photo.url} alt="" credit={photo.credit} className="mt-5" priority />
      ) : null}

      {body.length ? (
        <div className="np-copy mt-5 max-w-[42rem] text-[1.0625rem] leading-[1.9]">
          <p>
            <span className="np-dropcap">{cap}</span>
            {remainder}
          </p>
          {rest.map((text, index) => (
            <p key={index}>{text}</p>
          ))}
        </div>
      ) : (
        <p className="np-copy mt-5 text-ink-secondary">මෙම පුවතේ විස්තර තවම සකස් වෙමින් පවතී. මූලාශ්‍රවල මුල් ලිපි කියවන්න.</p>
      )}
      {report.isPending ? <p className="np-meta mt-2 text-[0.8125rem]">සම්පූර්ණ වාර්තාව පූරණය වෙමින්…</p> : null}

      <p className="np-meta mt-4 text-[0.8125rem]">
        {written
          ? 'මෙම වාර්තාව මූලාශ්‍ර ලැයිස්තුවේ ඇති මාධ්‍ය ආයතනවල වාර්තා ඇසුරින් ස්වයංක්‍රීයව සකස් කළකි. තොරතුරු ඔවුන්ට අයත් ය.'
          : 'මූලාශ්‍ර ලැයිස්තුවේ ඇති මාධ්‍ය වාර්තා ඇසුරින් ස්වයංක්‍රීයව සකස් කළ සාරාංශයකි. තොරතුරු ඔවුන්ට අයත් ය.'}
      </p>

    </article>
    <div className="min-w-0 lg:col-span-4 lg:ps-8">
      <CoverageSidebar
        outlets={outlets}
        summary={coverage}
        entity={{ type: kind, id }}
        className={outlets.length <= 6 ? 'lg:sticky lg:top-16' : undefined}
      />
    </div>
    </div>
    <RelatedStories kind={kind} id={id} className="mt-10" />
    </>
  )
}
