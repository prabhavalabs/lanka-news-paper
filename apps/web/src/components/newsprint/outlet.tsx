import { cn } from '@snap/ui/utils'

import { isLatin } from '@/lib/format'

/** Outlet name as a printed byline: Latin names in small caps, Sinhala names never tracked. */
export function OutletName({ name, className }: { name: string; className?: string }) {
  const latin = isLatin(name)
  return (
    <span lang={latin ? 'en' : undefined} className={cn(latin ? 'np-smallcaps' : 'font-headline', 'font-bold text-ink', className)}>
      {name}
    </span>
  )
}

export function ExternalMark() {
  return (
    <>
      <span aria-hidden="true" className="ms-0.5 inline-block text-[0.75em] leading-none">↗</span>
      <span className="sr-only"> (නව පටිත්තක විවෘත වේ)</span>
    </>
  )
}
