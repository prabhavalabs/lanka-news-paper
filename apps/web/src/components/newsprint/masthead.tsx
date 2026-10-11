import type { PublicCategory } from '@snap/api-client'
import { Search } from 'lucide-react'
import { forwardRef, useState } from 'react'
import { Link, NavLink, useNavigate } from 'react-router'

import { cn } from '@snap/ui/utils'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { gregorianLongDate, sinhalaLongDate } from '@/lib/format'
import { Fleuron } from './ornament'

const BRAND_LABEL = 'තුලාව මුල් පිටුව'

function Wordmark({ className }: { className?: string }) {
  return <img src="/brand/name-navy.svg" alt="තුලාව" width={932} height={448} className={cn('np-ink-art w-auto', className)} />
}

function EditionLine() {
  const now = new Date()
  return (
    <p className="np-meta flex flex-wrap items-center justify-center gap-x-3 gap-y-0 py-1 text-center text-[0.875rem] text-ink-secondary">
      <span>කොළඹ සංස්කරණය</span>
      <span aria-hidden="true">·</span>
      <time dateTime={now.toISOString().slice(0, 10)}>{sinhalaLongDate(now)}</time>
      <span aria-hidden="true" className="hidden sm:inline">·</span>
      <span lang="en" className="np-latin">
        {gregorianLongDate(now)}
      </span>
    </p>
  )
}

/**
 * Broadsheet nameplate. Full on the front page; a compact folio-style head on inside pages.
 * Brand artwork is printed in ink (no navy/gold on reading surfaces).
 */
export const Masthead = forwardRef<HTMLDivElement, { full: boolean }>(function Masthead({ full }, ref) {
  if (!full) {
    return (
      <div ref={ref} className="mx-auto max-w-[1280px] px-4 pt-3 md:px-12">
        <div className="np-rule" />
        <div className="flex items-center justify-between gap-3 py-2">
          <Link to="/" aria-label={BRAND_LABEL} className="inline-flex items-center gap-2">
            <img src="/brand/thulawa-seal-512.png" alt="" width={40} height={40} className="np-woodcut size-10 md:size-12" />
            <Wordmark className="h-10 md:h-12" />
          </Link>
          <p className="np-meta hidden text-end text-[0.875rem] sm:block">
            {sinhalaLongDate()}
            <br />
            <span lang="en" className="np-latin">
              {gregorianLongDate()}
            </span>
          </p>
        </div>
      </div>
    )
  }
  return (
    <div ref={ref} className="mx-auto max-w-[1280px] px-4 pt-3 md:px-12 md:pt-5">
      <div className="np-rule" />
      <div className="grid grid-cols-1 items-center gap-4 py-3 md:grid-cols-[1fr_auto_1fr] md:py-5">
        <div className="hidden md:block">
          <img src="/brand/thulawa-seal-512.png" alt="" width={96} height={96} className="np-woodcut size-24" />
        </div>
        <Link to="/" aria-label={BRAND_LABEL} className="flex flex-col items-center">
          <span className="flex items-center gap-3">
            <img src="/brand/thulawa-seal-512.png" alt="" width={56} height={56} className="np-woodcut size-14 md:hidden" />
            <Wordmark className="h-[4.5rem] md:h-32 lg:h-36" />
          </span>
          <img
            src="/brand/tagline-gold-dark.svg"
            alt="අසමතුලිත ලෝකයකට සමතුලිත පුවත්"
            width={1900}
            height={156}
            className="np-ink-art mt-2 h-[1.125rem] w-auto md:h-6"
          />
        </Link>
        <div className="hidden justify-self-end md:block">
          <div className="max-w-[13rem] border border-ink p-0.5">
            <p className="border border-ink px-3 py-2 text-center font-headline text-[0.9375rem] leading-[1.5] font-medium">
              <Fleuron className="me-1 text-ink-secondary" />
              සෑම පුවතක්ම එය වාර්තා කළ මාධ්‍යයේ මුල් ලිපියට සබැඳේ
            </p>
          </div>
        </div>
      </div>
      <div className="np-rule-oxford" />
      <EditionLine />
    </div>
  )
})

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    'inline-flex min-h-11 shrink-0 items-center px-2.5 font-headline text-[1rem] leading-none whitespace-nowrap',
    isActive ? 'font-extrabold underline decoration-2 underline-offset-[0.45em]' : 'font-bold text-ink-secondary hover:text-ink',
  )

