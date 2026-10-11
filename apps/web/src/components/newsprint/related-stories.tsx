import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'

import { cn } from '@snap/ui/utils'
import { absoluteTitle, outletsReported, relativeTime } from '@/lib/format'
import { storyHref } from './story'

const client = createClient()

function excerpt(text: string | null, max = 150) {
  const clean = (text ?? '').replace(/\s+/g, ' ').trim()
  if (clean.length <= max) return clean
  const cut = clean.slice(0, max)
  return `${cut.slice(0, Math.max(cut.lastIndexOf(' '), max * 0.6))}…`
}

// Ruled 3-up grid: vertical rules between columns, a hairline between rows.
function cellClass(index: number) {
  return cn(
    'py-4',
    index > 0 && 'border-t border-rule',
    'md:border-t-0',
    index % 3 !== 0 && 'md:border-s md:border-rule md:ps-5',
    index % 3 !== 2 && 'md:pe-5',
    index >= 3 && 'md:border-t md:border-rule',
  )
}

/** "සම්බන්ධ පුවත්": up to six stories to read next, same section first. */
export function RelatedStories({ kind, id, className }: { kind: 'event' | 'article'; id: string; className?: string }) {
  const related = useQuery({
    queryKey: ['related', kind, id],
    queryFn: () => client.relatedStories(kind, id),
    enabled: Boolean(id),
    retry: false,
    staleTime: 5 * 60_000,
  })
  const items = related.data?.items.slice(0, 6) ?? []
  if (!items.length) return null
  return (
    <section aria-labelledby="related-title" className={className}>
      <div className="np-rule-double" />
      <h2 id="related-title" className="np-section-head mt-2">
        සම්බන්ධ පුවත්
      </h2>
      <ul className="mt-1 grid grid-cols-1 md:grid-cols-3">
        {items.map((story, index) => (
          <li key={`${story.kind}:${story.id}`} className={cellClass(index)}>
            <article aria-labelledby={`related-${story.id}`}>
              {story.category ? <p className="np-kicker text-[0.8125rem]">{story.category.name_si}</p> : null}
              <h3 id={`related-${story.id}`} className="np-headline text-[1.3125rem] leading-[1.42]">
                <Link to={storyHref(story)} className="np-headline-link max-lg:inline-block max-lg:min-h-11">
                  {story.headline}
                </Link>
              </h3>
              {story.summary ? <p className="np-copy mt-1 text-[0.9375rem] leading-[1.7] text-ink-secondary">{excerpt(story.summary)}</p> : null}
              <p className="np-meta mt-1 text-[0.75rem]">
                {outletsReported(story.source_count || story.sources.length)}
                {' · '}
                <time dateTime={story.updated_at} title={absoluteTitle(story.updated_at)}>
                  {relativeTime(story.updated_at)}
                </time>
              </p>
            </article>
          </li>
        ))}
      </ul>
    </section>
  )
}
