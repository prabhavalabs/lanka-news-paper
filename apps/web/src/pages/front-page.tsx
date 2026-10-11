import { createClient, type FrontPage as FrontPageData, type FrontPageStory, type PublicArticle } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Link } from 'react-router'

import { cn } from '@snap/ui/utils'
import { EditorialPanel } from '@/components/newsprint/editorial-panel'
import { OrnamentRule } from '@/components/newsprint/ornament'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { BriefStory, LeadStory, TopStory, storyHref } from '@/components/newsprint/story'
import { editionDate, sinhalaLongDate } from '@/lib/format'

const client = createClient()

/**
 * Builds an automatic edition from the latest reports when the curated front page is unavailable,
 * so readers always get a complete paper. Stories group by event; nothing is marked curated.
 */
function automaticEdition(items: PublicArticle[]): FrontPageData {
  const groups = new Map<string, PublicArticle[]>()
  for (const item of items) {
    const key = item.event_id ? `e:${item.event_id}` : `a:${item.id}`
    groups.set(key, [...(groups.get(key) ?? []), item])
  }
  const stories: FrontPageStory[] = [...groups.entries()].map(([key, articles]) => {
    const first = articles[0] as PublicArticle
    const newest = articles.reduce((a, b) => (a.published_at > b.published_at ? a : b))
    return {
      kind: key.startsWith('e:') ? 'event' : 'article',
      id: key.slice(2),
      headline: first.headline,
      summary: first.analysis?.summary ?? first.excerpt,
      category: first.category,
      breaking: false,
      published_at: first.published_at,
      updated_at: newest.published_at,
      source_count: new Set(articles.map((article) => article.source.id)).size,
      sources: articles.map((article) => ({
        id: article.source.id,
        name: article.source.name,
        type: article.source.type,
        article_id: article.id,
        headline: article.headline,
        original_url: article.original_url,
        published_at: article.published_at,
      })),
      curated: false,
      slot: 'more',
      editor_note: null,
      media: first.media,
    }
  })
  stories.sort((a, b) => b.source_count - a.source_count || b.updated_at.localeCompare(a.updated_at))
  const [lead = null, ...rest] = stories
  return {
    edition_date: new Date().toISOString().slice(0, 10),
    generated_at: new Date().toISOString(),
    lead: lead ? { ...lead, slot: 'lead' } : null,
    top: rest.slice(0, 6).map((story) => ({ ...story, slot: 'top' })),
    more: rest.slice(6),
    editorial: null,
    curated: false,
  }
}

function useEdition() {
  const front = useQuery({ queryKey: ['front-page'], queryFn: () => client.frontPage(), staleTime: 30_000 })
  const fallback = useQuery({
    queryKey: ['front-page-fallback'],
    queryFn: async () => automaticEdition((await client.listNews({ limit: '40' })).items),
    enabled: front.isError,
  })
  if (front.data) return { edition: front.data, pending: false, error: false, retry: () => front.refetch() }
  return {
    edition: fallback.data,
    pending: front.isPending || fallback.isPending,
    error: front.isError && fallback.isError,
    retry: () => {
      void front.refetch()
      void fallback.refetch()
    },
  }
}

// Column rules between top stories: 2-up on tablets, 3- or 4-up on desktop (4 when it fills rows evenly).
function topColumnClass(index: number, lgCols: 3 | 4) {
  return cn(
    'border-t border-rule pt-5 sm:pt-6',
    index < 2 && 'sm:border-t-0 sm:pt-0',
    index % 2 === 1 && 'sm:border-s sm:ps-6',
    'lg:border-t-0 lg:pt-0 lg:border-s-0 lg:ps-0',
    index >= lgCols && 'lg:border-t lg:pt-6',
    index % lgCols !== 0 && 'lg:border-s lg:ps-6',
  )
}

