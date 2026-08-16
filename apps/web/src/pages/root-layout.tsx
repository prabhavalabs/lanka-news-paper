import { useEffect } from 'react'
import { Link, Outlet } from 'react-router'

import { useChromeStore } from '../chrome-store'

const sections = ['පුවත්', 'දේශපාලන', 'ආර්ථික', 'ක්‍රීඩා', 'ලෝක']

function colomboDateLine() {
  const formatted = new Intl.DateTimeFormat('si-LK', {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    timeZone: 'Asia/Colombo',
  }).format(new Date())
  return `${formatted} | කොළඹ`
}

export function RootLayout() {
  const condensed = useChromeStore((state) => state.condensed)
  const setCondensed = useChromeStore((state) => state.setCondensed)
  const setSearchOpen = useChromeStore((state) => state.setSearchOpen)

  useEffect(() => {
    const onScroll = () => setCondensed(window.scrollY > 300)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [setCondensed])

  return (
    <div className="min-h-screen bg-paper text-ink">
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:start-4 focus:top-4 focus:bg-paper focus:px-3 focus:py-2">
        මුල් අන්තර්ගතයට යන්න
      </a>
      {condensed ? (
        <div className="sticky top-0 z-10 flex h-12 items-center justify-between border-b border-rule bg-paper px-6">
          <Link to="/" className="font-headline text-lg font-bold">
            ලංකා පුවත්
          </Link>
          <nav className="hidden gap-4 text-[0.8125rem] md:flex" aria-label="කොටස්">
            {sections.map((section) => (
              <span key={section}>{section}</span>
            ))}
          </nav>
        </div>
      ) : null}
      <header className="mx-auto max-w-[1280px] px-6 py-8 text-center md:px-12">
        <div className="h-px bg-rule" />
        <p className="font-headline mt-6 text-[length:var(--text-masthead)] leading-[1.1] font-bold">
          ලංකා පුවත්
        </p>
        <p className="mt-2 text-[0.8125rem] text-[color:var(--ink-tertiary)]">{colomboDateLine()}</p>
        <div className="mt-4 flex flex-col gap-[3px]">
          <div className="h-[3px] bg-ink" />
          <div className="h-px bg-ink" />
        </div>
        <div className="relative mt-3 flex items-center">
          <nav
            className="flex flex-1 gap-5 overflow-x-auto text-[0.8125rem] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
            aria-label="කොටස්"
          >
            {sections.map((section) => (
              <span key={section} className="shrink-0">
                {section}
              </span>
            ))}
          </nav>
          <button
            type="button"
            className="ms-3 size-11 shrink-0 border border-ink"
            aria-label="සොයන්න"
            onClick={() => setSearchOpen(true)}
          >
            සොයන්න
          </button>
        </div>
        <div className="mt-3 h-px bg-rule" />
      </header>
      <main id="main" className="mx-auto max-w-[1280px] px-6 pb-16 md:px-12">
        <Outlet />
      </main>
      <footer className="mx-auto max-w-[1280px] px-6 py-8 md:px-12">
        <div className="mb-4 flex flex-col gap-[3px]">
          <div className="h-[3px] bg-ink" />
          <div className="h-px bg-ink" />
        </div>
        <p className="text-[0.75rem] text-[color:var(--ink-tertiary)]">මූලාශ්‍ර වෙත යොමු කරන සොයාගැනීමේ වේදිකාවකි.</p>
      </footer>
    </div>
  )
}
