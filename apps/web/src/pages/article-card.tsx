import type { PublicArticle } from '@snap/api-client'
import { Link } from 'react-router'

import { HalftoneFigure } from '@/components/newsprint/figure'
import { SourceLine, fromArticles } from '@/components/newsprint/sources'
import { OutletName } from '@/components/newsprint/outlet'
import { absoluteTitle, lede, relativeTime, sourceTypeLabel } from '@/lib/format'

/** One outlet's report in a feed: kicker, publisher's headline, byline, lede, original link. */
export function ArticleCard({ item }: { item: PublicArticle }) {
  const text = lede(item.analysis?.summary ?? item.excerpt, 240)
  return (
    <article aria-labelledby={`card-${item.id}`} className="py-5">
      {item.category ? <p className="np-kicker">{item.category.name_si}</p> : null}
      <h2 id={`card-${item.id}`} className="np-headline mt-0.5 text-[1.375rem] leading-[1.45] md:text-[1.5rem]">
        <Link to={`/a/${item.id}`} className="np-headline-link">
          {item.headline}
        </Link>
      </h2>
      <p className="np-meta mt-1">
        <OutletName name={item.source.name} />
        {' · '}
        {sourceTypeLabel(item.source.type)}
        {' · '}
        <time dateTime={item.published_at} title={absoluteTitle(item.published_at)}>
          {relativeTime(item.published_at)}
        </time>
      </p>
      <HalftoneFigure src={item.media} alt="" credit={item.source.name} className="mt-3 max-w-md" />
      {text ? <p className="np-body mt-2 text-[1.0625rem] text-ink-secondary">{text}</p> : null}
      {item.editorial_note ? (
        <p className="mt-2 border-s-2 border-ink ps-3 font-headline text-[0.9375rem] font-medium text-ink-secondary">කතු සටහන: {item.editorial_note}</p>
      ) : null}
      <div className="flex flex-wrap items-center gap-x-5">
        <SourceLine sources={fromArticles([item])} />
        {item.event_id ? (
          <Link to={`/e/${item.event_id}`} className="np-link np-meta inline-flex min-h-11 items-center">
            අනෙක් මාධ්‍ය වාර්තා
          </Link>
        ) : null}
      </div>
    </article>
  )
}
