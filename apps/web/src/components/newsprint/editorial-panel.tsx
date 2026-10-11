import type { EditorialSummary } from '@snap/api-client'
import { Link } from 'react-router'

import { cn } from '@snap/ui/utils'
import { sinhalaDate } from '@/lib/format'
import { Fleuron } from './ornament'

/** The boxed "කතුවැකිය": set in the flow of the page, clearly marked as the editor's opinion. */
export function EditorialPanel({ editorial, className }: { editorial: EditorialSummary | null; className?: string }) {
  return (
    <aside aria-labelledby="editorial-panel-title" className={cn('border-[3px] border-double border-ink p-0.5', className)}>
      <div className="border border-ink px-3 py-3">
        <p id="editorial-panel-title" className="np-headline flex items-center justify-center gap-2 text-center text-[1.375rem] leading-[1.4]">
          <Fleuron flip className="text-[0.7em] text-ink-secondary" />
          කතුවැකිය
          <Fleuron className="text-[0.7em] text-ink-secondary" />
        </p>
        <p className="np-meta border-b border-ink pb-1.5 text-center text-[0.75rem]">කතුවරයාගේ මතය · පුවත් වාර්තාවක් නොවේ</p>
        {editorial ? (
          <>
            <h2 className="np-headline mt-2 text-[1.3125rem] leading-[1.42]">
              <Link to={`/editorial/${editorial.id}`} className="np-headline-link max-lg:inline-block max-lg:min-h-11">
                {editorial.title}
              </Link>
            </h2>
            {editorial.standfirst ? <p className="np-standfirst mt-1">{editorial.standfirst}</p> : null}
            <p className="np-meta mt-1 text-[0.8125rem]">
              {editorial.author_name} · <time dateTime={editorial.published_at}>{sinhalaDate(editorial.published_at)}</time>
            </p>
          </>
        ) : (
          <p className="np-copy mt-2 text-[1rem] leading-[1.7] text-ink-secondary">
            අද කලාපයට කතුවැකියක් තවම පළ කර නැත.{' '}
            <Link to="/editorial" className="np-link">
              පෙර කතුවැකි
            </Link>
          </p>
        )}
      </div>
    </aside>
  )
}