function BreakingStrip({ stories }: { stories: FrontPageStory[] }) {
  if (!stories.length) return null
  return (
    <section aria-label="විශේෂ පුවත්" className="mb-5 bg-ink px-3 py-2 text-paper">
      <ul className="flex flex-col gap-1">
        {stories.map((story) => (
          <li key={story.id} className="flex flex-wrap items-baseline gap-x-3 font-headline text-[1.0625rem] leading-[1.55]">
            <span className="border border-paper px-1.5 text-[0.9375rem] font-extrabold">විශේෂ පුවත</span>
            <Link to={storyHref(story)} className="font-bold underline underline-offset-4">
              {story.headline}
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}

export function FrontPage() {
  const { edition, pending, error, retry } = useEdition()

  useEffect(() => {
    document.title = 'තුලාව | Thulawa News — අසමතුලිත ලෝකයකට සමතුලිත පුවත්'
  }, [])

  if (pending) {
    return (
      <div className="grid gap-8 lg:grid-cols-12">
        <GalleyPlaceholder lines={10} className="lg:col-span-8" />
        <GalleyPlaceholder lines={6} className="hidden lg:col-span-4 lg:flex" />
      </div>
    )
  }
  if (error || !edition) {
    return (
      <Notice title="අද කලාපය පූරණය කළ නොහැකි විය" retry={retry}>
        සම්බන්ධතාවය පරීක්ෂා කර නැවත උත්සාහ කරන්න.
      </Notice>
    )
  }

  const { lead } = edition
  const all = [lead, ...edition.top, ...edition.more].filter((story): story is FrontPageStory => Boolean(story))
  const breaking = all.filter((story) => story.breaking)
  // Balance the opening spread: one top story runs under the lead, one beside it in the rail.
  const [underLead, ...afterUnder] = edition.top
  const rail = afterUnder.slice(0, 1)
  const row = afterUnder.slice(1)
  const rowCols: 3 | 4 = row.length % 4 === 0 ? 4 : 3
  const curatedCount = all.filter((story) => story.curated).length

  if (!lead && !all.length) {
    return (
      <>
        <h1 className="sr-only">අද මුල් පිටුව</h1>
        <div className="grid gap-8 lg:grid-cols-12">
          <Notice title="අද කලාපයට තවම පුවත් නැත" className="lg:col-span-8">
            නව වාර්තා ලැබෙත්ම මෙම පිටුව පිරේ.
          </Notice>
          <EditorialPanel editorial={edition.editorial} className="lg:col-span-4" />
        </div>
      </>
    )
  }

  return (
    <>
      <h1 className="sr-only">අද මුල් පිටුව — {sinhalaLongDate(editionDate(edition.edition_date))}</h1>
      <BreakingStrip stories={breaking} />

      <div className="grid grid-cols-1 gap-y-8 lg:grid-cols-12">
        <div className="lg:col-span-8 lg:border-e lg:border-rule lg:pe-8">
          {lead ? <LeadStory story={lead} /> : null}
          {underLead ? (
            <div className={cn(lead && 'mt-6 border-t-2 border-ink pt-4')}>
              <TopStory story={underLead} />
            </div>
          ) : null}
        </div>
        <div className="flex flex-col gap-8 lg:col-span-4 lg:ps-8">
          <EditorialPanel editorial={edition.editorial} />
          {rail.length ? (
            <section aria-labelledby="rail-title">
              <h2 id="rail-title" className="np-section-head">
                ප්‍රධාන පුවත්
              </h2>
              <div className="pt-3">
                {rail.map((story) => (
                  <TopStory key={story.id} story={story} />
                ))}
              </div>
            </section>
          ) : null}
        </div>
      </div>

      {row.length ? (
        <section aria-labelledby="top-title" className="mt-10">
          <div className="np-rule-double" />
          <h2 id="top-title" className="np-section-head mt-2">
            තවත් ප්‍රධාන පුවත්
          </h2>
          <ul className={cn('mt-4 grid grid-cols-1 gap-x-6 gap-y-6 sm:grid-cols-2', rowCols === 4 ? 'lg:grid-cols-4' : 'lg:grid-cols-3')}>
            {row.map((story, index) => (
              <li key={story.id} className={topColumnClass(index, rowCols)}>
                <TopStory story={story} />
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {edition.more.length ? (
        <section aria-labelledby="more-title" className="mt-10">
          <div className="np-rule-double" />
          <h2 id="more-title" className="np-section-head mt-2">
            තවත් පුවත්
          </h2>
          <ul className="np-columns mt-4 md:columns-2 lg:columns-3">
            {edition.more.map((story) => (
              <li key={story.id} className="mb-4 break-inside-avoid border-b border-rule pb-4">
                <BriefStory story={story} />
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      <OrnamentRule className="mt-8" kind="fleuron" />
      <div className="np-meta mt-3 flex flex-col items-center gap-1 text-center text-[0.875rem]">
        <p>
          {curatedCount > 0
            ? `මෙම කලාපයේ පුවත් ${curatedCount}ක් ප්‍රධාන කතුවරයා තෝරා ගත්තේය; ඉතිරි ඉඩ නවතම වාර්තා අනුව පිරිණි.`
            : 'මෙම කලාපය නවතම වාර්තා අනුව සකස් විය.'}
        </p>
        <p className="flex flex-wrap justify-center gap-x-5">
          <Link to="/search" className="np-link font-headline inline-flex min-h-11 items-center font-bold text-ink">
            සියලු නවතම පුවත් →
          </Link>
          <Link to="/about" className="np-link inline-flex min-h-11 items-center">
            පුවත්පත සකසන ආකාරය
          </Link>
        </p>
      </div>
    </>
  )
}
