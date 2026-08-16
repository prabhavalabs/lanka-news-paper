import { useLocation } from 'react-router'

import { ThemeToggle } from '@/components/theme-toggle'
import { Separator } from '@/components/ui/separator'
import { SidebarTrigger } from '@/components/ui/sidebar'

const titles: Record<string, string> = {
  '/': 'Newsroom overview',
  '/sources': 'Sources',
  '/queue': 'Editorial queue',
  '/complaints': 'Complaints',
  '/routing': 'AI & routing',
}

export function SiteHeader() {
  const location = useLocation()
  const title = location.pathname.startsWith('/sources/')
    ? 'Source details'
    : (titles[location.pathname] ?? 'SNAP newsroom')

  return (
    <header className="sticky top-0 z-30 flex h-(--header-height) shrink-0 items-center gap-2 border-b bg-background/90 backdrop-blur transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-(--header-height)">
      <div className="flex w-full items-center gap-1 px-4 lg:gap-2 lg:px-6">
        <SidebarTrigger className="-ml-1" />
        <Separator orientation="vertical" className="mx-2 h-4 data-vertical:self-auto" />
        <h1 className="truncate text-base font-medium">{title}</h1>
        <div className="ml-auto">
          <ThemeToggle />
        </div>
      </div>
    </header>
  )
}
