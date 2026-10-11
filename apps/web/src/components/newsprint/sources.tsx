import type { PublicArticle, StorySource } from '@snap/api-client'
import { Fragment } from 'react'

import { cn } from '@snap/ui/utils'
import { absoluteTitle, relativeTime, sinhalaDateTime, sourceTypeLabel } from '@/lib/format'
import { OutletName } from './outlet'

export type SourceRef = {
  key: string
  name: string
  type: string
  url: string
  publishedAt?: string
  headline?: string
}

export function fromStorySources(sources: StorySource[]): SourceRef[] {
  return sources.map((source) => ({
    key: source.article_id || source.id,
    name: source.name,
    type: source.type,
    url: source.original_url,
    publishedAt: source.published_at,
    headline: source.headline,
  }))
}

export function fromArticles(articles: PublicArticle[]): SourceRef[] {
  return articles.map((article) => ({
    key: article.id,
    name: article.source.name,
    type: article.source.type,
    url: article.original_url,
    publishedAt: article.published_at,
    headline: article.headline,
  }))
}

/** One report per outlet (the earliest), so a source line never repeats a name. */
export function uniqueOutlets(sources: SourceRef[]) {
  const seen = new Map<string, SourceRef>()
  for (const source of [...sources].sort((a, b) => (a.publishedAt ?? '').localeCompare(b.publishedAt ?? ''))) {
    if (!seen.has(source.name)) seen.set(source.name, source)
  }
  return [...seen.values()]
}

/**
 * The single small attribution line that closes every story:
 * "මූලාශ්‍ර: Hiru News · ITN News" — each name opens the original article in a new tab.
 */
export function SourceLine({
  sources,
  updatedAt,
  linked = true,
  className,
}: {
  sources: SourceRef[]
  updatedAt?: string
  /** Phones print names only (links would be below tap size); the reading page carries them. */
  linked?: boolean
  className?: string
}) {
  const outlets = uniqueOutlets(sources)
  if (!outlets.length) return null
  return (
    <p className={cn('np-meta text-[0.8125rem] leading-[1.7] text-ink-secondary', className)}>
      <span>මූලාශ්‍ර: </span>
      {outlets.map((source, index) => (
        <Fragment key={source.key}>
          {index > 0 ? <span aria-hidden="true"> · </span> : null}
          {!linked ? (
            <OutletName name={source.name} className="font-normal text-ink-secondary" />
          ) : (
            <a href={source.url} target="_blank" rel="noopener noreferrer" className="np-link">
              <OutletName name={source.name} className="font-normal text-ink-secondary" />
              <span className="sr-only"> — මුල් ලිපිය (නව පටිත්තක)</span>
            </a>
          )}
        </Fragment>
      ))}
      {updatedAt ? (
        <>
          <span aria-hidden="true"> — </span>
          <time dateTime={updatedAt} title={absoluteTitle(updatedAt)}>
            {relativeTime(updatedAt)}
          </time>
        </>
      ) : null}
    </p>
  )
}

/** Small closing note on the reading page: each outlet's original article, with time. */
export function SourceNotes({ sources, className }: { sources: SourceRef[]; className?: string }) {
  if (!sources.length) return null
  const ordered = [...sources].sort((a, b) => (a.publishedAt ?? '').localeCompare(b.publishedAt ?? ''))
  return (
    <section aria-labelledby="source-notes" className={cn('border-t border-ink pt-2', className)}>
      <h2 id="source-notes" className="np-kicker text-[0.875rem] text-ink">
        මූලාශ්‍ර
      </h2>
      <ol className="np-meta mt-1 text-[0.875rem] leading-[1.6]">
        {ordered.map((source) => (
          <li key={source.key} className="border-b border-rule py-1.5 last:border-b-0">
            <a href={source.url} target="_blank" rel="noopener noreferrer" className="np-link inline-flex min-h-11 flex-wrap items-center gap-x-2 lg:min-h-0">
              <OutletName name={source.name} className="font-normal text-ink" />
              {source.headline ? <span className="text-ink-secondary">“{source.headline}”</span> : null}
              <span className="sr-only"> — මුල් ලිපිය (නව පටිත්තක)</span>
            </a>
            <span className="block text-[0.8125rem]">
              {sourceTypeLabel(source.type)}
              {source.publishedAt ? (
                <>
                  {' · '}
                  <time dateTime={source.publishedAt}>{sinhalaDateTime(source.publishedAt)}</time>
                </>
              ) : null}
            </span>
          </li>
        ))}
      </ol>
    </section>
  )
}
