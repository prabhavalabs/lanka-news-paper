import type { PublicArticle, StorySource } from '@snap/api-client'
import { Link } from 'react-router'

import { cn } from '@snap/ui/utils'
import { absoluteTitle, relativeTime, sourceTypeLabel } from '@/lib/format'
import { ExternalMark, OutletName } from './outlet'

export type SourceRef = {
  key: string
  name: string
  type: string
  url: string
  publishedAt?: string
  headline?: string
  articleId?: string
}

export function fromStorySources(sources: StorySource[]): SourceRef[] {
  return sources.map((source) => ({
    key: source.article_id || source.id,
    name: source.name,
    type: source.type,
    url: source.original_url,
    publishedAt: source.published_at,
    headline: source.headline,
    articleId: source.article_id,
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
    articleId: article.id,
  }))
}

function OriginalLink({ source, children, className }: { source: SourceRef; children: React.ReactNode; className?: string }) {
  return (
    <a
      href={source.url}
      target="_blank"
      rel="noopener noreferrer"
      className={cn('np-link inline-flex min-h-11 items-center', className)}
    >
      {children}
      <ExternalMark />
    </a>
  )
}

/**
 * Compact attribution under a story: every outlet, each with its own link to the original.
 * Wraps freely; every link keeps a 44px tap height.
 */
export function SourceLine({ sources, className }: { sources: SourceRef[]; className?: string }) {
  if (!sources.length) return null
  return (
    <div className={cn('np-meta flex flex-wrap items-center gap-x-3', className)}>
      <span className="text-ink-secondary">මුල් ලිපිය:</span>
      {sources.map((source) => (
        <OriginalLink key={source.key} source={source} className="text-ink">
          <OutletName name={source.name} className="font-bold" />
        </OriginalLink>
      ))}
    </div>
  )
}

/** Full ledger of reporting outlets, chronological, for lead stories and story pages. */
export function SourceLedger({
  sources,
  title = 'මෙම පුවත ගැන මාධ්‍ය වාර්තා',
  showHeadlines = true,
  headingLevel = 2,
  className,
}: {
  sources: SourceRef[]
  title?: string
  showHeadlines?: boolean
  headingLevel?: 2 | 3
  className?: string
}) {
  if (!sources.length) return null
  const ordered = [...sources].sort((a, b) => (a.publishedAt ?? '').localeCompare(b.publishedAt ?? ''))
  const Heading = headingLevel === 2 ? 'h2' : 'h3'
  return (
    <section className={cn('border-t-2 border-ink', className)} aria-label={title}>
      <Heading className="np-kicker flex items-baseline justify-between gap-3 pt-2 text-ink">
        <span>{title}</span>
        <span className="np-meta">{ordered.length}</span>
      </Heading>
      <ol className="mt-1">
        {ordered.map((source) => (
          <li key={source.key} className="border-t border-rule py-2 first:border-t-0">
            <p className="np-meta flex flex-wrap items-baseline gap-x-2">
              <OutletName name={source.name} className="text-[1rem]" />
              <span>{sourceTypeLabel(source.type)}</span>
              {source.publishedAt ? (
                <time dateTime={source.publishedAt} title={absoluteTitle(source.publishedAt)}>
                  · {relativeTime(source.publishedAt)}
                </time>
              ) : null}
            </p>
            {showHeadlines && source.headline ? (
              <p className="font-headline text-[1rem] leading-[1.6] font-medium text-ink-secondary">
                <span aria-hidden="true">“</span>
                {source.headline}
                <span aria-hidden="true">”</span>
              </p>
            ) : null}
            <p className="flex flex-wrap gap-x-5">
              <OriginalLink source={source} className="font-headline font-bold">
                මුල් ලිපිය — {source.name}
              </OriginalLink>
              {source.articleId ? (
                <Link to={`/a/${source.articleId}`} className="np-link np-meta inline-flex min-h-11 items-center">
                  මෙම වාර්තාව ගැන
                </Link>
              ) : null}
            </p>
          </li>
        ))}
      </ol>
    </section>
  )
}
