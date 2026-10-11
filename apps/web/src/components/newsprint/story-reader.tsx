import { createClient, type PublicCategory } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Link } from 'react-router'

import { absoluteTitle, dropCapSplit, outletsReported, paragraphs as splitParagraphs, sinhalaDateTime } from '@/lib/format'
import { HalftoneFigure, Illustration } from './figure'
import { OrnamentRule } from './ornament'
import { ReportProblem } from './report-problem'
import { SourceNotes, uniqueOutlets, type SourceRef } from './sources'

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
 * The reading page for one story: headline, standfirst, illustration and the full report
 * at a comfortable measure, with the original articles listed small at the very end.
 */
export function StoryReader({
  kind,
  id,
  headline,
  summary,
  category,
  breaking = false,
  sources,
  photo,
  updatedAt,
  analysis,
}: {
  kind: 'event' | 'article'
  id: string
  headline: string
  summary: string | null | undefined
  category: PublicCategory | null | undefined
  breaking?: boolean
  sources: SourceRef[]
  photo?: { url: string | null; credit: string } | undefined
  updatedAt?: string
  analysis?: React.ReactNode
}) {
  const report = useStoryReport(kind, id)
  const written = report.data?.report
  const title = written?.headline || headline
  const body = written?.paragraphs.length ? written.paragraphs : splitParagraphs(summary)
  const outletCount = uniqueOutlets(sources).length
  const stamp = written?.written_at ?? updatedAt

  useEffect(() => {
    document.title = `${title} | තුලාව`
  }, [title])

  const [first = '', ...rest] = body
  const [cap, remainder] = dropCapSplit(first)

  return (
    <article aria-labelledby="story-title" className="mx-auto max-w-[40rem]">
      <p className="np-kicker flex flex-wrap items-center justify-center gap-x-3 text-center text-[0.875rem]">
        {breaking ? <span className="bg-ink px-1.5 text-paper">විශේෂ පුවත</span> : null}
        {category ? (
          <Link to={`/c/${category.slug}`} className="np-headline-link max-lg:inline-flex max-lg:min-h-11 max-lg:items-center">
            {category.name_si}
          </Link>
        ) : null}
      </p>
      <h1 id="story-title" className="np-headline mt-1 text-center text-[2.125rem] leading-[1.3] md:text-[2.875rem] md:leading-[1.24]">
        {title}
      </h1>
      {written?.standfirst ? (
        <p className="mt-3 text-center font-headline text-[1.25rem] leading-[1.6] text-ink-secondary md:text-[1.375rem]">{written.standfirst}</p>
      ) : null}
      <p className="np-meta mt-3 border-y border-ink py-1.5 text-center text-[0.875rem]">
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
        <div className="np-copy mt-5 text-[1.125rem] leading-[1.9]">
          <p>
            <span className="np-dropcap">{cap}</span>
            {remainder}
          </p>
          {rest.map((text, index) => (
            <p key={index}>{text}</p>
          ))}
        </div>
      ) : (
        <p className="np-copy mt-5 text-ink-secondary">මෙම පුවතේ විස්තර තවම සකස් වෙමින් පවතී. පහත මුල් ලිපි කියවන්න.</p>
      )}
      {report.isPending ? <p className="np-meta mt-2 text-[0.8125rem]">සම්පූර්ණ වාර්තාව පූරණය වෙමින්…</p> : null}

      <p className="np-meta mt-4 text-[0.8125rem]">
        {written
          ? 'මෙම වාර්තාව පහත මාධ්‍ය ආයතනවල වාර්තා ඇසුරින් ස්වයංක්‍රීයව සකස් කළකි. තොරතුරු ඔවුන්ට අයත් ය.'
          : 'පහත මාධ්‍ය වාර්තා ඇසුරින් ස්වයංක්‍රීයව සකස් කළ සාරාංශයකි. තොරතුරු ඔවුන්ට අයත් ය.'}
      </p>

      <OrnamentRule className="mt-6" />
      <SourceNotes sources={sources} className="mt-4" />
      {analysis ? <div className="mt-6">{analysis}</div> : null}
      <div className="mt-6">
        <ReportProblem entityType={kind} entityId={id} />
      </div>
    </article>
  )
}
