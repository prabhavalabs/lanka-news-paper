import type { EditorialSummary } from '@snap/api-client'
import { Link } from 'react-router'

import { cn } from '@snap/ui/utils'
import { sinhalaDate } from '@/lib/format'
import { Fleuron } from './ornament'

/** The boxed "කතුවැකිය" panel: double-ruled frame, clearly the editor's opinion. */
export function EditorialPanel({ editorial, className }: { editorial: EditorialSummary | null; className?: string }) {
  return (
    <aside aria-labelledby="editorial-panel-title" className={cn('border-[3px] border-double border-ink p-1', className)}>
      <div className="border border-ink px-4 py-4">
        <h2 id="editorial-panel-title" className="np-headline flex items-center justify-center gap-2 text-center text-[1.625rem] leading-[1.4]">
          <Fleuron flip className="text-base text-ink-secondary" />
          කතුවැකිය
          <Fleuron className="text-base text-ink-secondary" />
        </h2>
        <p className="np-meta text-center text-[0.8125rem]">කතුවරයාගේ මතය · පුවත් වාර්තාවක් නොවේ</p>
        <div className="np-rule mt-3" />
        {editorial ? (
          <article className="mt-3">
            <h3 className="np-headline text-[1.375rem] leading-[1.45]">
              <Link to={`/editorial/${editorial.id}`} className="np-headline-link">
                {editorial.title}
              </Link>
            </h3>
            {editorial.standfirst ? <p className="np-body mt-2 text-[1rem] text-ink-secondary">{editorial.standfirst}</p> : null}
            <p className="np-meta mt-2">
              {editorial.author_name}
              {' · '}
              <time dateTime={editorial.published_at}>{sinhalaDate(editorial.published_at)}</time>
            </p>
            <p className="mt-1 flex flex-wrap justify-between gap-x-4">
              <Link to={`/editorial/${editorial.id}`} className="np-link font-headline inline-flex min-h-11 items-center font-bold">
                කතුවැකිය කියවන්න →
              </Link>
              <Link to="/editorial" className="np-link np-meta inline-flex min-h-11 items-center">
                පෙර කතුවැකි
              </Link>
            </p>
          </article>
        ) : (
          <div className="mt-3">
            <p className="np-body text-[1rem] text-ink-secondary">අද කලාපය සඳහා කතුවැකියක් තවම පළ කර නැත.</p>
            <Link to="/editorial" className="np-link font-headline inline-flex min-h-11 items-center font-bold">
              පෙර කතුවැකි →
            </Link>
          </div>
        )}
      </div>
    </aside>
  )
}
