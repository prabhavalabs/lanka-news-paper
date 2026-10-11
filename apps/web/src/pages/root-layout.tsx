import { createClient } from '@snap/api-client'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { Link, Outlet, useLocation } from 'react-router'

import { SiteFooter } from '@/components/newsprint/footer'
import { Masthead, SearchDialog, SectionStrip } from '@/components/newsprint/masthead'
import { useChromeStore } from '../chrome-store'

const client = createClient()

export function RootLayout() {
  const { pathname } = useLocation()
  const isFront = pathname === '/'
  const condensed = useChromeStore((state) => state.condensed)
  const setCondensed = useChromeStore((state) => state.setCondensed)
  const searchOpen = useChromeStore((state) => state.searchOpen)
  const setSearchOpen = useChromeStore((state) => state.setSearchOpen)
  const categories = useQuery({ queryKey: ['categories'], queryFn: () => client.categories(), staleTime: 10 * 60_000 })
  const breaking = useQuery({ queryKey: ['breaking'], queryFn: () => client.breaking(), enabled: !isFront })
  const nameplate = useRef<HTMLDivElement>(null)

  // The small wordmark joins the sticky section strip once the nameplate has scrolled away.
  useEffect(() => {
    const node = nameplate.current
    if (!node) return
    const observer = new IntersectionObserver(([entry]) => setCondensed(!entry?.isIntersecting), { threshold: 0 })
    observer.observe(node)
    return () => observer.disconnect()
  }, [setCondensed, isFront])

  useEffect(() => {
    window.scrollTo({ top: 0 })
  }, [pathname])

  const nav = categories.data?.items.filter((item) => item.slug !== 'latest') ?? []
  const flash = !isFront ? breaking.data?.items[0] : undefined

  return (
    <div className="min-h-screen bg-paper text-ink">
      <a
        href="#main"
        className="sr-only z-50 focus:not-sr-only focus:fixed focus:start-3 focus:top-3 focus:border focus:border-ink focus:bg-paper focus:px-4 focus:py-3 focus:font-bold"
      >
        ප්‍රධාන අන්තර්ගතයට යන්න
      </a>
      <header>
        <Masthead ref={nameplate} full={isFront} />
      </header>
      <SectionStrip categories={nav} condensed={condensed} onSearch={() => setSearchOpen(true)} />
      {flash ? (
        <div className="bg-ink text-paper">
          <p className="mx-auto flex max-w-[1280px] flex-wrap items-center gap-x-3 px-4 py-2 font-headline text-[1rem] leading-[1.5] md:px-12">
            <span className="border border-paper px-1.5 font-extrabold">විශේෂ පුවත</span>
            <Link to={`/e/${flash.id}`} className="font-bold underline underline-offset-4">
              {flash.title}
            </Link>
          </p>
        </div>
      ) : null}
      <SearchDialog open={searchOpen} onOpenChange={setSearchOpen} />
      <main id="main" tabIndex={-1} className="mx-auto max-w-[1280px] px-4 pt-5 outline-none md:px-12 md:pt-8">
        <Outlet />
      </main>
      <SiteFooter />
    </div>
  )
}
