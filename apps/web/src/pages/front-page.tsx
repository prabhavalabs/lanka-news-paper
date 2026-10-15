import {
  createClient,
  type EditionBlock,
  type FrontPage as FrontPageData,
  type FrontPageStory,
  type PublicArticle,
 type PublicStoryline,
} from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router'

import { cn } from '@snap/ui/utils'
import { EditorialPanel } from '@/components/newsprint/editorial-panel'
import { Asterism, Fleuron } from '@/components/newsprint/ornament'
import { GalleyPlaceholder, Notice } from '@/components/newsprint/states'
import { PhoneBlock, PrintBlock, storyHref } from '@/components/newsprint/story'
import { composeEdition, editionBlocks, itemKey, type Lane, type MeasuredAreas } from '@/lib/edition'
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
        icon: '',
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
    blocks: [],
    edition_id: null,
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

function BreakingLine({ stories }: { stories: FrontPageStory[] }) {
  if (!stories.length) return null
  return (
    <section aria-label="විශේෂ පුවත්" className="border-b border-ink py-1.5">
      <ul className="flex flex-col">
        {stories.map((story) => (
          <li key={story.id} className="flex flex-wrap items-baseline gap-x-2 font-headline text-[1rem] leading-[1.6]">
            <span className="bg-ink px-1.5 text-[0.8125rem] text-paper">විශේෂ පුවත</span>
            <Link to={storyHref(story)} className="np-headline-link max-lg:inline-flex max-lg:min-h-11 max-lg:items-center">
              {story.headline}
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}

/** Leftover depth at the foot of a lane takes a printer's ornament, never a blank. */
function LaneFiller() {
  return (
    <div className="np-filler flex-1" aria-hidden="true">
      <div className="h-full items-center justify-center gap-2 border-t border-rule text-ink-tertiary">
        <Fleuron flip />
        <Asterism />
        <Fleuron />
      </div>
    </div>
  )
}

function LaneColumn({ lane, index, editorial }: { lane: Lane; index: number; editorial: FrontPageData['editorial'] }) {
  return (
    <div
      className={cn('flex min-w-0 flex-col', index > 0 && 'border-s border-ink ps-4', 'pe-4 last:pe-0')}
      style={{ gridColumn: `span ${lane.span} / span ${lane.span}` }}
    >
      {lane.items.map((item, itemIndex) => (
        <div key={itemKey(item)} data-measure={itemKey(item)} className={cn(itemIndex > 0 && 'border-t border-rule')}>
          {item.type === 'editorial' ? (
            <EditorialPanel editorial={editorial} className="my-3" />
          ) : (
            <PrintBlock block={item.block} span={lane.span} />
          )}
        </div>
      ))}
      <LaneFiller />
    </div>
  )
}

/** Desktop broadsheet: spreads of interlocking lanes, ruled, every story complete in its box. */
function Broadsheet({ blocks, editorial }: { blocks: EditionBlock[]; editorial: FrontPageData['editorial'] }) {
  const [areas, setAreas] = useState<{ blocks: EditionBlock[]; areas: MeasuredAreas } | null>(null)
  const measuredAreas = areas?.blocks === blocks ? areas.areas : undefined
  const sections = useMemo(() => composeEdition(blocks, true, measuredAreas), [blocks, measuredAreas])
  const root = useRef<HTMLDivElement>(null)

  // First pass lays out from estimates; once type has loaded we measure every story's printed
  // area and pour the page again so the lanes end level.
  useEffect(() => {
    const node = root.current
    if (!node || measuredAreas) return
    let cancelled = false
    const measure = () => {
      if (cancelled || node.offsetWidth === 0) return
      const next: MeasuredAreas = new Map()
      node.querySelectorAll<HTMLElement>('[data-measure]').forEach((element) => {
        next.set(element.dataset.measure ?? '', element.offsetHeight * element.clientWidth)
      })
      if (next.size) setAreas({ blocks, areas: next })
    }
    const observer = new ResizeObserver(() => void document.fonts.ready.then(measure))
    observer.observe(node)
    return () => {
      cancelled = true
      observer.disconnect()
    }
  }, [blocks, measuredAreas])

  return (
    <div ref={root} className="hidden lg:block">
      {sections.map((section, index) => (
        <section
          key={index}
          aria-label={index === 0 ? 'ප්‍රධාන පුවත' : undefined}
          className={cn('grid grid-cols-12', index > 0 && 'border-t-2 border-ink')}
        >
          {section.lanes.map((lane, laneIndex) => (
            <LaneColumn key={laneIndex} lane={lane} index={laneIndex} editorial={editorial} />
          ))}
        </section>
      ))}
    </div>
  )
}

/** Phones and tablets: one column in page order, the editorial right after the lead. */
function Galley({ blocks, editorial }: { blocks: EditionBlock[]; editorial: FrontPageData['editorial'] }) {
  return (
    <div className="lg:hidden">
      {blocks.map((block, index) => (
        <div key={block.story.id}>
          <div className={cn(index > 0 && 'border-t border-ink')}>
            <PhoneBlock block={block} />
          </div>
          {index === 0 ? <EditorialPanel editorial={editorial} className="mb-4" /> : null}
        </div>
      ))}
    </div>
  )
}

export function FrontPage() {
 const trending=useQuery({queryKey:['trending'],queryFn:()=>client.trending(),staleTime:60_000})
  const { edition, pending, error, retry } = useEdition()

  useEffect(() => {
    document.title = 'තුලාව | Thulawa News — අසමතුලිත ලෝකයකට සමතුලිත පුවත්'
  }, [])

  const blocks = useMemo(() => (edition ? editionBlocks(edition) : []), [edition])

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

  const breaking = blocks.map((block) => block.story).filter((story) => story.breaking).slice(0, 2)
  const curatedCount = blocks.filter((block) => block.story.curated).length

  return (
    <>
      <h1 className="sr-only">අද මුල් පිටුව — {sinhalaLongDate(editionDate(edition.edition_date))}</h1>
      <BreakingLine stories={breaking} />
 <TrendingStories items={trending.data?.items ?? []} />
      {blocks.length ? (
        <>
          <Broadsheet blocks={blocks} editorial={edition.editorial} />
          <Galley blocks={blocks} editorial={edition.editorial} />
        </>
      ) : (
        <div className="grid gap-6 py-6 lg:grid-cols-12">
          <Notice title="අද කලාපයට තවම පුවත් නැත" className="lg:col-span-8">
            නව වාර්තා ලැබෙත්ම මෙම පිටුව පිරේ.
          </Notice>
          <EditorialPanel editorial={edition.editorial} className="lg:col-span-4" />
        </div>
      )}
      <div className="np-rule-double mt-2" />
      <p className="np-meta mt-2 text-center text-[0.8125rem]">
        මෙහි පුවත් මාධ්‍ය ආයතනවල වාර්තා ඇසුරින් සකස් කළ ඒවා ය; එක් එක් පුවත අවසානයේ මුල් ලිපි සඳහන් වේ.
        {curatedCount > 0 ? ` පුවත් ${curatedCount}ක් ප්‍රධාන කතුවරයා තෝරා ගත්තේය.` : ''}
      </p>
    </>
  )
}

function TrendingStories({ items }: { items: PublicStoryline[] }) {
  const top = items[0]
  if (!top) return null
  return <section aria-labelledby="trending-heading" className="border-b border-ink py-4">
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
      <h2 id="trending-heading" className="font-headline text-sm font-bold">දැන් කතාබහට ලක්වන</h2>
      <ul className="flex min-w-0 flex-wrap gap-2">{items.map((item) => <li key={item.id} className="min-w-0 max-w-full"><Link to={`/s/${item.slug}`} className="inline-flex min-h-11 max-w-full items-center rounded-full border border-rule px-3 py-1 font-headline text-sm hover:border-ink focus-visible:outline-2"><span className="truncate">{item.title_si}</span></Link></li>)}</ul>
    </div>
    <div className="mt-4 grid gap-3 border-t border-rule pt-4 md:grid-cols-[2fr_1fr] md:gap-6">
      <div className="min-w-0"><p className="mb-2 font-headline text-xs text-ink-tertiary">අඛණ්ඩ පුවත් පෙළ</p><h3 className="font-headline text-xl font-bold leading-relaxed sm:text-2xl"><Link className="np-headline-link" to={`/s/${top.slug}`}>{top.title_si}</Link></h3>{top.summary_si && <p className="mt-2 font-body text-base leading-relaxed">{top.summary_si}</p>}</div>
      <div className="min-w-0 border-t border-rule pt-3 md:border-s md:border-t-0 md:ps-5 md:pt-0">{top.latest_event && <><p className="font-headline text-xs text-ink-tertiary">නවතම වර්ධනය</p><Link to={`/e/${top.latest_event.id}`} className="np-headline-link mt-2 block font-headline text-base font-bold leading-relaxed">{top.latest_event.display_title}</Link></>}<p className="mt-3 text-sm text-ink-secondary">වාර්තා {top.article_count} · මාධ්‍ය ආයතන {top.source_count}</p><Link to={`/s/${top.slug}`} className="np-headline-link mt-2 inline-flex min-h-11 items-center font-headline text-sm">සම්පූර්ණ පුවත් පෙළ →</Link></div>
    </div>
  </section>
}
