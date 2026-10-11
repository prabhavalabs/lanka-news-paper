import { Link } from 'react-router'

import { cn } from '@snap/ui/utils'
import { Asterism } from './ornament'

/** Static grey galleys standing in for type while it loads; pulses only without reduced motion. */
export function GalleyPlaceholder({ lines = 6, className }: { lines?: number; className?: string }) {
  return (
    <div className={cn('flex flex-col gap-3', className)} role="status" aria-live="polite">
      <span className="sr-only">පූරණය වෙමින්…</span>
      <div className="h-9 w-4/5 bg-tint motion-safe:animate-pulse" />
      <div className="h-9 w-3/5 bg-tint motion-safe:animate-pulse" />
      {Array.from({ length: lines }, (_, index) => (
        <div key={index} className="h-4 bg-tint motion-safe:animate-pulse" style={{ width: `${92 - ((index * 17) % 30)}%` }} />
      ))}
    </div>
  )
}

/** A boxed printer's notice for errors and empty states. */
export function Notice({
  title,
  children,
  retry,
  className,
}: {
  title: string
  children?: React.ReactNode
  retry?: () => void
  className?: string
}) {
  return (
    <div className={cn('mx-auto max-w-xl border border-ink p-1', className)} role="status">
      <div className="border border-ink px-5 py-6 text-center">
        <Asterism className="text-ink-secondary" />
        <p className="np-headline mt-2 text-[1.375rem] leading-[1.45]">{title}</p>
        {children ? <div className="np-body mt-2 text-ink-secondary">{children}</div> : null}
        <div className="mt-4 flex flex-wrap justify-center gap-3">
          {retry ? (
            <button type="button" className="np-btn np-btn-ghost" onClick={retry}>
              නැවත උත්සාහ කරන්න
            </button>
          ) : null}
          <Link to="/" className="np-btn np-btn-ghost">
            මුල් පිටුවට
          </Link>
        </div>
      </div>
    </div>
  )
}