/** Sticky section strip: horizontally scrollable on phones, with search at the end. */
export function SectionStrip({
  categories,
  condensed,
  onSearch,
}: {
  categories: PublicCategory[]
  condensed: boolean
  onSearch: () => void
}) {
  return (
    <div className="sticky top-0 z-30 border-b border-ink bg-paper/95 supports-[backdrop-filter]:backdrop-blur-[2px]">
      <div className="mx-auto flex max-w-[1280px] items-center gap-1 px-2 md:px-10">
        <Link
          to="/"
          aria-label={BRAND_LABEL}
          tabIndex={condensed ? 0 : -1}
          aria-hidden={!condensed}
          className={cn('shrink-0 items-center pe-2 ps-2', condensed ? 'inline-flex' : 'hidden')}
        >
          <Wordmark className="h-8" />
        </Link>
        <nav aria-label="පුවත් අංශ" className="np-strip min-w-0 flex-1 overflow-x-auto">
          <ul className="flex items-center">
            <li>
              <NavLink to="/" end className={navLinkClass}>
                මුල් පිටුව
              </NavLink>
            </li>
            <li>
              <NavLink to="/editorial" className={navLinkClass}>
                කතුවැකිය
              </NavLink>
            </li>
            {categories.map((category) => (
              <li key={category.slug}>
                <NavLink to={`/c/${category.slug}`} className={navLinkClass}>
                  {category.name_si}
                </NavLink>
              </li>
            ))}
            <li>
              <NavLink to="/brief" className={navLinkClass}>
                උදෑසන සංග්‍රහය
              </NavLink>
            </li>
            <li>
              <NavLink to="/sources" className={navLinkClass}>
                මූලාශ්‍ර
              </NavLink>
            </li>
          </ul>
        </nav>
        <button
          type="button"
          onClick={onSearch}
          className="grid size-11 shrink-0 place-items-center border-s border-rule hover:bg-tint"
          aria-label="පුවත් සොයන්න"
        >
          <Search aria-hidden="true" className="size-5" strokeWidth={1.75} />
        </button>
      </div>
    </div>
  )
}

/** Search as a plain form so Sinhala keyboards' "Go" key submits reliably. */
export function SearchDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const navigate = useNavigate()
  const [value, setValue] = useState('')
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="top-16 translate-y-0 border border-ink bg-paper-raised p-5 shadow-[4px_4px_0_color-mix(in_oklch,var(--ink),transparent_88%)] ring-0 sm:max-w-lg">
        <DialogTitle className="np-headline text-[1.5rem] leading-[1.4] font-extrabold">පුවත් සොයන්න</DialogTitle>
        <DialogDescription className="np-meta text-[0.9375rem]">ශීර්ෂපාඨයක වචනයක් හෝ නමක් ඇතුළත් කරන්න.</DialogDescription>
        <form
          role="search"
          className="flex gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            const q = value.trim()
            navigate(q ? `/search?q=${encodeURIComponent(q)}` : '/search')
            onOpenChange(false)
          }}
        >
          <label htmlFor="site-search" className="sr-only">
            සෙවුම් පදය
          </label>
          <input
            id="site-search"
            type="search"
            autoFocus
            enterKeyHint="search"
            value={value}
            onChange={(event) => setValue(event.target.value)}
            className="np-field flex-1"
            placeholder="උදා: ඉන්ධන මිල"
          />
          <button type="submit" className="np-btn">
            සොයන්න
          </button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
