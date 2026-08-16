import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { Link, Outlet, useNavigate } from 'react-router'

import { Command, CommandDialog, CommandInput } from '@/components/ui/command'
import { useChromeStore } from '../chrome-store'

const client = createClient()

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
  const navigate = useNavigate()
  const condensed = useChromeStore((state) => state.condensed)
  const setCondensed = useChromeStore((state) => state.setCondensed)
  const searchOpen = useChromeStore((state) => state.searchOpen)
  const setSearchOpen = useChromeStore((state) => state.setSearchOpen)
  const categories = useQuery({ queryKey: ['categories'], queryFn: () => client.categories() })
  const breaking = useQuery({ queryKey: ['breaking'], queryFn: () => client.breaking() })

  useEffect(() => {
    const onScroll = () => setCondensed(window.scrollY > 300)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [setCondensed])

  const nav = categories.data?.items.filter((item) => item.slug !== 'latest') ?? []

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
        </div>
      ) : null}
      {breaking.data?.items[0] ? (
        <div className="bg-ink px-6 py-2 text-center text-sm text-paper">
          විශේෂ පුවත්:{' '}
          <Link className="underline" to={`/e/${breaking.data.items[0].id}`}>
            {breaking.data.items[0].title}
          </Link>
        </div>
      ) : null}
      <header className="mx-auto max-w-[1280px] px-6 py-8 text-center md:px-12">
        <div className="h-px bg-rule" />
        <p className="font-headline mt-6 text-[length:var(--text-masthead)] leading-[1.1] font-bold">ලංකා පුවත්</p>
        <p className="mt-2 text-[0.8125rem] text-muted-foreground">{colomboDateLine()}</p>
        <div className="mt-4 flex flex-col gap-[3px]">
          <div className="h-[3px] bg-ink" />
          <div className="h-px bg-ink" />
        </div>
        <div className="relative mt-3 flex items-center">
          <nav className="flex flex-1 gap-5 overflow-x-auto text-[0.8125rem] [scrollbar-width:none]" aria-label="කොටස්">
            <Link to="/" className="shrink-0">
              පුවත්
            </Link>
            {nav.map((item) => (
              <Link key={item.slug} to={`/c/${item.slug}`} className="shrink-0">
                {item.name_si}
              </Link>
            ))}
            <Link to="/brief" className="shrink-0">
              උදෑසන සංග්‍රහය
            </Link>
          </nav>
          <button type="button" className="ms-3 size-11 shrink-0 border border-ink" aria-label="සොයන්න" onClick={() => setSearchOpen(true)}>
            සොයන්න
          </button>
        </div>
        <div className="mt-3 h-px bg-rule" />
      </header>
      <CommandDialog open={searchOpen} onOpenChange={setSearchOpen} title="සොයන්න" description="ශීර්ෂපාඨ සොයන්න">
        <Command>
          <CommandInput
            placeholder="සොයන්න"
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                navigate(`/search?q=${encodeURIComponent(event.currentTarget.value)}`)
                setSearchOpen(false)
              }
            }}
          />
        </Command>
      </CommandDialog>
      <main id="main" className="mx-auto max-w-[1280px] px-6 pb-16 md:px-12">
        <Outlet />
      </main>
      <footer className="mx-auto max-w-[1280px] px-6 py-8 md:px-12">
        <div className="mb-4 flex flex-col gap-[3px]">
          <div className="h-[3px] bg-ink" />
          <div className="h-px bg-ink" />
        </div>
        <nav className="flex flex-wrap gap-4 text-[0.75rem] text-muted-foreground">
          <Link to="/about">අප ගැන</Link>
          <Link to="/privacy">රහස්‍යතාව</Link>
          <Link to="/corrections">නිවැරදි කිරීම්</Link>
          <Link to="/contact">සම්බන්ධ වන්න</Link>
          <Link to="/sources">මූලාශ්‍ර</Link>
        </nav>
      </footer>
    </div>
  )
}
